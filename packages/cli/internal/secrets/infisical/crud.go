package infisical

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// SetInput captures `env set` flags. Path is empty when the user wants
// the resolver to derive it from cwd (subproject-aware default).
type SetInput struct {
	Env       string
	Path      string
	Key       string
	Value     string
	Overwrite bool // matches the legacy `--yes` semantics

	// Cfg / Creds let callers (specifically the capability adapter in
	// verbs.go) forward a pre-resolved profile through. When nil, the
	// internal resolveCfgAndCreds resolves the default env profile
	// itself — same auth source either way.
	Cfg   *WorkspaceConfig
	Creds *Credentials
}

// SetResult is the JSON envelope for `env set`. Action is one of
// "created" / "updated" / "unchanged" so agents can short-circuit retry
// loops.
type SetResult struct {
	Schema string `json:"schema"`
	Env    string `json:"env"`
	Path   string `json:"path"`
	Key    string `json:"key"`
	Action string `json:"action"`
}

// RenderTTY prints a one-line set confirmation.
func (r *SetResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	verb := r.Action
	if verb == "" {
		verb = "wrote"
	}
	fmt.Fprintf(w, "✓ %s %s at %s (env=%s)\n", verb, r.Key, r.Path, r.Env)
}

// Set writes a single key into Infisical. Auto-detects whether the key
// already exists and routes to Update vs Create accordingly. Returns
// ENV_SET_OVERWRITE_REQUIRED when an existing key would be modified
// without --yes.
func Set(ctx context.Context, projectRoot string, in SetInput) (*SetResult, error) {
	cfg, creds, err := resolveCfgAndCreds(projectRoot, in.Cfg, in.Creds)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Key) == "" {
		return nil, cliErrors.New(cliErrors.ENV_SET_KEY_REQUIRED,
			"必须提供 <KEY> 位置参数。")
	}
	if err := AssertValidKey(in.Key); err != nil {
		return nil, err
	}
	env, err := SanitizeEnvName(envOrDefault(in.Env, cfg.DefaultEnvOrFallback()))
	if err != nil {
		return nil, err
	}

	client, err := NewClient(ctx, cfg, creds)
	if err != nil {
		return nil, err
	}

	path := NormalizePath(in.Path)

	// Probe existing value first. If absent, Create; if present and
	// identical, return action="unchanged"; if present and different,
	// require --yes to overwrite.
	existing, err := client.RetrieveSecret(env, path, in.Key)
	if err != nil {
		var cErr *output.Error
		if errors.As(err, &cErr) && cErr.Code == string(cliErrors.ENV_KEY_NOT_FOUND) {
			// Infisical's secret-create requires the parent folder to
			// exist; without this, `env set -p some/new/dir KEY V`
			// surfaces a confusing 404. Auto-provision the chain.
			if err := client.EnsureFolder(env, path); err != nil {
				return nil, err
			}
			if _, err := client.CreateSecret(env, path, in.Key, in.Value); err != nil {
				return nil, err
			}
			return &SetResult{
				Schema: "one-cli/env-set/v1",
				Env:    env, Path: path, Key: in.Key, Action: "created",
			}, nil
		}
		return nil, err
	}
	if existing.SecretValue == in.Value {
		return &SetResult{
			Schema: "one-cli/env-set/v1",
			Env:    env, Path: path, Key: in.Key, Action: "unchanged",
		}, nil
	}
	if !in.Overwrite {
		return nil, cliErrors.New(cliErrors.ENV_SET_OVERWRITE_REQUIRED,
			"密钥 "+in.Key+" 已存在且值不同。加 --yes 确认覆盖。").
			WithContext(map[string]any{
				"env": env, "path": path, "key": in.Key,
			})
	}
	if _, err := client.UpdateSecret(env, path, in.Key, in.Value); err != nil {
		return nil, err
	}
	return &SetResult{
		Schema: "one-cli/env-set/v1",
		Env:    env, Path: path, Key: in.Key, Action: "updated",
	}, nil
}

// GetInput captures `env get` flags.
type GetInput struct {
	Env  string
	Path string
	Key  string

	Cfg   *WorkspaceConfig
	Creds *Credentials
}

// GetResult is the JSON envelope. The value is included intentionally — in
// JSON mode this lets agents pipe `one env get FOO -o json | jq -r .value`
// into a subprocess. We do NOT log the value anywhere; only stdout.
type GetResult struct {
	Schema string `json:"schema"`
	Env    string `json:"env"`
	Path   string `json:"path"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

// RenderTTY prints just the value (so users can pipe it sanely).
// Header line goes to context only; the bare value is the last line.
func (r *GetResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintln(w, r.Value)
}

// Get reads a single key. Returns ENV_KEY_NOT_FOUND when absent.
func Get(ctx context.Context, projectRoot string, in GetInput) (*GetResult, error) {
	cfg, creds, err := resolveCfgAndCreds(projectRoot, in.Cfg, in.Creds)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Key) == "" {
		return nil, cliErrors.New(cliErrors.ENV_SET_KEY_REQUIRED,
			"必须提供 <KEY> 位置参数。")
	}
	env, err := SanitizeEnvName(envOrDefault(in.Env, cfg.DefaultEnvOrFallback()))
	if err != nil {
		return nil, err
	}
	client, err := NewClient(ctx, cfg, creds)
	if err != nil {
		return nil, err
	}
	path := NormalizePath(in.Path)
	secret, err := client.RetrieveSecret(env, path, in.Key)
	if err != nil {
		return nil, err
	}
	return &GetResult{
		Schema: "one-cli/env-get/v1",
		Env:    env, Path: path, Key: in.Key, Value: secret.SecretValue,
	}, nil
}

// ListInput captures `env list` flags.
type ListInput struct {
	Env       string
	Path      string
	Recursive bool

	Cfg   *WorkspaceConfig
	Creds *Credentials
}

// ListResult is the JSON envelope. Values are deliberately omitted —
// listing a folder shouldn't dump every secret on stdout. Use `get` for
// individual values.
type ListResult struct {
	Schema string   `json:"schema"`
	Env    string   `json:"env"`
	Path   string   `json:"path"`
	Keys   []string `json:"keys"`
	Total  int      `json:"total"`
}

// RenderTTY prints the keys, one per line (no values).
func (r *ListResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "Path: %s · env: %s · %d key%s\n",
		r.Path, r.Env, r.Total, func() string {
			if r.Total == 1 {
				return ""
			}
			return "s"
		}())
	for _, k := range r.Keys {
		fmt.Fprintf(w, "  %s\n", k)
	}
}

// List returns the keys at a given path/env (without values). Recursive
// mode walks subfolders too.
func List(ctx context.Context, projectRoot string, in ListInput) (*ListResult, error) {
	cfg, creds, err := resolveCfgAndCreds(projectRoot, in.Cfg, in.Creds)
	if err != nil {
		return nil, err
	}
	env, err := SanitizeEnvName(envOrDefault(in.Env, cfg.DefaultEnvOrFallback()))
	if err != nil {
		return nil, err
	}
	client, err := NewClient(ctx, cfg, creds)
	if err != nil {
		return nil, err
	}
	path := NormalizePath(in.Path)
	secrets, err := client.ListSecrets(env, path, in.Recursive)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(secrets))
	for _, s := range secrets {
		keys = append(keys, s.SecretKey)
	}
	sort.Strings(keys)
	return &ListResult{
		Schema: "one-cli/env-list/v1",
		Env:    env, Path: path, Keys: keys, Total: len(keys),
	}, nil
}

func envOrDefault(env, fallback string) string {
	if strings.TrimSpace(env) == "" {
		return fallback
	}
	return env
}
