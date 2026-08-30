package workspace

import (
	"context"
	"fmt"
	"strings"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

// WorkspaceProfileSettingsSchema versions the safe workspace-level profile
// binding projection. It deliberately exposes only the selected profile name
// and resolution source; profile values and credentials remain private to the
// machine profile service.
const WorkspaceProfileSettingsSchema = "one-cli/workspace-profile/v1"

type WorkspaceProfileSettings struct {
	Schema          string             `json:"schema"`
	Root            string             `json:"root"`
	Environment     string             `json:"environment"`
	Domain          string             `json:"domain"`
	Backend         string             `json:"backend,omitempty"`
	Configurable    bool               `json:"configurable"`
	SelectedProfile string             `json:"selectedProfile"`
	Profile         *ProjectProfileRef `json:"profile,omitempty"`
}

// WorkspaceEnvironmentProfile reads the environment backend from the shared
// manifest and resolves its effective machine-local Workspace profile. It is
// a projection only and never writes one.manifest.json.
func (s *Service) WorkspaceEnvironmentProfile(
	_ context.Context,
	root, environment string,
) (WorkspaceProfileSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workspaceEnvironmentProfile(root, environment)
}

func (s *Service) workspaceEnvironmentProfile(
	root, environment string,
) (WorkspaceProfileSettings, error) {
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		return WorkspaceProfileSettings{}, err
	}
	environment, err = validateEnvironment(manifest, environment)
	if err != nil {
		return WorkspaceProfileSettings{}, err
	}
	backend := strings.TrimSpace(workspacecore.EnvBackend(manifest))
	profileEnvironment := workspacecore.ProfileBindingEnvironment(manifest, environment)
	settings := WorkspaceProfileSettings{
		Schema:      WorkspaceProfileSettingsSchema,
		Root:        root,
		Environment: environment,
		Domain:      string(profile.DomainEnv),
		Backend:     backend,
	}
	if backend == "" {
		return settings, nil
	}
	spec, ok := s.catalog.Lookup(catalog.DomainEnv, backend)
	if !ok {
		return WorkspaceProfileSettings{}, fmt.Errorf(
			"%w: unknown env backend %q", ErrInvalidInput, backend,
		)
	}
	settings.Configurable = spec.Profile.Configurable
	if !settings.Configurable {
		return settings, nil
	}
	settings.Profile = s.resolveProfileRef(
		manifest, root, profileEnvironment, "", profile.DomainEnv, backend,
	)
	settings.SelectedProfile, err = s.directProfileSelection(
		root, "", profileEnvironment, profile.DomainEnv, backend, settings.Profile,
	)
	if err != nil {
		return WorkspaceProfileSettings{}, err
	}
	return settings, nil
}

// UpdateWorkspaceEnvironmentProfile changes only the machine-local Workspace
// binding. The backend remains owned by one.manifest.json and is therefore
// read-only here. An empty profile explicitly removes the Workspace binding
// and falls back to the normal resolver precedence.
func (s *Service) UpdateWorkspaceEnvironmentProfile(
	_ context.Context,
	root, environment, profileName string,
) (WorkspaceProfileSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		return WorkspaceProfileSettings{}, err
	}
	backend := strings.TrimSpace(workspacecore.EnvBackend(manifest))
	if backend == "" {
		return WorkspaceProfileSettings{}, fmt.Errorf(
			"%w: env backend is not configured", ErrInvalidInput,
		)
	}
	spec, ok := s.catalog.Lookup(catalog.DomainEnv, backend)
	if !ok {
		return WorkspaceProfileSettings{}, fmt.Errorf(
			"%w: unknown env backend %q", ErrInvalidInput, backend,
		)
	}
	if !spec.Profile.Configurable {
		return WorkspaceProfileSettings{}, fmt.Errorf(
			"%w: backend %s/%s does not accept a profile",
			ErrInvalidInput, profile.DomainEnv, backend,
		)
	}
	if s.profiles == nil {
		return WorkspaceProfileSettings{}, fmt.Errorf("workspace: profile service is unavailable")
	}
	environment, err = validateEnvironment(manifest, environment)
	if err != nil {
		return WorkspaceProfileSettings{}, err
	}
	bindingEnvironment := workspacecore.ProfileBindingEnvironment(manifest, environment)
	if err := s.changeProfileBinding(
		manifest, root, "", bindingEnvironment, profile.DomainEnv, backend, profileName,
	); err != nil {
		return WorkspaceProfileSettings{}, err
	}
	return s.workspaceEnvironmentProfile(root, environment)
}

func workspaceDirectSource(environment string) string {
	if environment != "" {
		return "workspace-environment"
	}
	return "workspace"
}
