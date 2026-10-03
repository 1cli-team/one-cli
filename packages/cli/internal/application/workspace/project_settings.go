package workspace

import (
	"context"
	"fmt"
	"strings"

	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
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
	DefaultEnvironment    string                     `json:"defaultEnvironment,omitempty"`
	AvailableEnvironments []string                   `json:"availableEnvironments"`
	Environment           ProjectEnvironmentSettings `json:"environment"`
}

type ProjectEnvironmentSettings struct {
	Backend  string `json:"backend,omitempty"`
	Path     string `json:"path,omitempty"`
	Inherits bool   `json:"inherits"`
}

// ProjectSettings returns manifest-owned project and environment metadata.
// It does not inspect task configuration or retrieve credential values.
func (s *Service) ProjectSettings(
	_ context.Context,
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
	return ProjectSettings{
		Schema:      ProjectSettingsSchema,
		Root:        root,
		Environment: environment,
		Revision:    revision,
		Project: ProjectSettingsProject{
			Name:                  project.Name,
			RelativeDir:           project.RelativeDir,
			Kind:                  projectKind(project.RelativeDir),
			DefaultEnvironment:    defaultEnvironment,
			AvailableEnvironments: environments,
			Environment:           env,
		},
	}, nil
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
