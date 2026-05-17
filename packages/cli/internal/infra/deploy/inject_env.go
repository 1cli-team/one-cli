package deploy

import (
	"context"
	"fmt"
	"sort"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// LoadInjectionOptions controls how LoadInjectionEnv resolves which loader
// to use and which environment to read. Empty values fall back to the
// "auto" path used by `one run`: highest-priority available loader, env
// name resolved from the manifest.environments.default chain.
type LoadInjectionOptions struct {
	// LoaderID forces a specific secrets loader (e.g. "dotenv",
	// "infisical"). Empty means auto-pick by priority.
	LoaderID string

	// EnvName forces a specific manifest env name. Empty means resolve
	// via secrets.ResolveEnvName (--env flag →
	// manifest.environments.default → manifest.environments.names[0]).
	EnvName string
}

// InjectionResult is the output of LoadInjectionEnv. nil result means
// "no injection" (opt-out, no loader available, project-level disabled,
// or dotenv files absent — all benign).
type InjectionResult struct {
	// Vars is the KV map the provider should merge into its child
	// process env. Never empty when InjectionResult is non-nil.
	Vars map[string]string

	// Source is the loader.ID() that produced Vars. Used for dry-run
	// output and structured logging.
	Source string

	// EnvName is the manifest env name actually used (after resolution).
	// Empty for workspaces with no environments declared.
	EnvName string

	// Keys is the sorted list of KEY names in Vars. Pre-sorted so the
	// deploycmd dry-run output and the per-provider ApplyResult both
	// see a stable ordering without re-sorting.
	Keys []string
}

// LoadInjectionEnv reads the project's env vars from the active secrets
// backend and returns them as a map ready for cmd.Env injection. Returns
// (nil, nil) when no injection should happen — callers must be nil-safe.
//
// Behaviour:
//   - manifest.projects[i].env.disabled = true   → (nil, nil)
//   - opts.LoaderID set but unknown              → error
//   - opts.LoaderID = "" and no loader available → (nil, nil)
//   - loader returns zero vars                   → (nil, nil)
//   - any Load() error (e.g. Infisical 5xx)      → bubble up
//
// The dotenv backend itself treats a missing .env as "zero vars"
// rather than an error (see dotenv/loader.go), so deploy automatically
// downgrades — dotenv files are .gitignore'd and CI runners will never
// have one. Infisical errors, by contrast, indicate a real problem and
// bubble up so the user can choose --env-provider dotenv.
func LoadInjectionEnv(ctx context.Context, in ApplyInput, opts LoadInjectionOptions) (*InjectionResult, error) {
	if projectEnvDisabled(in) {
		return nil, nil
	}

	envName, _, err := secrets.ResolveEnvName(in.ProjectRoot, opts.EnvName, true)
	if err != nil {
		return nil, err
	}

	var loader secrets.Loader
	if id := strings.TrimSpace(opts.LoaderID); id != "" {
		loader = secrets.Find(id)
		if loader == nil {
			ids := make([]string, 0, len(secrets.All()))
			for _, l := range secrets.All() {
				ids = append(ids, l.ID())
			}
			return nil, cliErrors.New(cliErrors.BACKEND_NOT_ENABLED,
				fmt.Sprintf("未知 secrets backend %q（已注册：%s）", id, strings.Join(ids, ", "))).
				WithContext(map[string]any{
					"requested": id,
					"available": ids,
				})
		}
	} else {
		loader = secrets.PickAvailable(in.ProjectRoot)
	}
	if loader == nil {
		return nil, nil
	}

	vars, err := loader.Load(ctx, in.ProjectRoot, in.Project.RelativeDir, envName)
	if err != nil {
		return nil, err
	}
	if len(vars) == 0 {
		return nil, nil
	}
	for k := range vars {
		if vErr := secrets.AssertValidKey(k); vErr != nil {
			return nil, vErr
		}
	}
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return &InjectionResult{
		Vars:    vars,
		Source:  loader.ID(),
		EnvName: envName,
		Keys:    keys,
	}, nil
}

// projectEnvDisabled returns true when the manifest entry for in.Project
// has its env override marked disabled. ApplyInput.Manifest can be nil in
// tests, so we guard.
func projectEnvDisabled(in ApplyInput) bool {
	if in.Manifest == nil {
		return false
	}
	env := workspace.ProjectEnv(in.Manifest, in.Project.Name)
	return env != nil && env.Disabled
}
