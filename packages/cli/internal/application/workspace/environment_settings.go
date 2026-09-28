package workspace

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

type WorkspaceEnvironmentSettings struct {
	Schema      string `json:"schema"`
	Revision    string `json:"revision"`
	Backend     string `json:"backend"`
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	SiteURL     string `json:"siteUrl"`
}

func (s *Service) WorkspaceEnvironment(_ context.Context, root, environment string) (WorkspaceEnvironmentSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	manifest, revision, e := workspacecore.ReadManifestSnapshot(root)
	if e != nil {
		return WorkspaceEnvironmentSettings{}, e
	}
	if _, e = validateEnvironment(manifest, environment); e != nil {
		return WorkspaceEnvironmentSettings{}, e
	}
	result := WorkspaceEnvironmentSettings{Schema: "one-cli/workspace-environment/v1", Revision: revision, Backend: workspacecore.EnvBackend(manifest)}
	if manifest.Env != nil {
		result.ProjectID = manifest.Env.ProjectID
		result.ProjectName = manifest.Env.ProjectName
		result.SiteURL = manifest.Env.SiteURL
	}
	return result, nil
}

func validateEnvironment(_ *workspacecore.Manifest, requested string) (string, error) {
	environment := strings.TrimSpace(requested)
	if requested == "" {
		return "", nil
	}
	if requested == environment && len(environment) <= 128 && environmentIDPattern.MatchString(environment) {
		return environment, nil
	}
	return "", fmt.Errorf(
		"%w: environment %q is not a safe environment id",
		ErrInvalidInput,
		environment,
	)
}

var environmentIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
