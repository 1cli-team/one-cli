// Package addcmd contributes `one add` to the explicit root command.
// Adds a new project to the current workspace by rendering a built-in
// technology stack. Ordinary calls configure local development only; CI,
// deployment, and image-registry choices are not added implicitly.
//
// The workspace mutation lives in modules/creation. This file is a thin
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
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

func Commands(service *creationmodule.Service) []*cobra.Command { return buildContributions(service) }

func buildContributions(service *creationmodule.Service) []*cobra.Command {
	return []*cobra.Command{newAddCmd(service)}
}

type addFlags struct {
	name string
	yes  bool
}

func newAddCmd(service *creationmodule.Service) *cobra.Command {
	flags := &addFlags{}
	cmd := &cobra.Command{
		Use:     "add [template-id]",
		Long:    i18n.T("add.tip"),
		Example: "  one add\n  one add react-spa --name web --yes\n  one add empty-app --name web --yes\n  one add empty-service --name api --yes",
		Args:    i18n.MaximumNArgs(1),
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
	i18n.MarkFlagUsage(cmd, "name", "add.flag.name")
	i18n.MarkFlagUsage(cmd, "yes", "add.flag.yes")
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
		return cliErrors.New(cliErrors.NO_TEMPLATES, i18n.T("add.registry_empty"))
	}

	if templateID == "" {
		if !interactive {
			return cliErrors.New(cliErrors.TEMPLATE_REQUIRED,
				i18n.T("add.template_required"))
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
			i18n.Tf("add.template_missing", templateID)).
			WithContext(map[string]any{
				"requested_template":  templateID,
				"available_templates": ids,
			})
	}

	name := strings.TrimSpace(flags.name)
	if name == "" {
		if !interactive {
			return cliErrors.New(cliErrors.SUBPROJECT_NAME_REQUIRED,
				i18n.T("add.name_required"))
		}
		got, perr := prompt.Text(i18n.T("add.prompt_name"), "user-service", func(v string) error {
			v = strings.TrimSpace(v)
			if v == "" {
				return errors.New(i18n.T("add.enter_name"))
			}
			if !workspace.IsValidProjectName(v) {
				return errors.New(i18n.T("common.name_format"))
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
			i18n.Tf("add.name_invalid", name))
	}

	// All workspace mutation now lives in creation.Service (the same
	// engine).
	// addcmd remains a thin shell: validate flags, prompt where the
	// command-specific UX is, then hand off.
	// Ordinary add deliberately leaves deployment unset. An explicit advanced
	// flag retains the automation path that configures it immediately.
	projectInput := creationmodule.ProjectInput{
		Template: entry,
		Name:     name,
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
		Schema:         "one-cli/add/v1",
		SubprojectName: project.Name,
		TargetPath:     project.TargetPath,
		TemplateID:     project.TemplateID,
		Toolchain:      project.Toolchain,
		PackageManager: project.PackageManager,
		Warnings:       project.Warnings,
		Projects:       addedProjects(project.Projects),
	})

	return nil
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
	Warnings []string     `json:"warnings,omitempty"`
	Projects []addProject `json:"projects,omitempty"`
}

type addProject struct {
	Name       string `json:"name"`
	TargetPath string `json:"target_path"`
	Toolchain  string `json:"toolchain"`
}

func addedProjects(projects []creationmodule.ProjectResult) []addProject {
	if len(projects) == 0 {
		return nil
	}
	result := make([]addProject, 0, len(projects))
	for _, project := range projects {
		result = append(result, addProject{Name: project.Name, TargetPath: project.TargetPath, Toolchain: project.Toolchain})
	}
	return result
}

// RenderTTY prints a friendly add-success summary.
func (r *addResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	if len(r.Projects) > 0 {
		fmt.Fprintf(w, i18n.T("add.group_success")+"\n", r.SubprojectName)
		fmt.Fprintln(w, i18n.T("add.group_projects"))
		for _, project := range r.Projects {
			fmt.Fprintf(w, i18n.T("add.group_member")+"\n", project.Name, project.TargetPath)
		}
		fmt.Fprintln(w, i18n.T("add.group_next_steps"))
		fmt.Fprintf(w, "  one run %s:dev\n", r.SubprojectName)
		return
	}
	fmt.Fprintf(w, i18n.T("add.success")+"\n", r.SubprojectName)
	fmt.Fprintf(w, i18n.T("add.location")+"\n", r.TargetPath)
	fmt.Fprintf(w, i18n.T("add.stack")+"\n", r.TemplateID, r.Toolchain)
	if r.PackageManager != "" {
		fmt.Fprintf(w, i18n.T("add.package_manager")+"\n", r.PackageManager)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("common.next_steps"))
	if r.Toolchain == string(template.ToolchainNone) {
		fmt.Fprintln(w, i18n.T("add.empty_next_steps"))
		return
	}
	fmt.Fprintf(w, "  one dev -p %s\n", r.SubprojectName)
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
		return "", cliErrors.New(cliErrors.NO_TEMPLATES, i18n.T("add.no_templates"))
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
			Label:       t.DisplayName(),
			Description: t.DisplayDescription(),
			Value:       t.ID,
		})
	}
	return prompt.SelectWithDescriptions(i18n.T("add.prompt_stack"), tplOpts)
}
