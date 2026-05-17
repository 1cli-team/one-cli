package preset

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/ci"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

// ProjectInput names everything ApplyProject needs to materialise one
// subproject into an existing workspace. All callers (addcmd, preset.Apply)
// fully resolve the template + chosen deploy before calling here.
type ProjectInput struct {
	// Template is the resolved registry entry (caller did the lookup).
	Template *template.Template
	// Name is the subproject name (validated for the IsValidProjectName
	// regex by the caller).
	Name string
	// Deploy is the optional override; "" means use Template.Defaults["deploy"].
	// Must already be in Template.Compat["deploy"]; we re-check defensively.
	Deploy string
	// Container is the optional container backend for kustomize deploys.
	// Empty means use Docker Hub as the preset default when kustomize is
	// selected.
	Container string
}

// ProjectResult mirrors what addcmd today emits in its add/v1 envelope,
// plus the resolved deploy backend (so the preset engine can aggregate
// deploy_summary across projects).
type ProjectResult struct {
	Name           string
	TargetPath     string
	TemplateID     string
	Toolchain      string
	PackageManager string
	// DeployBackend is the effective deploy backend that ended up on
	// the manifest for this project (e.g. "kustomize", "vercel"). ""
	// when the template carries no deploy domain.
	DeployBackend string
	Warnings      []string
}

// ApplyProject renders the template into projectRoot, upserts the
// manifest, applies template defaults (with the optional deploy
// override), and runs infra + CI sync. ai.Refresh is the CALLER's
// responsibility — addcmd does it once per command; preset.Apply does
// it once per `one create --preset` after all projects land.
//
// interactive controls whether the deploy-target prompts and the
// (rare) deploy backend picker may interrupt for input. Preset paths
// always pass false; addcmd passes !flags.yes && output.CanPrompt().
//
// On render failure, ApplyProject only rolls back directories it
// created itself (mirrors cbb95a1's guard) — never touches a
// pre-existing tree.
func ApplyProject(ctx context.Context, projectRoot string, in ProjectInput, interactive bool) (ProjectResult, error) {
	if in.Template == nil {
		return ProjectResult{}, fmt.Errorf("preset.ApplyProject: nil template")
	}
	if !workspace.IsValidProjectName(in.Name) {
		return ProjectResult{}, cliErrors.New(cliErrors.INVALID_NAME,
			fmt.Sprintf("项目名称格式不合法: %q", in.Name))
	}

	entry := in.Template
	categoryDir, err := categoryDirFor(string(entry.Category))
	if err != nil {
		return ProjectResult{}, err
	}
	targetDir := filepath.Join(projectRoot, categoryDir, in.Name)

	_, statErr := os.Stat(targetDir)
	createdFromScratch := os.IsNotExist(statErr)

	if exists, _ := dirNonEmpty(targetDir); exists {
		return ProjectResult{}, cliErrors.New(cliErrors.TARGET_EXISTS,
			fmt.Sprintf("项目目录已存在: %s", targetDir)).
			WithContext(map[string]any{
				"subproject_name": in.Name,
				"target_path":     targetDir,
			})
	}

	templateLocalID, err := parseLocalTemplateID(entry.Repo)
	if err != nil {
		return ProjectResult{}, err
	}

	packageManager := defaultPackageManagerFor(string(entry.Toolchain))
	vars := template.CommonVariables(in.Name, packageManager)

	if err := prompt.Spin(fmt.Sprintf("正在生成模板 %s", entry.ID), func() error {
		return template.Render(templateLocalID, targetDir, vars)
	}); err != nil {
		if createdFromScratch {
			_ = os.RemoveAll(targetDir)
		}
		return ProjectResult{}, err
	}
	prompt.Step(fmt.Sprintf("模板渲染完成 → %s", in.Name))

	relDir, err := filepath.Rel(projectRoot, targetDir)
	if err != nil {
		relDir = filepath.Join(categoryDir, in.Name)
	}

	manifestPM := manifestPackageManagerFor(string(entry.Toolchain), packageManager)
	if err := workspace.UpsertManifestProject(projectRoot, workspace.ManifestProjectInput{
		Name:           in.Name,
		RelativeDir:    relDir,
		TemplateID:     entry.ID,
		Toolchain:      string(entry.Toolchain),
		PackageManager: manifestPM,
	}); err != nil {
		return ProjectResult{}, err
	}

	// Pick the deploy backend before applying defaults. addcmd's prompt
	// path falls through unchanged because pickDeployBackend short-
	// circuits non-interactive callers + single-compat templates.
	deployBackend, err := pickDeployBackend(entry, in.Deploy, interactive)
	if err != nil {
		return ProjectResult{}, err
	}

	if err := applyTemplateDefaults(projectRoot, entry, in.Name, deployBackend); err != nil {
		return ProjectResult{}, err
	}

	effectiveDeploy := deployBackend
	if effectiveDeploy == "" && entry.Defaults != nil {
		effectiveDeploy = entry.Defaults["deploy"]
	}
	if effectiveDeploy == "kustomize" && entry.Defaults != nil && entry.Defaults["container"] != "" {
		containerBackend := strings.TrimSpace(in.Container)
		if containerBackend == "" {
			containerBackend = "dockerhub"
		}
		if err := workspace.SetProjectContainerKind(projectRoot, in.Name, containerBackend); err != nil {
			return ProjectResult{}, err
		}
	}

	if err := promptDeployTargets(projectRoot, entry, in.Name, interactive, deployBackend); err != nil {
		return ProjectResult{}, err
	}

	addManifest, _ := workspace.ReadManifest(projectRoot)
	var thisSub *workspace.ManifestProject
	for i := range addManifest.Projects {
		if addManifest.Projects[i].Name == in.Name {
			thisSub = &addManifest.Projects[i]
			break
		}
	}
	addSelected := workspace.SelectionForProject(addManifest, thisSub)
	addCIProvider := "ci/github-actions"
	if err := prompt.Spin("正在同步基础设施 / CI 配置", func() error {
		if err := infra.SyncSubproject(infra.Options{
			ProjectRoot:    projectRoot,
			TargetDir:      targetDir,
			ProjectName:    in.Name,
			TemplateID:     entry.ID,
			Toolchain:      toolchain.Toolchain(entry.Toolchain),
			PackageManager: toolchain.PackageManager(packageManager),
			Selected:       addSelected,
		}); err != nil {
			return err
		}
		_, err := ci.Sync(ci.SyncOptions{
			ProjectRoot:    projectRoot,
			TargetDir:      targetDir,
			ProjectName:    in.Name,
			Toolchain:      toolchain.Toolchain(entry.Toolchain),
			PackageManager: toolchain.PackageManager(packageManager),
			ProviderID:     addCIProvider,
		})
		return err
	}); err != nil {
		return ProjectResult{}, err
	}

	compatManifest, _ := workspace.ReadManifest(projectRoot)
	compatSelection := workspace.SelectionForProject(compatManifest, nil)
	warnings := template.CheckAllowedBackends(*entry, compatSelection, "")

	_ = ctx // ctx is currently unused; reserved for future cancellation hooks.

	return ProjectResult{
		Name:           in.Name,
		TargetPath:     targetDir,
		TemplateID:     entry.ID,
		Toolchain:      string(entry.Toolchain),
		PackageManager: manifestPM,
		DeployBackend:  effectiveDeploy,
		Warnings:       warningMessages(warnings),
	}, nil
}

// pickDeployBackend resolves the deploy backend for a subproject.
// Precedence: explicit override (e.g. addcmd's --deploy-provider or
// preset's `@<deploy>` segment) > interactive multi-option prompt >
// template default ("" means "let applyTemplateDefaults use the
// registry default").
func pickDeployBackend(tpl *template.Template, flagDeploy string, interactive bool) (string, error) {
	if tpl == nil {
		return "", nil
	}
	flagDeploy = strings.TrimSpace(flagDeploy)
	compat := []string{}
	if tpl.Compat != nil {
		compat = append(compat, tpl.Compat["deploy"]...)
	}
	if flagDeploy != "" {
		if len(compat) > 0 && !containsString(compat, flagDeploy) {
			return "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
				fmt.Sprintf("deploy %q 不在模板 %s 的 compat.deploy 列表里（合法值：%v）", flagDeploy, tpl.ID, compat))
		}
		return flagDeploy, nil
	}
	if !interactive || len(compat) <= 1 {
		return "", nil
	}
	defaultID := ""
	if tpl.Defaults != nil {
		defaultID = strings.TrimSpace(tpl.Defaults["deploy"])
	}
	options := make([]prompt.Option[string], 0, len(compat))
	for _, b := range compat {
		label := b
		if b == defaultID {
			label = b + "  (default)"
		}
		options = append(options, prompt.Option[string]{Label: label, Value: b})
	}
	return prompt.Select("选择该项目的 deploy 后端（compat.deploy 列表）", options)
}

// applyTemplateDefaults writes the template's per-domain backend
// selections into the manifest, honouring the deploy override and
// skipping the container default when a non-kustomize deploy is in use
// (same precedent as the pre-extraction version in addcmd).
func applyTemplateDefaults(projectRoot string, tpl *template.Template, subprojectName, deployOverride string) error {
	if tpl == nil || len(tpl.Defaults) == 0 {
		return nil
	}
	for domain, backend := range tpl.Defaults {
		if domain == "" || backend == "" {
			continue
		}
		if domain == "deploy" && strings.TrimSpace(deployOverride) != "" {
			backend = deployOverride
		}
		if domain == "container" && strings.TrimSpace(deployOverride) != "" && deployOverride != "kustomize" {
			continue
		}
		id := domain + "/" + backend
		switch domain {
		case "container", "deploy":
			if _, err := workspace.SetPerProjectSelection(projectRoot, domain, id, subprojectName); err != nil {
				return err
			}
		case "env":
			m, err := workspace.ReadManifest(projectRoot)
			if err != nil {
				return err
			}
			if workspace.EnvBackend(m) != "" {
				continue
			}
			if _, err := workspace.SetWorkspaceSelection(projectRoot, domain, id); err != nil {
				return err
			}
		case "ci", "dev":
			continue
		}
	}
	return nil
}

// promptDeployTargets fills deploy-target metadata that lives in the
// manifest (k8s namespace, kustomization path, S3 bucket). Mirrors the
// pre-extraction version; interactive=false skips the prompts but
// still applies deterministic defaults (workspace.id → S3 bucket etc).
func promptDeployTargets(projectRoot string, tpl *template.Template, subprojectName string, interactive bool, deployOverride string) error {
	if tpl == nil {
		return nil
	}
	defaults := tpl.Defaults
	if len(defaults) == 0 && strings.TrimSpace(deployOverride) == "" {
		return nil
	}

	effectiveBackend := strings.TrimSpace(deployOverride)
	if effectiveBackend == "" {
		effectiveBackend = defaults["deploy"]
	}

	if workspace.IsS3CompatibleDeploy(effectiveBackend) {
		m, err := workspace.ReadManifest(projectRoot)
		if err != nil {
			return err
		}
		if bucket := workspace.ExplicitDeployBucketForProject(m, subprojectName); bucket != "" {
			return nil
		}
		if projectID := workspace.WorkspaceID(m); projectID != "" {
			return workspace.SetProjectDeployBucket(projectRoot, subprojectName, projectID)
		}
		if !interactive {
			return nil
		}
		v, err := prompt.Text(
			fmt.Sprintf("S3 bucket — writes projects[%s].deploy.bucket (legacy manifest without workspace.id only)",
				subprojectName),
			"", nil)
		if err != nil {
			return err
		}
		v = strings.TrimSpace(v)
		if v != "" {
			if err := workspace.SetProjectDeployBucket(projectRoot, subprojectName, v); err != nil {
				return err
			}
		}
	}

	if !interactive {
		return nil
	}

	if backend := effectiveBackend; backend == "kustomize" {
		m, err := workspace.ReadManifest(projectRoot)
		if err != nil {
			return err
		}
		explicitNamespace := workspace.ExplicitDeployNamespace(m)
		defaultNamespace := workspace.WorkspaceID(m)
		hasNamespace := explicitNamespace != "" || defaultNamespace != ""
		hasPath := workspace.DeployKustomizationPath(m) != ""
		if hasNamespace && hasPath {
			return nil
		}
		ns := explicitNamespace
		if ns == "" && defaultNamespace == "" {
			v, err := prompt.Text(
				"k8s namespace — 写入 manifest.deploy.namespace（workspace 级，所有 k8s 项目共享）",
				"default", nil)
			if err != nil {
				return err
			}
			ns = strings.TrimSpace(v)
		}
		path := workspace.DeployKustomizationPath(m)
		if path == "" {
			path = "kustomize/overlays/prod"
		}
		if err := workspace.SetWorkspaceDeployTarget(projectRoot, ns, path); err != nil {
			return err
		}
	}

	return nil
}

// warningMessages flattens compat warnings to strings; empty / nil is
// returned untouched so callers may apply omitempty.
func warningMessages(ws []template.Warning) []string {
	if len(ws) == 0 {
		return nil
	}
	out := make([]string, 0, len(ws))
	for _, w := range ws {
		out = append(out, w.Message())
	}
	return out
}

// parseLocalTemplateID strips the `local:` prefix and validates the slug.
func parseLocalTemplateID(repo string) (string, error) {
	if !strings.HasPrefix(repo, template.LocalTemplatePrefix) {
		return "", cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND,
			fmt.Sprintf("Phase 4a 仅支持 local: 前缀模板；待 phase 5 支持远程下载: %s", repo))
	}
	id := strings.TrimSpace(strings.TrimPrefix(repo, template.LocalTemplatePrefix))
	id = strings.TrimLeft(id, "/")
	if id == "" {
		return "", cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND,
			fmt.Sprintf("本地模板配置无效：%s。请使用 local:<template-name> 格式。", repo))
	}
	for _, seg := range strings.FieldsFunc(id, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return "", cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND,
				fmt.Sprintf("本地模板配置无效：%s。不允许使用 \"..\" 路径。", repo))
		}
	}
	return id, nil
}

func categoryDirFor(category string) (string, error) {
	switch category {
	case "frontend":
		return "apps", nil
	case "backend":
		return "services", nil
	case "library":
		return "packages", nil
	default:
		return "", cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND,
			fmt.Sprintf("未知模板分类: %s", category))
	}
}

func defaultPackageManagerFor(tc string) string {
	if tc == "go" {
		return ""
	}
	return "pnpm"
}

func manifestPackageManagerFor(tc, pm string) string {
	if tc == "go" {
		return ""
	}
	return pm
}

func dirNonEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return len(entries) > 0, nil
}
