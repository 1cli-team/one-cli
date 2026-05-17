package dotenv

// ops.go is the package-local API for the dotenv backend's CRUD
// operations. Callers (envcmd) invoke these directly instead of going
// through pkg/plugin.
//
// Environment model (v0.7+): every dotenv subproject can hold one or
// more environment-scoped files alongside the base .env. The resolver
// loads them in this order, with later layers overriding earlier ones:
//
//	.env                  base — shared across environments
//	.env.<env>            committed per-environment overrides
//	.env.local            gitignored local overrides (no env)
//	.env.<env>.local      gitignored local overrides (per env)
//
// Set writes to <subproject>/.env.<env> (creating the file if absent),
// preserving existing comments / blank lines / key ordering. The
// trailing `.local` overlays are read-only as far as `one env set`
// goes — managing those is purely manual on the developer's machine.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// GetInput addresses one key inside one subproject's dotenv stack.
// SubprojectPath is the relative directory; when empty the cwd is used
// to infer it. Env names which environment overlay is on top of the
// base .env. Empty means base-only.
type GetInput struct {
	ProjectRoot    string
	SubprojectPath string
	Env            string
	Key            string
}

// Stable JSON envelope schema strings for the dotenv operations.
const (
	SchemaGet  = "one-cli/env-get/v1"
	SchemaList = "one-cli/env-list/v1"
	SchemaSet  = "one-cli/env-set/v1"
)

// GetResult is what a successful Get returns. Source is the absolute
// path of the highest-priority file the value came from (i.e. the
// last layer in the overlay chain that defined the key).
type GetResult struct {
	Schema string `json:"schema"`
	Source string `json:"source"`
	Env    string `json:"env,omitempty"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

// ListInput is a Get without a key — enumerate the merged file stack.
type ListInput struct {
	ProjectRoot    string
	SubprojectPath string
	Env            string
}

// ListResult is the sorted set of KEY names across the overlay chain
// (no values). Sources lists the files that actually contributed at
// least one key, in load order.
type ListResult struct {
	Schema  string   `json:"schema"`
	Sources []string `json:"sources"`
	Env     string   `json:"env,omitempty"`
	Keys    []string `json:"keys"`
}

// SetInput writes a single key. Env addresses which committed overlay
// to write into (.env.<env>); empty Env writes to the base .env.
// Overwrite is the explicit-confirmation gate that mirrors the
// infisical backend's --yes semantics.
type SetInput struct {
	ProjectRoot    string
	SubprojectPath string
	Env            string
	Key            string
	Value          string
	Overwrite      bool
}

// SetResult mirrors the infisical SetResult shape so JSON consumers
// can switch backends without a schema fork. Action is one of
// "created" / "updated" / "unchanged".
type SetResult struct {
	Schema string `json:"schema"`
	Source string `json:"source"`
	Env    string `json:"env,omitempty"`
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
	if r.Env != "" {
		fmt.Fprintf(w, "✓ %s %s in %s (env=%s)\n", verb, r.Key, r.Source, r.Env)
	} else {
		fmt.Fprintf(w, "✓ %s %s in %s\n", verb, r.Key, r.Source)
	}
}

// Get reads a single key from the overlay chain. Returns
// ENV_KEY_NOT_FOUND when the key is missing from every layer.
func Get(in GetInput) (*GetResult, error) {
	subDir, err := resolveSubprojectDir(in.ProjectRoot, in.SubprojectPath)
	if err != nil {
		return nil, err
	}
	chain := overlayChain(subDir, in.Env)
	merged, sources, err := loadOverlay(chain)
	if err != nil {
		return nil, err
	}
	val, ok := merged[in.Key]
	if !ok {
		where := strings.Join(sources, " + ")
		if where == "" {
			where = "未找到 .env"
		}
		return nil, cliErrors.New(cliErrors.ENV_KEY_NOT_FOUND,
			fmt.Sprintf("KEY %q 不在 dotenv 文件中（%s）", in.Key, where))
	}
	// Source = the last file in the overlay that defined this key.
	source := sources[len(sources)-1]
	for i := len(chain) - 1; i >= 0; i-- {
		vars, _, _ := LoadDotenvFile(chain[i])
		if _, ok := vars[in.Key]; ok {
			source = chain[i]
			break
		}
	}
	return &GetResult{
		Schema: SchemaGet,
		Source: source,
		Env:    in.Env,
		Key:    in.Key,
		Value:  val,
	}, nil
}

// List returns sorted KEY names from the overlay chain (no values).
func List(in ListInput) (*ListResult, error) {
	subDir, err := resolveSubprojectDir(in.ProjectRoot, in.SubprojectPath)
	if err != nil {
		return nil, err
	}
	chain := overlayChain(subDir, in.Env)
	merged, sources, err := loadOverlay(chain)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return &ListResult{
		Schema:  SchemaList,
		Sources: sources,
		Env:     in.Env,
		Keys:    keys,
	}, nil
}

// Set writes (or updates) a single key in the dotenv file targeted by
// Env. Empty Env writes to <subproject>/.env. Otherwise writes to
// <subproject>/.env.<env>. Creates the file when missing. Returns
// ENV_SET_OVERWRITE_REQUIRED when the key already exists with a
// different value and Overwrite is false.
func Set(in SetInput) (*SetResult, error) {
	if strings.TrimSpace(in.Key) == "" {
		return nil, cliErrors.New(cliErrors.ENV_SET_KEY_REQUIRED,
			"必须提供 <KEY> 位置参数。")
	}
	subDir, err := resolveSubprojectDir(in.ProjectRoot, in.SubprojectPath)
	if err != nil {
		return nil, err
	}
	target := envFileName(subDir, in.Env)

	existing, found, err := LoadDotenvFile(target)
	if err != nil {
		return nil, cliErrors.New(cliErrors.RUN_DOTENV_MISSING,
			fmt.Sprintf("读取 %s 失败：%v", target, err))
	}

	action := "created"
	if found {
		if cur, ok := existing[in.Key]; ok {
			if cur == in.Value {
				return &SetResult{
					Schema: SchemaSet,
					Source: target,
					Env:    in.Env,
					Key:    in.Key,
					Action: "unchanged",
				}, nil
			}
			if !in.Overwrite {
				return nil, cliErrors.New(cliErrors.ENV_SET_OVERWRITE_REQUIRED,
					"密钥 "+in.Key+" 已存在且值不同。加 --yes 确认覆盖。").
					WithContext(map[string]any{
						"source": target,
						"env":    in.Env,
						"key":    in.Key,
					})
			}
			action = "updated"
		}
	}

	if err := writeDotenvKey(target, in.Key, in.Value); err != nil {
		return nil, err
	}
	return &SetResult{
		Schema: SchemaSet,
		Source: target,
		Env:    in.Env,
		Key:    in.Key,
		Action: action,
	}, nil
}

// envFileName returns <subDir>/.env when env is empty, or
// <subDir>/.env.<env> otherwise.
func envFileName(subDir, env string) string {
	env = strings.TrimSpace(env)
	if env == "" {
		return filepath.Join(subDir, ".env")
	}
	return filepath.Join(subDir, ".env."+env)
}

// overlayChain returns the absolute paths in load order for a given
// (subDir, env) pair. The order is base → committed env → local → local env.
func overlayChain(subDir, env string) []string {
	env = strings.TrimSpace(env)
	chain := []string{filepath.Join(subDir, ".env")}
	if env != "" {
		chain = append(chain, filepath.Join(subDir, ".env."+env))
	}
	chain = append(chain, filepath.Join(subDir, ".env.local"))
	if env != "" {
		chain = append(chain, filepath.Join(subDir, ".env."+env+".local"))
	}
	return chain
}

// loadOverlay merges every existing file in the chain. Later entries
// override earlier ones. Missing files are silently skipped (this is
// the overlay shape — base may exist alone, or only an env file may
// exist). I/O errors propagate as RUN_DOTENV_MISSING.
func loadOverlay(chain []string) (map[string]string, []string, error) {
	merged := map[string]string{}
	sources := []string{}
	for _, path := range chain {
		vars, found, err := LoadDotenvFile(path)
		if err != nil {
			return nil, nil, cliErrors.New(cliErrors.RUN_DOTENV_MISSING,
				fmt.Sprintf("读取 %s 失败：%v", path, err))
		}
		if !found {
			continue
		}
		sources = append(sources, path)
		for k, v := range vars {
			merged[k] = v
		}
	}
	return merged, sources, nil
}

// resolveSubprojectDir resolves the absolute directory the caller
// addressed. Selector is pnpm-style: a subproject name (`web`) OR a
// relative path (`apps/web`, `./apps/web`). When empty, walks up from
// cwd. If cwd is not inside any subproject (e.g. user is at workspace
// root), the workspace root itself is used — root-scoped keys live in
// `<projectRoot>/.env*` and are intended as globals for every
// subproject.
func resolveSubprojectDir(projectRoot, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		sub, err := workspace.ResolveProjectFromCWD(projectRoot, cwd)
		if err != nil {
			return "", err
		}
		if sub == nil {
			// Workspace-root scope: write/read at the workspace root.
			return projectRoot, nil
		}
		return sub.TargetDir, nil
	}
	sub, err := workspace.ResolveProjectFromSelector(projectRoot, selector)
	if err != nil {
		return "", err
	}
	if sub != nil {
		return sub.TargetDir, nil
	}
	// Fall through: accept a raw relative dir even if it isn't a
	// declared subproject (covers ad-hoc dirs / pre-add scenarios).
	// If the path doesn't resolve to a real directory, the file write
	// later will fail with a clean error.
	return filepath.Join(projectRoot, filepath.FromSlash(selector)), nil
}

// writeDotenvKey upserts key=value in the target file. Preserves
// existing comments, blank lines, and key ordering by replacing only
// the matching line; appends a new line when the key is absent.
// Creates the file (and parent dirs) when missing.
func writeDotenvKey(path, key, value string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	original := string(raw)
	encoded := key + "=" + serializeValue(value)

	lines := strings.Split(original, "\n")
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		eq := strings.Index(trimmed, "=")
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(trimmed[:eq])
		if k == key {
			lines[i] = encoded
			replaced = true
			break
		}
	}
	out := ""
	if replaced {
		out = strings.Join(lines, "\n")
	} else {
		// Append. Ensure the existing content (if any) ends with a
		// newline so the new line lives on its own.
		out = strings.TrimRight(original, "\n")
		if out != "" {
			out += "\n"
		}
		out += encoded + "\n"
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out), 0o600)
}
