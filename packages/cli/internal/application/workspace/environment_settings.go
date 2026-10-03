package workspace

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

type WorkspaceEnvironmentSettings struct {
	Environments []string `json:"environments"`
	Schema       string   `json:"schema"`
	Revision     string   `json:"revision"`
	Backend      string   `json:"backend"`
	ProjectID    string   `json:"projectId"`
	SiteURL      string   `json:"siteUrl"`
}

func (s *Service) WorkspaceEnvironment(_ context.Context, root, environment string) (WorkspaceEnvironmentSettings, error) {
	manifest, revision, e := workspacecore.ReadManifestSnapshot(root)
	if e != nil {
		return WorkspaceEnvironmentSettings{}, e
	}
	if _, e = validateEnvironment(manifest, environment); e != nil {
		return WorkspaceEnvironmentSettings{}, e
	}
	result := WorkspaceEnvironmentSettings{Environments: workspacecore.EnvironmentNames(manifest), Schema: "one-cli/workspace-environment/v1", Revision: revision, Backend: workspacecore.EnvBackend(manifest)}
	if manifest.Env != nil {
		result.ProjectID = manifest.Env.ProjectID
		result.SiteURL = manifest.Env.SiteURL
	}
	return result, nil
}

func validateEnvironment(manifest *workspacecore.Manifest, requested string) (string, error) {
	environment := strings.TrimSpace(requested)
	if requested == "" {
		return "dev", nil
	}
	if requested == environment && len(environment) <= 128 && environmentIDPattern.MatchString(environment) {
		if manifest != nil && manifest.Env != nil && !slices.Contains(workspacecore.EnvironmentNames(manifest), environment) {
			return "", fmt.Errorf("%w: %s", ErrInvalidInput, i18n.Tf("env.environment_unknown", environment, strings.Join(workspacecore.EnvironmentNames(manifest), ", ")))
		}
		return environment, nil
	}
	return "", fmt.Errorf(
		"%w: environment %q is not a safe environment id",
		ErrInvalidInput,
		environment,
	)
}

var environmentIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
