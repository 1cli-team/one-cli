package environment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/dotenv"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

type SwitchPlan struct {
	Workspace    execution.Workspace
	From         string
	To           string
	ManifestPath string
	tuples       []dotenvTuple
}

func (p SwitchPlan) Entries() int { return len(p.tuples) }

func (s *Service) PlanSwitch(scope execution.Scope, target string) (SwitchPlan, error) {
	target = strings.TrimSpace(target)
	if target != workspace.EnvBackendDotenv && target != workspace.EnvBackendInfisical {
		return SwitchPlan{}, cliErrors.New(cliErrors.ENV_BACKEND_INVALID,
			fmt.Sprintf("不支持的 backend %q；合法值: dotenv / infisical", target))
	}
	resolution, err := s.resolve(resolveInput{Scope: scope, AllowUnknown: true})
	if err != nil {
		return SwitchPlan{}, err
	}
	current := workspace.EnvBackend(resolution.Workspace.Manifest())
	if current == "" {
		current = workspace.EnvBackendDotenv
	}
	if current == target {
		return SwitchPlan{}, cliErrors.New(cliErrors.ENV_BACKEND_UNCHANGED,
			fmt.Sprintf("工作区已经是 %s 后端，无需切换。", target)).
			WithContext(map[string]any{"backend": target})
	}
	plan := SwitchPlan{
		Workspace: resolution.Workspace, From: current, To: target,
		ManifestPath: filepath.Join(resolution.Workspace.Root(), workspace.ManifestFilename),
	}
	if target == workspace.EnvBackendDotenv {
		return plan, nil
	}
	// Resolve credentials during Switch, not while planning: every dotenv
	// tuple can select a different machine-local Profile by project and
	// environment, and PlanSwitch does not yet know whether sync will run.
	plan.tuples, err = collectDotenvTuples(
		resolution.Workspace.Root(), resolution.Workspace.Manifest(),
	)
	if err != nil {
		return SwitchPlan{}, err
	}
	return plan, nil
}

type SwitchOptions struct {
	Sync      bool
	Overwrite bool
	DryRun    bool
}

func (s *Service) Switch(
	ctx context.Context,
	plan SwitchPlan,
	options SwitchOptions,
) (*SwitchResult, error) {
	result := &SwitchResult{
		Schema: "one-cli/env-switch/v1", From: plan.From, To: plan.To,
		ManifestPath: plan.ManifestPath,
	}
	if plan.To == workspace.EnvBackendDotenv {
		result.SkippedSync = true
		if options.DryRun {
			return result, nil
		}
		if err := writeEnvBackend(plan.Workspace.Manifest(), plan.Workspace.Root(), plan.To); err != nil {
			return nil, err
		}
		return result, nil
	}
	if options.DryRun {
		result.Synced = len(plan.tuples)
		result.SkippedSync = !options.Sync
		return result, nil
	}
	if options.Sync && len(plan.tuples) > 0 {
		first := plan.tuples[0]
		if err := s.ensureInfisicalBound(
			ctx, plan.Workspace, "", first.environment, first.project,
		); err != nil {
			return nil, err
		}
		for _, tuple := range plan.tuples {
			config, credentials, err := s.resolveInfisical(
				plan.Workspace, "", tuple.environment, tuple.project,
			)
			if err != nil {
				return nil, err
			}
			_, err = s.setInfisical(ctx, plan.Workspace.Root(), infisical.SetInput{
				Env: tuple.environment, Path: tuple.path, Key: tuple.key, Value: tuple.value,
				Overwrite: options.Overwrite, Cfg: config, Creds: credentials,
			})
			if err != nil {
				var outputError *output.Error
				if errors.As(err, &outputError) &&
					outputError.Code == string(cliErrors.ENV_SET_OVERWRITE_REQUIRED) {
					result.Conflicts++
					continue
				}
				return nil, err
			}
			result.Synced++
		}
		if result.Conflicts > 0 {
			return nil, cliErrors.New(cliErrors.ENV_MIGRATE_CONFLICT,
				fmt.Sprintf("%d 个 key 在 Infisical 已存在且值不同；加 --overwrite 重跑以覆盖。", result.Conflicts)).
				WithContext(map[string]any{
					"backend": plan.To, "conflicts": result.Conflicts, "synced": result.Synced,
				})
		}
	}
	result.SkippedSync = !options.Sync
	if err := writeEnvBackend(plan.Workspace.Manifest(), plan.Workspace.Root(), plan.To); err != nil {
		return nil, err
	}
	return result, nil
}

func writeEnvBackend(manifest *workspace.Manifest, root, target string) error {
	if manifest.Domains == nil {
		manifest.Domains = &workspace.WorkspaceDomains{}
	}
	if manifest.Domains.Env == nil {
		manifest.Domains.Env = &workspace.BackendRef{}
	}
	manifest.Domains.Env.Kind = target
	return workspace.WriteManifest(root, manifest)
}

type dotenvTuple struct {
	project     string
	environment string
	path        string
	key         string
	value       string
}

func collectDotenvTuples(root string, manifest *workspace.Manifest) ([]dotenvTuple, error) {
	var environments []string
	if manifest.Environments != nil {
		environments = append(environments, manifest.Environments.Names...)
	}
	if len(environments) == 0 {
		environments = []string{"dev"}
	}
	var tuples []dotenvTuple
	for _, project := range manifest.Projects {
		relativeDir := strings.TrimSpace(project.RelativeDir)
		projectDir := filepath.Join(root, relativeDir)
		if _, err := os.Stat(projectDir); err != nil {
			continue
		}
		path := "/" + strings.Trim(relativeDir, "/")
		if relativeDir == "" {
			path = "/"
		}
		for _, environment := range environments {
			merged := map[string]string{}
			for _, file := range dotenvOverlayChain(projectDir, environment) {
				content, err := os.ReadFile(file)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return nil, fmt.Errorf("read %s: %w", file, err)
				}
				for key, value := range dotenv.Parse(string(content)) {
					merged[key] = value
				}
			}
			keys := make([]string, 0, len(merged))
			for key := range merged {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				tuples = append(tuples, dotenvTuple{
					project: project.Name, environment: environment, path: path,
					key: key, value: merged[key],
				})
			}
		}
	}
	return tuples, nil
}

func dotenvOverlayChain(projectDir, environment string) []string {
	return []string{
		filepath.Join(projectDir, ".env"),
		filepath.Join(projectDir, ".env."+environment),
		filepath.Join(projectDir, ".env.local"),
		filepath.Join(projectDir, ".env."+environment+".local"),
	}
}
