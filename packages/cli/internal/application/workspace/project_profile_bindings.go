package workspace

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

// UpdateProjectProfileBinding changes only a machine-local profile choice.
// Project metadata and backend selection remain owned by one.manifest.json,
// which this application boundary treats as a read-only input.
func (s *Service) UpdateProjectProfileBinding(
	ctx context.Context,
	root, projectName, domainName, environment, profileName string,
) (ProjectSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	manifest, project, err := readProject(root, projectName)
	if err != nil {
		return ProjectSettings{}, err
	}
	domain, backend, err := s.projectProfileBackend(manifest, project.Name, domainName)
	if err != nil {
		return ProjectSettings{}, err
	}
	environment, err = validateEnvironment(manifest, environment)
	if err != nil {
		return ProjectSettings{}, err
	}
	bindingEnvironment := workspacecore.ProfileBindingEnvironment(manifest, environment)
	if err := s.changeProfileBinding(
		manifest, root, project.Name, bindingEnvironment, domain, backend, profileName,
	); err != nil {
		return ProjectSettings{}, err
	}
	return s.projectSettings(ctx, root, project.Name, environment)
}

func readProject(
	root, projectName string,
) (*workspacecore.Manifest, *workspacecore.ManifestProject, error) {
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		return nil, nil, err
	}
	project := findProject(manifest, strings.TrimSpace(projectName))
	if project == nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrProjectNotFound, projectName)
	}
	return manifest, project, nil
}

func (s *Service) projectProfileBackend(
	manifest *workspacecore.Manifest,
	projectName, domainName string,
) (profile.Domain, string, error) {
	var domain profile.Domain
	var backend string
	switch strings.TrimSpace(domainName) {
	case string(profile.DomainEnv):
		domain = profile.DomainEnv
		backend = workspacecore.EnvBackend(manifest)
	case string(profile.DomainDeploy):
		domain = profile.DomainDeploy
		backend = effectiveDeployBackend(manifest, projectName)
	case string(profile.DomainContainer):
		domain = profile.DomainContainer
		backend = workspacecore.ContainerKindForProject(manifest, projectName)
	default:
		return "", "", fmt.Errorf(
			"%w: profile domain must be env, deploy, or container", ErrInvalidInput,
		)
	}
	backend = strings.TrimSpace(backend)
	if backend == "" {
		return "", "", fmt.Errorf(
			"%w: %s backend is not configured in one.manifest.json", ErrInvalidInput, domain,
		)
	}
	spec, ok := s.catalog.Lookup(catalog.Domain(domain), backend)
	if !ok {
		return "", "", fmt.Errorf(
			"%w: unknown %s backend %q in one.manifest.json", ErrInvalidInput, domain, backend,
		)
	}
	if !spec.Profile.Configurable {
		return "", "", fmt.Errorf(
			"%w: backend %s/%s does not accept a profile", ErrInvalidInput, domain, backend,
		)
	}
	return domain, backend, nil
}

var environmentIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

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

func (s *Service) changeProfileBinding(
	manifest *workspacecore.Manifest,
	root, projectName, environment string,
	domain profile.Domain,
	backend, requested string,
) error {
	if s.profiles == nil {
		return fmt.Errorf("workspace: profile service is unavailable")
	}
	name := strings.TrimSpace(requested)
	if name != "" {
		if _, err := s.profiles.Resolve(profile.ResolveInput{
			Domain:        domain,
			Backend:       backend,
			FlagOverride:  name,
			WorkspaceID:   workspacecore.WorkspaceID(manifest),
			WorkspaceRoot: root,
			ProjectName:   projectName,
			Environment:   environment,
			SkipDefault:   true,
		}); err != nil {
			return fmt.Errorf("%w: profile %q is not usable: %v", ErrInvalidInput, name, err)
		}
	}

	if environment != "" {
		if name == "" {
			return s.profiles.UnbindEnvironmentProfile(
				root, projectName, environment, domain, backend,
			)
		}
		workspaceID, workspaceName := manifestIdentity(manifest)
		return s.profiles.BindEnvironmentProfile(
			workspaceID, workspaceName, root, projectName, environment, domain, backend, name,
		)
	}

	workspaceID, workspaceName := manifestIdentity(manifest)
	if workspaceID == "" {
		if name == "" {
			// A legacy manifest without workspace.id cannot have an addressable
			// legacy binding. Treat explicit unbind as an idempotent no-op; never
			// upgrade the manifest merely to manufacture an identity.
			return nil
		}
		return fmt.Errorf("%w: workspace id is required to bind a profile", ErrInvalidInput)
	}
	if name == "" {
		return s.profiles.UnbindWorkspaceProfile(workspaceID, projectName, domain, backend)
	}
	return s.profiles.BindWorkspaceProfile(
		workspaceID, workspaceName, root, projectName, domain, backend, name,
	)
}

func manifestIdentity(manifest *workspacecore.Manifest) (id, name string) {
	if manifest == nil || manifest.Workspace == nil {
		return "", ""
	}
	return strings.TrimSpace(manifest.Workspace.ID), strings.TrimSpace(manifest.Workspace.Name)
}
