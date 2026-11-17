// Package environment owns the complete environment-variable feature slice.
// Registered backends own vendor operations; this package owns workspace policy.
package environment

import (
	"fmt"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	catalog "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/backend"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/secrets"
)

type Service struct {
	catalog  *catalog.Catalog
	backends map[string]Backend
}

func NewService(
	backendCatalog *catalog.Catalog,
	backends ...Backend,
) (*Service, error) {
	if backendCatalog == nil {
		return nil, i18n.Errorf("env.catalog_required")
	}

	registered := make(map[string]Backend, len(backends))
	for _, provider := range backends {
		if provider == nil || provider.ID() == "" {
			return nil, fmt.Errorf("environment: missing backend identity")
		}
		if _, exists := registered[provider.ID()]; exists {
			return nil, fmt.Errorf("environment: duplicate backend %q", provider.ID())
		}
		if _, exists := backendCatalog.Lookup(catalog.DomainEnv, provider.ID()); !exists {
			return nil, fmt.Errorf("environment: backend %q has no descriptor", provider.ID())
		}
		registered[provider.ID()] = provider
	}
	return &Service{catalog: backendCatalog, backends: registered}, nil
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
		for _, candidate := range s.catalog.All() {
			if candidate.ID.Domain == catalog.DomainEnv {
				backendName = candidate.ID.Name
				break
			}
		}
	}
	backend, ok := s.catalog.Lookup(catalog.DomainEnv, backendName)
	if !ok {
		return resolution{}, cliErrors.New(
			cliErrors.ENV_BACKEND_INVALID,
			i18n.Tf("env.backend_unknown", backendName),
		)
	}
	if input.Capability != "" && !backend.Has(input.Capability) {
		verb := strings.TrimSpace(input.Verb)
		return resolution{}, cliErrors.New(
			cliErrors.BACKEND_VERB_NOT_SUPPORTED,
			i18n.Tf("env.operation_unsupported", backend.Pair, verb),
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
	defaultEnvironment := "dev"
	environments := workspace.EnvironmentNames(manifest)
	result := &Summary{
		Schema:                "one-cli/env-summary/v1",
		Source:                source,
		DefaultEnvironment:    defaultEnvironment,
		AvailableEnvironments: environments,
		Scope:                 "workspace",
		Commands:              []string{"one env bind", "one env set <KEY>", "one env list", "one exec -- <command>"},
	}
	if project, ok := resolution.Workspace.ProjectFromWorkingDirectory(); ok {
		result.Scope = "project"
		result.Project = project.Name
	}
	return result, nil
}

func (s *Service) backend(resolved resolution) (Backend, error) {
	id := resolved.Scope.Backend().Name
	provider := s.backends[id]
	if provider == nil {
		return nil, cliErrors.New(cliErrors.ENV_BACKEND_INVALID, i18n.Tf("env.backend_unknown", id))
	}
	return provider, nil
}
