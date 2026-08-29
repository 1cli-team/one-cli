package environment

import (
	"context"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/dotenv"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type GetInput struct {
	Scope       execution.Scope
	Environment string
	Project     string
	Profile     string
	Key         string
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
	switch resolution.Scope.Backend().Name {
	case workspace.EnvBackendDotenv:
		result, err := dotenv.Get(dotenv.GetInput{
			ProjectRoot: root, SubprojectPath: input.Project,
			Env: environment, Key: input.Key,
		})
		if result == nil || err != nil {
			return nil, err
		}
		return &GetResult{
			Schema: result.Schema, Source: result.Source, Environment: result.Env,
			Key: result.Key, Value: result.Value,
		}, nil
	case workspace.EnvBackendInfisical:
		if err := s.ensureInfisicalBound(ctx, root); err != nil {
			return nil, err
		}
		config, credentials, err := s.resolveInfisical(resolution.Workspace, input.Profile)
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
	return nil, unsupportedVerb(resolution.Scope.Backend().Name, "get")
}

type ListInput struct {
	Scope       execution.Scope
	Environment string
	Project     string
	Profile     string
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
	switch resolution.Scope.Backend().Name {
	case workspace.EnvBackendDotenv:
		result, err := dotenv.List(dotenv.ListInput{
			ProjectRoot: root, SubprojectPath: input.Project, Env: environment,
		})
		if result == nil || err != nil {
			return nil, err
		}
		return &ListResult{
			Schema: result.Schema, Sources: result.Sources,
			Environment: result.Env, Keys: result.Keys,
		}, nil
	case workspace.EnvBackendInfisical:
		if err := s.ensureInfisicalBound(ctx, root); err != nil {
			return nil, err
		}
		config, credentials, err := s.resolveInfisical(resolution.Workspace, input.Profile)
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
	return nil, unsupportedVerb(resolution.Scope.Backend().Name, "list")
}

type PlanSetInput struct {
	Scope       execution.Scope
	Environment string
	Project     string
}

type SetPlan struct {
	Environment              string
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
	plan := SetPlan{
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
	Plan      SetPlan
	Profile   string
	Key       string
	Value     string
	Overwrite bool
}

func (s *Service) Set(ctx context.Context, input SetInput) (*SetResult, error) {
	if err := secrets.AssertValidKey(input.Key); err != nil {
		return nil, err
	}
	resolution := input.Plan.resolution
	if resolution.Workspace.Manifest() == nil {
		return nil, cliErrors.New(cliErrors.ONE_CLI_ERROR, "environment set plan is required")
	}
	root := resolution.Workspace.Root()
	environment := resolution.Scope.Environment()
	createdEnvironment := false
	if environment != "" && !contains(resolution.Declared, environment) {
		_, err := workspace.EnsureEnvironment(root, environment)
		if err != nil {
			return nil, err
		}
		// The resolution records what was declared at command start. Preserve
		// created_environment across an overwrite-confirmation retry even though
		// the first attempt has already persisted the new name.
		createdEnvironment = true
	}
	project, targetSelector := resolveSetTarget(resolution.Workspace, input.Plan.project)
	recordKey := func() error {
		if project != nil {
			return workspace.RecordProjectEnvKey(root, project.Name, input.Key)
		}
		if targetSelector == "" {
			return workspace.RecordWorkspaceEnvKey(root, input.Key)
		}
		return nil
	}
	switch resolution.Scope.Backend().Name {
	case workspace.EnvBackendDotenv:
		result, err := dotenv.Set(dotenv.SetInput{
			ProjectRoot: root, SubprojectPath: targetSelector,
			Env: environment, Key: input.Key, Value: input.Value, Overwrite: input.Overwrite,
		})
		if result == nil || err != nil {
			return nil, err
		}
		if err := recordKey(); err != nil {
			return nil, err
		}
		return &SetResult{
			Schema: result.Schema, Source: result.Source, Environment: result.Env,
			Key: result.Key, Action: result.Action, CreatedEnvironment: createdEnvironment,
		}, nil
	case workspace.EnvBackendInfisical:
		if err := s.ensureInfisicalBound(ctx, root); err != nil {
			return nil, err
		}
		config, credentials, err := s.resolveInfisical(resolution.Workspace, input.Profile)
		if err != nil {
			return nil, err
		}
		path, err := s.resolveInfisicalFolderPath(resolution.Workspace, config, targetSelector)
		if err != nil {
			return nil, err
		}
		result, err := infisical.Set(ctx, root, infisical.SetInput{
			Env: environment, Path: path, Key: input.Key, Value: input.Value,
			Overwrite: input.Overwrite, Cfg: config, Creds: credentials,
		})
		if result == nil || err != nil {
			return nil, err
		}
		if err := recordKey(); err != nil {
			return nil, err
		}
		return &SetResult{
			Schema: result.Schema, Environment: result.Env, Path: result.Path,
			Key: result.Key, Action: result.Action, CreatedEnvironment: createdEnvironment,
		}, nil
	}
	return nil, unsupportedVerb(resolution.Scope.Backend().Name, "set")
}

type PullInput struct {
	Scope       execution.Scope
	Environment string
	Project     string
	Profile     string
	Force       bool
	DryRun      bool
}

func (s *Service) Pull(ctx context.Context, input PullInput) (*PullResult, error) {
	resolution, err := s.resolve(resolveInput{
		Scope: input.Scope, Requested: input.Environment,
		Capability: catalog.CapabilityEnvPull, Verb: "pull",
	})
	if err != nil {
		return nil, err
	}
	root := resolution.Workspace.Root()
	if err := s.ensureInfisicalBound(ctx, root); err != nil {
		return nil, err
	}
	config, credentials, err := s.resolveInfisical(resolution.Workspace, input.Profile)
	if err != nil {
		return nil, err
	}
	result, err := infisical.Pull(ctx, root, infisical.PullInput{
		Env: resolution.Scope.Environment(), Project: input.Project,
		Force: input.Force, DryRun: input.DryRun, Cfg: config, Creds: credentials,
	})
	if result == nil || err != nil {
		return nil, err
	}
	entries := make([]PullEntry, 0, len(result.PerSubproject))
	for _, entry := range result.PerSubproject {
		entries = append(entries, PullEntry{
			Name: entry.Name, RelativeDir: entry.RelativeDir,
			InfisicalPath: entry.InfisicalPath, EnvFilePath: entry.EnvFilePath,
			Status: entry.Status, Reason: entry.Reason, KeysWritten: entry.KeysWritten,
		})
	}
	return &PullResult{
		Schema: result.Schema, Environment: result.Env, DryRun: result.DryRun,
		WrittenCount: result.WrittenCount, SkippedCount: result.SkippedCount,
		PerSubproject: entries,
	}, nil
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
