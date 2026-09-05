// Package addcmd contributes `one add` to the explicit root command.
// Adds a new project to the current workspace by rendering a built-in
// technology stack. Ordinary calls configure local development only; CI,
// deployment, and image-registry choices are not added implicitly.
//
// The workspace mutation lives in modules/creation, shared with
// `one create --preset`. This file is a thin
// shell: parse flags + positional, run the registry / prompts, then call
// into the engine.
package addcmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	creationmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/creation"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

func Commands(service *creationmodule.Service) []*cobra.Command { return buildContributions(service) }

func buildContributions(service *creationmodule.Service) []*cobra.Command {
	return []*cobra.Command{newAddCmd(service)}
}

type addFlags struct {
	name   string
	yes    bool
	deploy string
}

func newAddCmd(service *creationmodule.Service) *cobra.Command {
	flags := &addFlags{}
	cmd := &cobra.Command{
		Use:     "add [template-id]",
		Long:    i18n.T("add.tip"),
		Example: "  one add\n  one add react-spa --name web --yes",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			positional := ""
			if len(args) > 0 {
				positional = args[0]
			}
			return runAdd(cmd, service, positional, flags)
		},
	}
	cmd.Flags().StringVarP(&flags.name, "name", "n", "", i18n.T("add.flag.name"))
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, i18n.T("add.flag.yes"))
	cmd.Flags().StringVar(&flags.deploy, "deploy-provider", "",
		i18n.T("add.flag.deploy_provider"))
	i18n.MarkFlagUsage(cmd, "name", "add.flag.name")
	i18n.MarkFlagUsage(cmd, "yes", "add.flag.yes")
	i18n.MarkFlagUsage(cmd, "deploy-provider", "add.flag.deploy_provider")
	helpui.MarkAdvanced(cmd, "deploy-provider")
	i18n.MarkShort(cmd, "add.short")
	i18n.MarkLong(cmd, "add.tip")
	return cmd
}

func runAdd(cmd *cobra.Command, service *creationmodule.Service, positional string, flags *addFlags) error {
	activeWorkspace, err := execution.ResolveWorkspace(cmd.Context())
	if err != nil {
		return err
	}
	projectRoot := activeWorkspace.Root()

	templateID := positional
	interactive := !flags.yes && output.CanPrompt()

	registry, err := template.Fetch(cmd.Context(), "")
	if err != nil {
		return err
	}
	if len(registry.Templates) == 0 {
		return cliErrors.New(cliErrors.NO_TEMPLATES, "模板注册表为空。")
	}

	if templateID == "" {
		if !interactive {
			return cliErrors.New(cliErrors.TEMPLATE_REQUIRED,
				"非交互模式下必须通过位置参数指定模板 ID。可执行 `one templates` 查看可用模板。")
		}
		picked, perr := selectTemplateInteractively(registry.Templates)
		if perr != nil {
			return perr
		}
		templateID = picked
	}
	entry := findTemplate(registry.Templates, templateID)
	if entry == nil {
		ids := make([]string, 0, len(registry.Templates))
		for _, t := range registry.Templates {
			ids = append(ids, t.ID)
		}
		return cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND,
			fmt.Sprintf("模板 %q 不存在，使用 `one templates` 查看可用模板。", templateID)).
			WithContext(map[string]any{
				"requested_template":  templateID,
				"available_templates": ids,
			})
	}

	name := strings.TrimSpace(flags.name)
	if name == "" {
		if !interactive {
			return cliErrors.New(cliErrors.SUBPROJECT_NAME_REQUIRED,
				"非交互模式下必须通过 --name 指定项目名称。")
		}
		got, perr := prompt.Text(i18n.T("add.prompt_name"), "user-service", func(v string) error {
			v = strings.TrimSpace(v)
			if v == "" {
				return errors.New("请输入项目名称")
			}
			if !workspace.IsValidProjectName(v) {
				return errors.New("名称只能包含字母数字、下划线、连字符，且不能以连字符开头")
			}
			return nil
		})
		if perr != nil {
			return perr
		}
		name = strings.TrimSpace(got)
	}
	if !workspace.IsValidProjectName(name) {
		return cliErrors.New(cliErrors.INVALID_NAME,
			fmt.Sprintf("项目名称格式不合法: %q", name))
	}

	// All workspace mutation now lives in creation.Service (the same
	// engine `one create --preset` orchestrates over multiple projects).
	// addcmd remains a thin shell: validate flags, prompt where the
	// command-specific UX is, then hand off.
	// Ordinary add deliberately leaves deployment unset. An explicit advanced
	// flag retains the automation path that configures it immediately.
	projectInput := creationmodule.ProjectInput{
		Template:        entry,
		Name:            name,
		Deploy:          flags.deploy,
		Container:       entry.Defaults["container"],
		DeferDeployment: strings.TrimSpace(flags.deploy) == "",
	}
	if !projectInput.DeferDeployment && interactive {
		projectInput.ConfigureDeployTargets = true
		projectInput.DeployTarget, err = promptDeploymentTarget(activeWorkspace.Manifest(), name, flags.deploy)
		if err != nil {
			return err
		}
	}
	var result creationmodule.AddProjectResult
	if err := prompt.Spin(i18n.Tf("add.generating", entry.ID), func() error {
		var createErr error
		result, createErr = service.AddProject(cmd.Context(), projectRoot, projectInput)
		return createErr
	}); err != nil {
		return err
	}
	prompt.Step(i18n.Tf("add.generated", name))
	project := result.Project
	for _, w := range project.Warnings {
		prompt.Step("⚠ " + w)
	}

	output.Emit(&addResult{
		Schema:           "one-cli/add/v1",
		SubprojectName:   project.Name,
		TargetPath:       project.TargetPath,
		TemplateID:       project.TemplateID,
		Toolchain:        project.Toolchain,
		PackageManager:   project.PackageManager,
		Warnings:         project.Warnings,
		DeployConfigured: project.DeployBackend != "",
	})

	return nil
}

func promptDeploymentTarget(
	manifest *workspace.Manifest,
	projectName string,
	backend string,
) (creationmodule.DeploymentTarget, error) {
	target := creationmodule.DeploymentTarget{}
	var err error
	if workspace.IsS3CompatibleDeploy(backend) &&
		workspace.ExplicitDeployBucketForProject(manifest, projectName) == "" &&
		workspace.WorkspaceID(manifest) == "" {
		target.Bucket, err = prompt.Text(
			fmt.Sprintf("S3 bucket — writes projects[%s].deploy.bucket (legacy manifest without workspace.id only)", projectName),
			"",
			nil,
		)
		if err != nil {
			return creationmodule.DeploymentTarget{}, err
		}
	}
	if backend == workspace.DeployBackendKustomize &&
		workspace.ExplicitDeployNamespace(manifest) == "" &&
		workspace.WorkspaceID(manifest) == "" {
		target.Namespace, err = prompt.Text(
			"k8s namespace — 写入 manifest.deploy.namespace（workspace 级，所有 k8s 项目共享）",
			"default",
			nil,
		)
		if err != nil {
			return creationmodule.DeploymentTarget{}, err
		}
	}
	return target, nil
}

type addResult struct {
	Schema         string `json:"schema"`
	SubprojectName string `json:"subproject_name"`
	TargetPath     string `json:"target_path"`
	TemplateID     string `json:"template_id"`
	Toolchain      string `json:"toolchain"`
	PackageManager string `json:"package_manager,omitempty"`
	// Warnings (v0.5+) carries one entry per template `compat` mismatch.
	// Empty slice / nil is omitted from the JSON envelope so clean adds
	// match the pre-v0.5 wire shape.
	Warnings         []string `json:"warnings,omitempty"`
	DeployConfigured bool     `json:"-"`
}

// RenderTTY prints a friendly add-success summary.
func (r *addResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, i18n.T("add.success")+"\n", r.SubprojectName)
	fmt.Fprintf(w, i18n.T("add.location")+"\n", r.TargetPath)
	fmt.Fprintf(w, i18n.T("add.stack")+"\n", r.TemplateID, r.Toolchain)
	if r.PackageManager != "" {
		fmt.Fprintf(w, i18n.T("add.package_manager")+"\n", r.PackageManager)
	}
	if r.DeployConfigured {
		fmt.Fprintln(w, i18n.T("add.deploy_configured"))
	} else {
		fmt.Fprintln(w, i18n.T("add.deploy_deferred"))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("common.next_steps"))
	fmt.Fprintf(w, "  one dev %s\n", r.SubprojectName)
}

func findTemplate(items []template.Template, id string) *template.Template {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

type projectKind string

const (
	kindApplication projectKind = "application"
	kindService     projectKind = "service"
	kindLibrary     projectKind = "library"
)

func projectKindFor(t template.Template) projectKind {
	switch t.Category {
	case template.CategoryBackend:
		return kindService
	case template.CategoryLibrary:
		return kindLibrary
	default:
		return kindApplication
	}
}

func projectKindLabel(k projectKind) string {
	return i18n.T("add.kind." + string(k))
}

// selectTemplateInteractively asks what the user wants to add, then which
// technology stack. runAdd asks for the project name immediately afterwards.
func selectTemplateInteractively(items []template.Template) (string, error) {
	order := []projectKind{
		kindApplication,
		kindService,
		kindLibrary,
	}
	grouped := make(map[projectKind][]template.Template, len(order))
	for _, t := range items {
		kind := projectKindFor(t)
		grouped[kind] = append(grouped[kind], t)
	}

	available := make([]projectKind, 0, len(order))
	for _, c := range order {
		if len(grouped[c]) > 0 {
			available = append(available, c)
		}
	}

	if len(available) == 0 {
		return "", cliErrors.New(cliErrors.NO_TEMPLATES, "注册表中没有可用模板。")
	}

	var chosen projectKind
	if len(available) == 1 {
		chosen = available[0]
	} else {
		opts := make([]prompt.Option[projectKind], 0, len(available))
		for _, c := range available {
			opts = append(opts, prompt.Option[projectKind]{
				Label: projectKindLabel(c),
				Value: c,
			})
		}
		picked, err := prompt.Select(i18n.T("add.prompt_kind"), opts)
		if err != nil {
			return "", err
		}
		chosen = picked
	}

	templates := grouped[chosen]
	tplOpts := make([]prompt.Option[string], 0, len(templates))
	for _, t := range templates {
		tplOpts = append(tplOpts, prompt.Option[string]{
			Label:       t.Name,
			Description: t.Description,
			Value:       t.ID,
		})
	}
	return prompt.SelectWithDescriptions(i18n.T("add.prompt_stack"), tplOpts)
}
