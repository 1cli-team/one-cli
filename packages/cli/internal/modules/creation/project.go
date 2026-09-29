package creation

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/hooks"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/miseconfig"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// ProjectInput names everything the creation workflow needs to materialise one
// Project from a Template into an existing Workspace.
// Resolve the template before calling here.
type ProjectInput struct {
	// Template is the resolved registry entry (caller did the lookup).
	Template *template.Template
	// Name is the subproject name (validated for the IsValidProjectName
	// regex by the caller).
	Name string
}

// ProjectResult is the transport-neutral outcome of materialising a Template.
type ProjectResult struct {
	Name           string
	TargetPath     string
	TemplateID     string
	Toolchain      string
	PackageManager string
	Warnings       []string
}

// materializeProject renders the template into projectRoot, upserts the
// manifest, applies template defaults, and synchronizes workspace tooling.
//
// On render failure, materializeProject only rolls back directories it
// created itself (mirrors cbb95a1's guard) — never touches a
// pre-existing tree.
func materializeProject(ctx context.Context, projectRoot string, in ProjectInput) (ProjectResult, error) {
	projectRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return ProjectResult{}, err
	}
	unlock, err := fsutil.WorkspaceLock(ctx, projectRoot, "creation")
	if err != nil {
		return ProjectResult{}, err
	}
	defer unlock()
	// Share the mise writer's lock while publishing projected configuration.
	unlockMise, err := fsutil.WorkspaceLock(ctx, projectRoot, "mise")
	if err != nil {
		return ProjectResult{}, err
	}
	defer unlockMise()
	miseEnabled := miseconfig.Enabled(projectRoot)
	files := fsutil.NewFilePlan(projectRoot)
	if _, err := files.Read(workspace.ManifestFilename); err != nil {
		return ProjectResult{}, err
	}
	manifest, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return ProjectResult{}, err
	}
	if in.Template == nil {
		return ProjectResult{}, i18n.Errorf("creation.template_required")
	}
	if !workspace.IsValidProjectName(in.Name) {
		return ProjectResult{}, cliErrors.New(cliErrors.INVALID_NAME,
			i18n.Tf("add.name_invalid", in.Name))
	}

	entry := in.Template
	categoryDir, err := categoryDirFor(string(entry.Category))
	if err != nil {
		return ProjectResult{}, err
	}
	targetDir := filepath.Join(projectRoot, categoryDir, in.Name)
	if err := fsutil.SafeWritePath(projectRoot, targetDir); err != nil {
		return ProjectResult{}, err
	}

	_, statErr := os.Stat(targetDir)
	createdFromScratch := os.IsNotExist(statErr)

	if exists, _ := dirNonEmpty(targetDir); exists {
		return ProjectResult{}, cliErrors.New(cliErrors.TARGET_EXISTS,
			i18n.Tf("creation.directory_exists", targetDir)).
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
	if entry.Toolchain == "node" {
		packageManager, err = nodePackageManager(files)
		if err != nil {
			return ProjectResult{}, err
		}
	}
	vars := template.CommonVariables(in.Name, packageManager)

	if err := template.Render(templateLocalID, targetDir, vars); err != nil {
		if createdFromScratch {
			_ = os.RemoveAll(targetDir)
		}
		return ProjectResult{}, err
	}
	registered := false
	defer func() {
		if !registered && createdFromScratch {
			_ = os.RemoveAll(targetDir)
		}
	}()
	relDir, err := filepath.Rel(projectRoot, targetDir)
	if err != nil {
		relDir = filepath.Join(categoryDir, in.Name)
	}

	if entry.Toolchain == "node" {
		dirs, err := workspace.NodeProjectPackageDirs(projectRoot, relDir, files.Read)
		if err != nil {
			return ProjectResult{}, err
		}
		_ = dirs
	}

	manifestPM := manifestPackageManagerFor(string(entry.Toolchain), packageManager)
	newProject := workspace.ManifestProject{
		Name:        in.Name,
		RelativeDir: filepath.ToSlash(relDir),
		Toolchain:   string(entry.Toolchain),
	}
	for _, p := range manifest.Projects {
		if p.RelativeDir == newProject.RelativeDir || p.Name == in.Name {
			return ProjectResult{}, cliErrors.New(cliErrors.TARGET_EXISTS,
				i18n.Tf("creation.project_registered", in.Name))
		}
	}
	manifest.Projects = append(manifest.Projects, newProject)
	if err := planLanguages(files, manifest); err != nil {
		return ProjectResult{}, err
	}
	if entry.Toolchain == "node" {
		if err := configureNodePackage(files, relDir, vars["projectNameKebabCase"], packageManager); err != nil {
			return ProjectResult{}, err
		}
		if err := validateNodePackageNames(files, manifest); err != nil {
			return ProjectResult{}, err
		}
	}
	if hk, err := files.Read(workspace.HooksConfigFilename); err != nil {
		return ProjectResult{}, err
	} else if hk != nil {
		if err := hooks.PlanFiles(files, manifest); err != nil {
			return ProjectResult{}, err
		}
	}
	manifestRaw, err := workspace.MarshalManifest(manifest)
	if err != nil {
		return ProjectResult{}, err
	}
	if err := files.Set(workspace.ManifestFilename, manifestRaw, 0o644); err != nil {
		return ProjectResult{}, err
	}
	if miseEnabled {
		plan, err := miseconfig.BuildWithFiles(projectRoot, miseconfig.Options{}, files.Overlay())
		if err != nil {
			return ProjectResult{}, err
		}
		for path, content := range plan.ReadInputs() {
			if err := files.Expect(path, content); err != nil {
				return ProjectResult{}, err
			}
		}
		for _, change := range plan.Changes {
			if err := files.Set(change.Path, []byte(change.After), 0o644); err != nil {
				return ProjectResult{}, err
			}
		}
	}
	if err := files.Apply(ctx); err != nil {
		return ProjectResult{}, err
	}
	registered = true

	compatManifest, _ := workspace.ReadManifest(projectRoot)
	compatSelection := workspace.SelectionForProject(compatManifest, nil)
	warnings := template.CheckAllowedBackends(*entry, compatSelection, "")

	return ProjectResult{
		Name:           in.Name,
		TargetPath:     targetDir,
		TemplateID:     entry.ID,
		Toolchain:      string(entry.Toolchain),
		PackageManager: manifestPM,
		Warnings:       warningMessages(warnings),
	}, nil
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
			i18n.Tf("creation.remote_template_unsupported", repo))
	}
	id := strings.TrimSpace(strings.TrimPrefix(repo, template.LocalTemplatePrefix))
	id = strings.TrimLeft(id, "/")
	if id == "" {
		return "", cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND,
			i18n.Tf("creation.template_invalid", repo))
	}
	for _, seg := range strings.FieldsFunc(id, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return "", cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND,
				i18n.Tf("creation.template_parent_path", repo))
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
			i18n.Tf("creation.category_unknown", category))
	}
}

func defaultPackageManagerFor(tc string) string {
	if tc == "go" || tc == string(template.ToolchainNone) {
		return ""
	}
	return "pnpm"
}

func manifestPackageManagerFor(tc, pm string) string {
	if tc == "go" || tc == string(template.ToolchainNone) {
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
