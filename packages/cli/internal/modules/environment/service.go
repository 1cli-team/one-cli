// Package environment owns the complete environment-variable feature slice.
// It composes the built-in dotenv and Infisical adapters behind one workflow
// boundary so transports do not reproduce backend selection or workspace
// policy.
package environment

import (
	"fmt"
	"strings"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type Service struct {
	catalog  *catalog.Catalog
	profiles *configureapp.ProfileService
}

func NewService(
	backendCatalog *catalog.Catalog,
	profiles *configureapp.ProfileService,
) (*Service, error) {
	if backendCatalog == nil {
		return nil, fmt.Errorf("modules: environment catalog is required")
	}
	if profiles == nil {
		return nil, fmt.Errorf("modules: environment profile service is required")
	}
	return &Service{catalog: backendCatalog, profiles: profiles}, nil
}

type resolveInput struct {
	Scope        execution.Scope
	Requested    string
	AllowUnknown bool
	Capability   catalog.Capability
	Verb         string
}

type resolution struct {
	Workspace execution.Workspace
	Scope     execution.Scope
	Declared  []string
}

func (s *Service) resolve(input resolveInput) (resolution, error) {
	activeWorkspace, err := execution.ResolveWorkspaceScope(input.Scope)
	if err != nil {
		return resolution{}, err
	}
	root := activeWorkspace.Root()
	manifest := activeWorkspace.Manifest()
	backendName := workspace.EnvBackend(manifest)
	if backendName == "" {
		backendName = workspace.EnvBackendDotenv
	}
	backend, ok := s.catalog.Lookup(catalog.DomainEnv, backendName)
	if !ok {
		return resolution{}, cliErrors.New(
			cliErrors.ENV_BACKEND_INVALID,
			fmt.Sprintf("不支持的 env backend %q", backendName),
		)
	}
	if input.Capability != "" && !backend.Has(input.Capability) {
		verb := strings.TrimSpace(input.Verb)
		return resolution{}, cliErrors.New(
			cliErrors.BACKEND_VERB_NOT_SUPPORTED,
			fmt.Sprintf("%s 后端不支持 `one env %s`。", backend.Pair, verb),
		).WithContext(map[string]any{
			"domain": "env", "backend": backendName, "verb": verb,
		})
	}
	environment, declared, err := secrets.ResolveEnvName(root, input.Requested, input.AllowUnknown)
	if err != nil {
		return resolution{}, err
	}
	resolved := activeWorkspace.Scope().Derive(execution.ScopePatch{
		Environment: environment,
		Backend:     backend.ID,
	})
	return resolution{
		Workspace: activeWorkspace,
		Scope:     resolved,
		Declared:  declared,
	}, nil
}

func (s *Service) Summary(scope execution.Scope) (*Summary, error) {
	resolution, err := s.resolve(resolveInput{Scope: scope, AllowUnknown: true})
	if err != nil {
		return nil, err
	}
	manifest := resolution.Workspace.Manifest()
	source := workspace.EnvBackend(manifest)
	if source == "" {
		source = workspace.EnvBackendDotenv
	}
	defaultEnvironment := "dev"
	environments := append([]string(nil), workspace.DefaultEnvironments...)
	if manifest.Environments != nil {
		if strings.TrimSpace(manifest.Environments.Default) != "" {
			defaultEnvironment = manifest.Environments.Default
		}
		if len(manifest.Environments.Names) > 0 {
			environments = append([]string(nil), manifest.Environments.Names...)
		}
	}
	result := &Summary{
		Schema:                "one-cli/env-summary/v1",
		Source:                source,
		DefaultEnvironment:    defaultEnvironment,
		AvailableEnvironments: environments,
		Scope:                 "workspace",
		Commands:              []string{"one env set <KEY>", "one env list", "one env get <KEY>"},
	}
	if project, ok := resolution.Workspace.ProjectFromWorkingDirectory(); ok {
		result.Scope = "project"
		result.Project = project.Name
	}
	return result, nil
}
