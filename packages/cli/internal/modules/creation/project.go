package creation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

// ProjectInput names everything the creation workflow needs to materialise one
// Project from a Template into an existing Workspace.
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
	// DeferDeployment leaves deployment and image-registry selections unset.
	// Ordinary `one add` uses this so the first `one deploy` owns the choice.
	DeferDeployment bool
	// ConfigureDeployTargets preserves the interactive add path that writes
	// kustomize target metadata. Presets and first-deploy setup leave it false.
	ConfigureDeployTargets bool
	DeployTarget           DeploymentTarget
}

type DeploymentTarget struct {
	Bucket            string
	Namespace         string
	KustomizationPath string
}

// ProjectResult is the transport-neutral outcome of materialising a Template,
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

// materializeProject renders the template into projectRoot, upserts the
// manifest, applies template defaults (with the optional deploy
// override), and runs infra sync. CI is intentionally not generated as a
// side effect of adding a project. The Service owns AI-guide refresh timing.
//
// On render failure, materializeProject only rolls back directories it
// created itself (mirrors cbb95a1's guard) — never touches a
// pre-existing tree.
func materializeProject(ctx context.Context, projectRoot string, in ProjectInput) (ProjectResult, error) {
	if in.Template == nil {
		return ProjectResult{}, fmt.Errorf("creation: template is required")
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

	if err := template.Render(templateLocalID, targetDir, vars); err != nil {
		if createdFromScratch {
			_ = os.RemoveAll(targetDir)
		}
		return ProjectResult{}, err
	}
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

	deployBackend := ""
	if !in.DeferDeployment {
		deployBackend, err = pickDeployBackend(entry, in.Deploy)
		if err != nil {
			return ProjectResult{}, err
		}
	}

	if err := applyTemplateDefaults(projectRoot, entry, in.Name, deployBackend, !in.DeferDeployment); err != nil {
		return ProjectResult{}, err
	}

	effectiveDeploy := deployBackend
	if !in.DeferDeployment && effectiveDeploy == "" && entry.Defaults != nil {
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

	if !in.DeferDeployment {
		if err := applyDeployTargets(
			projectRoot, entry, in.Name, deployBackend, in.ConfigureDeployTargets, in.DeployTarget,
		); err != nil {
			return ProjectResult{}, err
		}
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
	if err := syncProject(syncProjectOptions{
		ProjectRoot:    projectRoot,
		TargetDir:      targetDir,
		ProjectName:    in.Name,
		TemplateID:     entry.ID,
		Toolchain:      toolchain.Toolchain(entry.Toolchain),
		PackageManager: toolchain.PackageManager(packageManager),
		Selected:       addSelected,
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

// ConfigureProjectDeployment applies a compatible deployment choice to an
// existing project and generates only the deployment/container artifacts.
// It is the mutation step used by the first-deploy wizard; project source is
// never re-rendered.
func configureProjectDeployment(ctx context.Context, projectRoot string, tpl *template.Template, projectName, backend string) error {
	if tpl == nil {
		return fmt.Errorf("creation: template is required")
	}
	backend, err := pickDeployBackend(tpl, backend)
	if err != nil {
		return err
	}
	if backend == "" {
		return cliErrors.New(cliErrors.BACKEND_NOT_ENABLED, "deployment target is required")
	}
	if err := applyTemplateDefaults(projectRoot, tpl, projectName, backend, true); err != nil {
		return err
	}
	if backend == "kustomize" {
		containerKind := "docker"
		if tpl.Defaults != nil && strings.TrimSpace(tpl.Defaults["container"]) != "" {
			containerKind = strings.TrimSpace(tpl.Defaults["container"])
		}
		if err := workspace.SetProjectContainerKind(projectRoot, projectName, containerKind); err != nil {
			return err
		}
	}
	if err := applyDeployTargets(projectRoot, tpl, projectName, backend, false, DeploymentTarget{}); err != nil {
		return err
	}

	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return err
	}
	var project *workspace.ManifestProject
	for i := range m.Projects {
		if m.Projects[i].Name == projectName {
			project = &m.Projects[i]
			break
		}
	}
	if project == nil {
		return cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND, "project not found: "+projectName)
	}
	targetDir := filepath.Join(projectRoot, filepath.FromSlash(project.RelativeDir))
	packageManager := project.PackageManager
	if packageManager == "" {
		packageManager = defaultPackageManagerFor(project.Toolchain)
	}
	selected := workspace.SelectionForProject(m, project)
	if err := syncProject(syncProjectOptions{
		ProjectRoot:    projectRoot,
		TargetDir:      targetDir,
		ProjectName:    project.Name,
		TemplateID:     project.TemplateID,
		Toolchain:      toolchain.Toolchain(project.Toolchain),
		PackageManager: toolchain.PackageManager(packageManager),
		Selected:       selected,
	}); err != nil {
		return err
	}
	_ = ctx
	return nil
}

// pickDeployBackend resolves the deploy backend for a subproject.
// Precedence: explicit override (e.g. addcmd's --deploy-provider or
// preset's `@<deploy>` segment) > interactive multi-option prompt >
// template default ("" means "let applyTemplateDefaults use the
// registry default").
func pickDeployBackend(tpl *template.Template, flagDeploy string) (string, error) {
	if tpl == nil {
		return "", nil
	}
	flagDeploy = strings.TrimSpace(flagDeploy)
	compat := []string{}
	if tpl.Compat != nil {
		compat = append(compat, tpl.Compat["deploy"]...)
	}
	if flagDeploy != "" {
		if len(compat) > 0 && !slices.Contains(compat, flagDeploy) {
			return "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
				fmt.Sprintf("deploy %q 不在模板 %s 的 compat.deploy 列表里（合法值：%v）", flagDeploy, tpl.ID, compat))
		}
		return flagDeploy, nil
	}
	return "", nil
}

// applyTemplateDefaults writes the template's per-domain backend
// selections into the manifest, honouring the deploy override and
// skipping the container default when a non-kustomize deploy is in use
// (same precedent as the pre-extraction version in addcmd).
func applyTemplateDefaults(projectRoot string, tpl *template.Template, subprojectName, deployOverride string, includeDeployment bool) error {
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
		if !includeDeployment && (domain == "deploy" || domain == "container") {
			continue
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
func applyDeployTargets(
	projectRoot string,
	tpl *template.Template,
	subprojectName string,
	deployOverride string,
	configure bool,
	target DeploymentTarget,
) error {
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
		if configure && strings.TrimSpace(target.Bucket) != "" {
			if err := workspace.SetProjectDeployBucket(projectRoot, subprojectName, strings.TrimSpace(target.Bucket)); err != nil {
				return err
			}
		}
	}

	if !configure {
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
			ns = strings.TrimSpace(target.Namespace)
		}
		path := workspace.DeployKustomizationPath(m)
		if path == "" {
			path = strings.TrimSpace(target.KustomizationPath)
			if path == "" {
				path = "kustomize/overlays/prod"
			}
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
