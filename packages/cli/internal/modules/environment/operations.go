package environment

import (
	"context"
	"errors"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type GetInput struct {
	Scope       execution.Scope
	Environment string
	Project     string
	Key         string
	// Retained for caller compatibility. Reads never initialize storage.
	RepositoryReadOnly bool
}

func (s *Service) Get(ctx context.Context, input GetInput) (*GetResult, error) {
	resolution, err := s.resolve(resolveInput{
		Scope: input.Scope, Requested: input.Environment,
		Capability: catalog.CapabilityEnvGet, Verb: "get",
	})
	if err != nil {
		return nil, err
	}
	root := resolution.Workspace.Root()
	environment := resolution.Scope.Environment()
	if err := requireInfisicalBackend(resolution); err != nil {
		return nil, err
	}
	config, credentials, err := s.resolveInfisical()
	if err != nil {
		return nil, err
	}
	path, err := s.resolveInfisicalFolderPath(resolution.Workspace, config, input.Project)
	if err != nil {
		return nil, err
	}
	result, err := infisical.Get(ctx, root, infisical.GetInput{
		Env: environment, Path: path, Key: input.Key, Cfg: config, Creds: credentials,
	})
	if result == nil || err != nil {
		return nil, err
	}
	return &GetResult{
		Schema: result.Schema, Environment: result.Env, Path: result.Path,
		Key: result.Key, Value: result.Value,
	}, nil
}

type ListInput struct {
	Scope              execution.Scope
	Environment        string
	Project            string
	RepositoryReadOnly bool
}

func (s *Service) List(ctx context.Context, input ListInput) (*ListResult, error) {
	resolution, err := s.resolve(resolveInput{
		Scope: input.Scope, Requested: input.Environment,
		Capability: catalog.CapabilityEnvList, Verb: "list",
	})
	if err != nil {
		return nil, err
	}
	root := resolution.Workspace.Root()
	environment := resolution.Scope.Environment()
	if err := requireInfisicalBackend(resolution); err != nil {
		return nil, err
	}
	config, credentials, err := s.resolveInfisical()
	if err != nil {
		return nil, err
	}
	path, err := s.resolveInfisicalFolderPath(resolution.Workspace, config, input.Project)
	if err != nil {
		return nil, err
	}
	result, err := infisical.List(ctx, root, infisical.ListInput{
		Env: environment, Path: path, Cfg: config, Creds: credentials,
	})
	if result == nil || err != nil {
		return nil, err
	}
	total := result.Total
	return &ListResult{
		Schema: result.Schema, Environment: result.Env, Path: result.Path,
		Keys: result.Keys, Total: &total,
	}, nil
}

type PlanSetInput struct {
	Scope       execution.Scope
	Environment string
	Project     string
}

type SetPlan struct {
	Environment              string
	NeedsBinding             bool
	NeedsEnvironmentCreation bool
	ProjectChoices           []string
	resolution               resolution
	project                  string
}

// PlanSet owns workspace, backend, environment, and project targeting policy.
// The transport only decides whether to confirm the new environment and which
// offered project (or workspace scope) the user selects.
func (s *Service) PlanSet(input PlanSetInput) (SetPlan, error) {
	resolved, err := s.resolve(resolveInput{
		Scope: input.Scope, Requested: input.Environment, AllowUnknown: true,
		Capability: catalog.CapabilityEnvSet, Verb: "set",
	})
	if err != nil {
		return SetPlan{}, err
	}
	if err := validateWriteEnvironment(resolved); err != nil {
		return SetPlan{}, err
	}
	config, err := infisical.LoadWorkspaceConfig(resolved.Workspace.Root())
	if err != nil {
		return SetPlan{}, err
	}
	plan := SetPlan{
		NeedsBinding:             config == nil || strings.TrimSpace(config.ProjectID) == "",
		Environment:              resolved.Scope.Environment(),
		NeedsEnvironmentCreation: resolved.Scope.Environment() != "" && !contains(resolved.Declared, resolved.Scope.Environment()),
		resolution:               resolved,
	}
	selector := strings.TrimSpace(input.Project)
	if selector != "" {
		plan.project = selector
		return plan, nil
	}
	if project, ok := resolved.Workspace.ProjectFromWorkingDirectory(); ok {
		plan.project = project.Name
		return plan, nil
	}
	plan.ProjectChoices = resolved.Workspace.ProjectNames()
	return plan, nil
}

func (p SetPlan) WithProject(project string) SetPlan {
	p.project = strings.TrimSpace(project)
	p.ProjectChoices = nil
	return p
}

type SetInput struct {
	Plan               SetPlan
	Key                string
	Value              string
	Overwrite          bool
	RepositoryReadOnly bool
}

func (s *Service) Set(ctx context.Context, input SetInput) (*SetResult, error) {
	if err := secrets.AssertValidKey(input.Key); err != nil {
		return nil, err
	}
	resolution := input.Plan.resolution
	if resolution.Workspace.Manifest() == nil {
		return nil, cliErrors.New(cliErrors.ONE_CLI_ERROR, i18n.T("env.set_plan_required"))
	}
	root := resolution.Workspace.Root()
	environment := resolution.Scope.Environment()
	if err := validateWriteEnvironment(resolution); err != nil {
		return nil, err
	}
	project, targetSelector := resolveSetTarget(resolution.Workspace, input.Plan.project)
	// Validate selectors before authentication, local writes, or remote creation.
	path, err := s.resolveInfisicalFolderPath(resolution.Workspace, nil, input.Plan.project)
	if err != nil {
		return nil, err
	}
	if input.RepositoryReadOnly {
		if err := requireInfisicalBackend(resolution); err != nil {
			return nil, err
		}
		if environment != "" && !contains(resolution.Declared, environment) {
			return nil, cliErrors.New(cliErrors.ENV_UNKNOWN_ENVIRONMENT,
				i18n.T("env.dashboard_environment_required"))
		}
	}
	config, credentials, err := s.resolveInfisical()
	if err != nil {
		return nil, err
	}
	var binding *BindingResult
	if !input.RepositoryReadOnly {
		binding, err = s.ensureInfisicalBound(ctx, resolution.Workspace)
		if err != nil {
			return nil, err
		}
	}
	createdEnvironment := false
	if !input.RepositoryReadOnly && environment != "" && !contains(resolution.Declared, environment) {
		if _, err := workspace.EnsureEnvironment(root, environment); err != nil {
			return nil, bindingFailure(err, binding)
		}
		createdEnvironment = true
	}
	recordKey := func() error {
		if input.RepositoryReadOnly {
			return nil
		}
		if project != nil {
			return workspace.RecordProjectEnvKey(root, project.Name, input.Key)
		}
		if targetSelector == "" {
			return workspace.RecordWorkspaceEnvKey(root, input.Key)
		}
		return nil
	}

	result, err := infisical.Set(ctx, root, infisical.SetInput{
		Env: environment, Path: path, Key: input.Key, Value: input.Value,
		Overwrite: input.Overwrite, Cfg: config, Creds: credentials,
	})
	if result == nil || err != nil {
		return nil, bindingFailure(err, binding)
	}
	if err := recordKey(); err != nil {
		return nil, bindingFailure(err, binding)
	}
	return &SetResult{
		Schema: result.Schema, Environment: result.Env, Path: result.Path, Binding: binding,
		Key: result.Key, Action: result.Action, CreatedEnvironment: createdEnvironment,
	}, nil
}

type DeleteInput struct {
	Scope              execution.Scope
	Environment        string
	Project            string
	Key                string
	RepositoryReadOnly bool
}

func (s *Service) Delete(ctx context.Context, input DeleteInput) (*infisical.DeleteResult, error) {
	resolution, err := s.resolve(resolveInput{
		Scope: input.Scope, Requested: input.Environment,
		Capability: catalog.CapabilityEnvDelete, Verb: "delete",
	})
	if err != nil {
		return nil, err
	}
	if resolution.Scope.Backend().Name != workspace.EnvBackendInfisical {
		return nil, unsupportedVerb(resolution.Scope.Backend().Name, "delete")
	}
	if err := requireInfisicalBackend(resolution); err != nil {
		return nil, err
	}
	config, credentials, err := s.resolveInfisical()
	if err != nil {
		return nil, err
	}
	path, err := s.resolveInfisicalFolderPath(resolution.Workspace, config, input.Project)
	if err != nil {
		return nil, err
	}
	return infisical.Delete(ctx, resolution.Workspace.Root(), infisical.DeleteInput{
		Env: resolution.Scope.Environment(), Path: path, Key: input.Key,
		Cfg: config, Creds: credentials,
	})
}

func resolveSetTarget(activeWorkspace execution.Workspace, selector string) (*workspace.Project, string) {
	selector = strings.TrimSpace(selector)
	if selector != "" {
		if project, ok := activeWorkspace.Project(selector); ok {
			return project, project.RelativeDir
		}
		return nil, selector
	}
	if project, ok := activeWorkspace.ProjectFromWorkingDirectory(); ok {
		return project, project.RelativeDir
	}
	return nil, ""
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validateWriteEnvironment(resolution resolution) error {
	if environment := resolution.Scope.Environment(); environment != "" {
		_, err := infisical.SanitizeEnvName(environment)
		return err
	}
	return nil
}

// Preserve the specific failure and recovery while reporting a completed binding.
func bindingFailure(err error, binding *BindingResult) error {
	if err == nil || binding == nil || !binding.Created {
		return err
	}
	var original *output.Error
	if !errors.As(err, &original) {
		original = cliErrors.New(cliErrors.ONE_CLI_ERROR, err.Error())
	}
	result := *original
	result.Message = i18n.Tf("env.bound_before_failure", binding.ProjectName, binding.ProjectID, err)
	ctx := make(map[string]any, len(original.Context)+3)
	for key, value := range original.Context {
		ctx[key] = value
	}
	ctx["project_id"], ctx["project_name"], ctx["partial_state"] = binding.ProjectID, binding.ProjectName, "project_bound"
	return result.WithContext(ctx).WithCause(err)
}
