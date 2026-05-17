// Package addcmd contributes `one add` to the root command via cliexts.
// Adds a new subproject to the current workspace by rendering a built-in
// template; applies the template's `defaults` map to the manifest and
// (for kustomize / docker templates) syncs infra + CI scaffolding.
//
// The bulk of the workspace mutation lives in internal/preset.ApplyProject
// (the same engine driving `one create --preset`). This file is a thin
// shell: parse flags + positional, run the registry / prompts, then call
// into the engine.
package addcmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/ai"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/preset"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func init() {
	cliexts.Register("add", buildContributions)
}

func buildContributions() []*cobra.Command {
	return []*cobra.Command{newAddCmd()}
}

type addFlags struct {
	name   string
	yes    bool
	deploy string
}

func newAddCmd() *cobra.Command {
	flags := &addFlags{}
	cmd := &cobra.Command{
		Use: "add [template-id]",
		Long: `添加项目到当前工作区。

  位置参数 = 模板 ID（从 one templates 选）：
    one add nestjs-api --name user-api --yes

模板模式渲染物理模板。

infra / deploy（Dockerfile / Kustomize / S3-compatible deploy / CI 工作流）
由模板自动决定。API / SSR 模板通常启用
container/docker + deploy/kustomize；静态前端模板通常启用 deploy/aws-s3。
container / deploy 是 per-project domain，可随后用
手编 one.manifest.json 调整 deploy/container 字段。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			positional := ""
			if len(args) > 0 {
				positional = args[0]
			}
			return runAdd(cmd, positional, flags)
		},
	}
	cmd.Flags().StringVarP(&flags.name, "name", "n", "", "项目名称（必选）")
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, "非交互模式")
	cmd.Flags().StringVar(&flags.deploy, "deploy-provider", "",
		"显式选择 deploy 后端（kustomize / aws-s3 / aliyun-oss / vercel ...）；非交互模式或想跳过选择 prompt 时使用。"+
			"必须在模板的 compat.deploy 列表里。")
	i18n.MarkShort(cmd, "add.short")
	return cmd
}

func runAdd(cmd *cobra.Command, positional string, flags *addFlags) error {
	projectRoot, err := workspace.ResolveProjectRoot("")
	if err != nil {
		return err
	}

	if !workspace.HasManifest(projectRoot) {
		return cliErrors.New(cliErrors.NOT_ONE_PROJECT,
			"未检测到 One CLI 项目，请在项目根目录执行。").
			WithContext(map[string]any{
				"cwd":           projectRoot,
				"manifest_path": workspace.ManifestPath(projectRoot),
			})
	}

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
		got, perr := prompt.Text("项目名称", "user-service", func(v string) error {
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

	// All workspace mutation now lives in preset.ApplyProject (the same
	// engine `one create --preset` orchestrates over multiple projects).
	// addcmd remains a thin shell: validate flags, prompt where the
	// command-specific UX is, then hand off.
	result, err := preset.ApplyProject(cmd.Context(), projectRoot, preset.ProjectInput{
		Template: entry,
		Name:     name,
		Deploy:   flags.deploy,
	}, interactive)
	if err != nil {
		return err
	}
	for _, w := range result.Warnings {
		prompt.Step("⚠ " + w)
	}

	// AI-guides refresh runs once per command. Preset.Apply calls this
	// itself after rendering all its projects; for the single-project
	// add path we call it here.
	guides := ai.Refresh(projectRoot, false)

	output.Emit(&addResult{
		Schema:         "one-cli/add/v1",
		SubprojectName: result.Name,
		TargetPath:     result.TargetPath,
		TemplateID:     result.TemplateID,
		Toolchain:      result.Toolchain,
		PackageManager: result.PackageManager,
		AiGuides:       guides,
		Warnings:       result.Warnings,
	})

	return nil
}

type addResult struct {
	Schema         string           `json:"schema"`
	SubprojectName string           `json:"subproject_name"`
	TargetPath     string           `json:"target_path"`
	TemplateID     string           `json:"template_id"`
	Toolchain      string           `json:"toolchain"`
	PackageManager string           `json:"package_manager,omitempty"`
	AiGuides       ai.RefreshResult `json:"ai_guides"`
	// Warnings (v0.5+) carries one entry per template `compat` mismatch.
	// Empty slice / nil is omitted from the JSON envelope so clean adds
	// match the pre-v0.5 wire shape.
	Warnings []string `json:"warnings,omitempty"`
}

// RenderTTY prints a friendly add-success summary.
func (r *addResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "✓ Added subproject: %s\n", r.SubprojectName)
	fmt.Fprintf(w, "  Path: %s\n", r.TargetPath)
	fmt.Fprintf(w, "  Template: %s (%s)\n", r.TemplateID, r.Toolchain)
	if r.PackageManager != "" {
		fmt.Fprintf(w, "  Package manager: %s\n", r.PackageManager)
	}
	if r.AiGuides.Status == "completed" && len(r.AiGuides.GeneratedFiles) > 0 {
		fmt.Fprintf(w, "  AI guides: refreshed (%d file%s)\n",
			len(r.AiGuides.GeneratedFiles), pluralS(len(r.AiGuides.GeneratedFiles)))
	}
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func findTemplate(items []template.Template, id string) *template.Template {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

// categoryLabel returns the bilingual display label used in the
// interactive picker.
func categoryLabel(c template.Category) string {
	switch c {
	case template.CategoryFrontend:
		return "前端 (Frontend)"
	case template.CategoryBackend:
		return "后端 (Backend)"
	case template.CategoryLibrary:
		return "工具库 (Library)"
	default:
		return string(c)
	}
}

// selectTemplateInteractively drives the two-step template picker:
// first pick a category, then pick a template within that category.
// The returned string is the template ID.
func selectTemplateInteractively(items []template.Template) (string, error) {
	order := []template.Category{
		template.CategoryFrontend,
		template.CategoryBackend,
		template.CategoryLibrary,
	}
	grouped := make(map[template.Category][]template.Template, len(order))
	for _, t := range items {
		grouped[t.Category] = append(grouped[t.Category], t)
	}

	available := make([]template.Category, 0, len(order))
	for _, c := range order {
		if len(grouped[c]) > 0 {
			available = append(available, c)
		}
	}
	seen := map[template.Category]bool{}
	for _, c := range order {
		seen[c] = true
	}
	for _, t := range items {
		if !seen[t.Category] {
			seen[t.Category] = true
			available = append(available, t.Category)
		}
	}

	if len(available) == 0 {
		return "", cliErrors.New(cliErrors.NO_TEMPLATES, "注册表中没有可用模板。")
	}

	var chosen template.Category
	if len(available) == 1 {
		chosen = available[0]
	} else {
		opts := make([]prompt.Option[template.Category], 0, len(available))
		for _, c := range available {
			opts = append(opts, prompt.Option[template.Category]{
				Label: categoryLabel(c),
				Value: c,
			})
		}
		picked, err := prompt.Select("选择模板分类", opts)
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
	return prompt.SelectWithDescriptions("选择模板", tplOpts)
}
