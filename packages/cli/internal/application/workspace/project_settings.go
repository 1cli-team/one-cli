package workspace

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

// ProjectSettingsSchema versions the safe, project-focused Dashboard
// projection. It deliberately contains no profile values or credentials.
const ProjectSettingsSchema = "one-cli/workspace-project/v1"

type ProjectSettings struct {
	Schema      string                 `json:"schema"`
	Root        string                 `json:"root"`
	Environment string                 `json:"environment"`
	Revision    string                 `json:"revision"`
	Project     ProjectSettingsProject `json:"project"`
}

type ProjectSettingsProject struct {
	Name                  string                     `json:"name"`
	RelativeDir           string                     `json:"relativeDir"`
	Kind                  string                     `json:"kind"`
	TemplateID            string                     `json:"templateId,omitempty"`
	Toolchain             string                     `json:"toolchain,omitempty"`
	PackageManager        string                     `json:"packageManager,omitempty"`
	DevAvailable          bool                       `json:"devAvailable"`
	Build                 ProjectBuildSettings       `json:"build"`
	Tasks                 *ProjectTasks              `json:"tasks,omitempty"`
	DefaultEnvironment    string                     `json:"defaultEnvironment,omitempty"`
	AvailableEnvironments []string                   `json:"availableEnvironments"`
	Environment           ProjectEnvironmentSettings `json:"environment"`
}

// ProjectBuildSettings is a read-only projection of the live build task.
// Source is project-relative; Status is ready, missing, or invalid.
type ProjectBuildSettings struct {
	Command string `json:"command,omitempty"`
	Source  string `json:"source,omitempty"`
	Status  string `json:"status"`
}

type ProjectEnvironmentSettings struct {
	Backend  string `json:"backend,omitempty"`
	Path     string `json:"path,omitempty"`
	Inherits bool   `json:"inherits"`
}

// ProjectSettings returns manifest-owned settings, the live build command,
// and environment metadata. Credential values never enter the response.
func (s *Service) ProjectSettings(
	ctx context.Context,
	root, projectName, environment string,
) (ProjectSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.projectSettings(ctx, root, projectName, environment)
}

func (s *Service) projectSettings(
	ctx context.Context,
	root, projectName, environment string,
) (ProjectSettings, error) {
	manifest, revision, err := workspacecore.ReadManifestSnapshot(root)
	if err != nil {
		return ProjectSettings{}, err
	}
	project := findProject(manifest, strings.TrimSpace(projectName))
	if project == nil {
		return ProjectSettings{}, fmt.Errorf("%w: %s", ErrProjectNotFound, projectName)
	}
	environment, err = validateEnvironment(manifest, environment)
	if err != nil {
		return ProjectSettings{}, err
	}
	environments, defaultEnvironment := projectEnvironments(manifest)

	env := ProjectEnvironmentSettings{
		Backend:  strings.TrimSpace(workspacecore.EnvBackend(manifest)),
		Inherits: true,
		Path:     "/" + project.RelativeDir,
	}
	packageManager := ""
	if project.Toolchain == "node" {
		pkg, _ := workspacecore.ReadPackageJSON(filepath.Join(root, project.RelativeDir))
		if pkg == nil || pkg.PackageManager == "" {
			pkg, _ = workspacecore.ReadPackageJSON(root)
		}
		if pkg != nil {
			packageManager, _, _ = strings.Cut(pkg.PackageManager, "@")
		}
	}
	tasks := s.projectTasks(ctx, root, project.Name)
	devAvailable := false
	if tasks != nil && tasks.Status == "ready" {
		for _, task := range tasks.Entries {
			if task.Name == "//:"+project.Name+":dev" || task.Name == "//"+filepath.ToSlash(project.RelativeDir)+":dev" {
				devAvailable = true
			}
		}
	}
	return ProjectSettings{
		Schema:      ProjectSettingsSchema,
		Root:        root,
		Environment: environment,
		Revision:    revision,
		Project: ProjectSettingsProject{
			Name:                  project.Name,
			RelativeDir:           project.RelativeDir,
			Kind:                  projectKind(project.RelativeDir),
			TemplateID:            project.TemplateID,
			Toolchain:             project.Toolchain,
			PackageManager:        packageManager,
			DevAvailable:          devAvailable,
			Build:                 projectBuildSettings(root, *project),
			Tasks:                 tasks,
			DefaultEnvironment:    defaultEnvironment,
			AvailableEnvironments: environments,
			Environment:           env,
		},
	}, nil
}

func projectBuildSettings(root string, project workspacecore.ManifestProject) ProjectBuildSettings {
	build := ProjectBuildSettings{Status: "missing"}
	switch project.Toolchain {
	case "node":
		build.Source = "package.json#scripts.build"
	case "go":
		build.Source = "Taskfile.yml#tasks.build"
	}
	args, err := execution.ProjectOperationArgs(root, workspacecore.Project{
		Name: project.Name, RelativeDir: project.RelativeDir,
		TargetDir:  filepath.Join(root, filepath.FromSlash(project.RelativeDir)),
		Toolchain:  project.Toolchain,
		TemplateID: project.TemplateID,
	}, "build")
	if err != nil {
		var taskError *output.Error
		if !errors.As(err, &taskError) || taskError.Code != string(cliErrors.RUNTIME_TASK_NOT_FOUND) {
			build.Status = "invalid"
		}
		return build
	}
	build.Command = strings.Join(args, " ")
	build.Status = "ready"
	return build
}

func projectKind(relativeDir string) string {
	dir := strings.TrimPrefix(strings.TrimSpace(relativeDir), "./")
	switch {
	case dir == "services" || strings.HasPrefix(dir, "services/"):
		return workspacecore.ProjectKindService
	case dir == "packages" || strings.HasPrefix(dir, "packages/"):
		return workspacecore.ProjectKindPackage
	default:
		return workspacecore.ProjectKindApp
	}
}

func projectEnvironments(manifest *workspacecore.Manifest) ([]string, string) {
	return workspacecore.EnvironmentNames(manifest), "dev"
}
