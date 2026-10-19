// Package environment owns the complete environment-variable feature slice.
// It composes the Infisical adapter behind one workflow
// boundary so transports do not reproduce backend selection or workspace
// policy.
package environment

import (
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	catalog "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/backend"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/secrets"
)

type Service struct {
	catalog *catalog.Catalog
	store   *store.Store
}

func NewService(
	backendCatalog *catalog.Catalog, db *store.Store,
) (*Service, error) {
	if backendCatalog == nil {
		return nil, i18n.Errorf("env.catalog_required")
	}

	return &Service{
		catalog: backendCatalog,
		store:   db,
	}, nil
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
	activeWorkspace, err := s.resolveWorkspace(input.Scope)
	if err != nil {
		return resolution{}, err
	}
	manifest := activeWorkspace.Manifest()
	backendName := workspace.EnvBackend(manifest)
	if backendName == "" {
		backendName = workspace.EnvBackendInfisical
	}
	backend, ok := s.catalog.Lookup(catalog.DomainEnv, backendName)
	if !ok {
		return resolution{}, cliErrors.New(
			cliErrors.ENV_BACKEND_INVALID,
			i18n.ForContext(input.Scope.Context()).Tf("env.backend_unknown", backendName),
		)
	}
	if input.Capability != "" && !backend.Has(input.Capability) {
		verb := strings.TrimSpace(input.Verb)
		return resolution{}, cliErrors.New(
			cliErrors.BACKEND_VERB_NOT_SUPPORTED,
			i18n.ForContext(input.Scope.Context()).Tf("env.operation_unsupported", backend.Pair, verb),
		).WithContext(map[string]any{
			"domain": "env", "backend": backendName, "verb": verb,
		})
	}
	environment, declared, err := secrets.ResolveEnvNameContext(input.Scope.Context(), manifest, input.Requested, input.AllowUnknown)
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
	if scope.Project() != "" {
		result.Scope = "project"
		result.Project = scope.Project()
	} else if project, ok := resolution.Workspace.ProjectFromWorkingDirectory(); ok {
		result.Scope = "project"
		result.Project = project.Name
	}
	return result, nil
}

func (s *Service) resolveWorkspace(scope execution.Scope) (execution.Workspace, error) {
	id := scope.WorkspaceRoot()
	if id == "" {
		id = scope.WorkingDirectory()
	}
	manifest, _, err := s.store.Load(scope.Context(), id)
	if err != nil {
		return execution.Workspace{}, err
	}
	return execution.NewWorkspace(scope, id, manifest), nil
}
