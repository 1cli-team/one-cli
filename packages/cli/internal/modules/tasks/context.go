package tasks

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

const environmentPlugin = `local http = require("http")
local json = require("json")
function PLUGIN:MiseEnv(ctx)
    local endpoint = os.getenv("ONE_ENV_ENDPOINT")
    local token = os.getenv("ONE_ENV_TOKEN")
    if not endpoint or not token then error(ctx.options.unavailable) end
    local response = http.get({url=endpoint .. "/" .. ctx.options.id, headers={Authorization="Bearer " .. token}})
    if response.status_code ~= 200 then error(ctx.options.rejected) end
    local entries = {}
    for key, value in pairs(json.decode(response.body)) do
        table.insert(entries, {key=key, value=value})
    end
    return {env=entries, cacheable=false, redact=false}
end
`

// An invocation owns its immutable environment snapshot. Only this loopback
// broker contains values: generated TOML contains task identifiers alone.
type environmentSession struct {
	env      []string
	base     []string
	dir      string
	dirs     []string
	server   *http.Server
	plugin   string
	bindings map[string]string
	uncached map[string]bool
}

func (s *environmentSession) close() {
	if s.server != nil {
		_ = s.server.Close()
	}
	for _, dir := range s.dirs {
		_ = os.RemoveAll(dir)
	}
}

func (s Service) prepareEnvironment(ctx context.Context, w execution.Workspace, p *Plan, base []string) (_ *environmentSession, err error) {
	session := &environmentSession{env: base, base: base, bindings: map[string]string{}, uncached: map[string]bool{}}
	defer func() {
		if err != nil {
			session.close()
		}
	}()
	projects := map[string]map[string]string{}
	provider := workspace.EnvBackend(w.Manifest())
	for _, task := range p.Tasks {
		if task.Project == "" {
			continue
		}
		project, _ := w.Project(task.Project)
		if !workspace.EnvironmentEnabled(w.Manifest(), project.RelativeDir) {
			continue
		}
		if _, ok := projects[task.Project]; ok {
			continue
		}
		if s.Loaders == nil || s.Loaders.Find(provider) == nil {
			return nil, i18n.Errorf("exec.provider_unregistered", provider)
		}
		values, e := s.Loaders.Find(provider).Load(ctx, w.Root(), project.RelativeDir, p.Environment)
		if e != nil {
			return nil, e
		}
		// An empty snapshot still requires a fresh execution: removing the last
		// variable must not restore outputs from the preceding environment.
		projects[task.Project] = maps.Clone(values)
	}
	if len(projects) == 0 {
		return session, nil
	}
	session.dir, err = os.MkdirTemp(w.Root(), ".one-run-")
	if err != nil {
		return nil, err
	}
	session.dirs = append(session.dirs, session.dir)
	session.plugin, err = installEnvironmentPlugin(base)
	if err != nil {
		return nil, err
	}
	token := make([]byte, 32)
	if _, err = rand.Read(token); err != nil {
		return nil, err
	}
	authorization := "Bearer " + hex.EncodeToString(token)
	valuesByID := map[string]map[string]string{}
	overlays := map[string]map[string]any{}
	for _, task := range p.Tasks {
		if _, ok := projects[task.Project]; ok {
			session.uncached[task.Name] = true
		}
	}
	// Downstream artifacts also depend on the remote inputs of their prerequisites.
	for changed := true; changed; {
		changed = false
		for _, task := range p.Tasks {
			if session.uncached[task.Name] {
				continue
			}
			for _, dep := range task.Dependencies {
				if !strings.HasPrefix(dep, "//") {
					scope, _, _ := strings.Cut(task.Name, ":")
					dep = scope + ":" + dep
				}
				affected := session.uncached[dep]
				for name := range session.uncached {
					if matches, _ := filepath.Match(dep, name); matches {
						affected = true
						break
					}
				}
				if affected {
					session.uncached[task.Name] = true
					changed = true
					break
				}
			}
		}
	}
	for i, task := range p.Tasks {
		if !session.uncached[task.Name] || !hasRun(task.Run) && task.File == "" {
			continue
		}
		values, bound := projects[task.Project]
		definition := task.runtimeName()
		if !strings.HasPrefix(definition, "//") {
			definition = "//:" + definition
		}
		scope, local, found := strings.Cut(definition, ":")
		if !found {
			scope = "//"
			local = task.runtimeName()
		}
		directory := filepath.Join(w.Root(), filepath.FromSlash(strings.TrimPrefix(scope, "//")))
		rel, e := filepath.Rel(w.Root(), directory)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, i18n.Errorf("tasks.binding_conflict", task.Name)
		}
		if overlays[directory] == nil {
			overlays[directory] = map[string]any{}
		}
		id := fmt.Sprintf("task-%d", i)
		if bound {
			valuesByID[id] = values
			session.bindings[task.Name] = id
		}
		metadata := map[string]any{
			"cache": map[string]any{"enabled": false},
			// An output that is never created prevents mise's independent mtime skip.
			"outputs": []string{filepath.ToSlash(filepath.Join(session.dir, id+"-uncached"))},
		}
		if bound {
			metadata["env"] = map[string]any{"_": map[string]any{session.plugin: map[string]string{"id": id, "unavailable": i18n.T("tasks.environment_unavailable"), "rejected": i18n.T("tasks.environment_rejected")}}}
		}
		overlays[directory][local] = metadata

	}
	for scope, tasks := range overlays {
		dir := session.dir
		if scope != w.Root() {
			dir = filepath.Join(scope, filepath.Base(session.dir))
			if err = os.Mkdir(dir, 0700); err != nil {
				return nil, err
			}
			session.dirs = append(session.dirs, dir)
		}
		raw, e := toml.Marshal(map[string]any{"tasks": tasks})
		if e != nil {
			return nil, e
		}
		if err = os.WriteFile(filepath.Join(dir, "bindings.toml"), raw, 0600); err != nil {
			return nil, err
		}
	}
	// A unique relative filename resolves independently in each native config
	// scope. Concurrent runs cannot see one another's transient metadata.
	patterns := append(configPatterns(base), filepath.ToSlash(filepath.Join(filepath.Base(session.dir), "binding?.toml")))
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	session.server = &http.Server{ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(out http.ResponseWriter, in *http.Request) {
		if in.Method != http.MethodGet || subtle.ConstantTimeCompare([]byte(in.Header.Get("Authorization")), []byte(authorization)) != 1 {
			http.Error(out, "unauthorized", http.StatusUnauthorized)
			return
		}
		values, ok := valuesByID[strings.TrimPrefix(in.URL.Path, "/")]
		if !ok {
			http.NotFound(out, in)
			return
		}
		out.Header().Set("Content-Type", "application/json")
		out.Header().Set("Cache-Control", "no-store")
		if values == nil {
			values = map[string]string{}
		}
		_ = json.NewEncoder(out).Encode(values)
	})}
	go func() { _ = session.server.Serve(listener) }()
	session.env = secrets.MergeIntoEnviron(base, map[string]string{
		"ONE_ENV_ENDPOINT": "http://" + listener.Addr().String(), "ONE_ENV_TOKEN": hex.EncodeToString(token),
		"MISE_ENV_CACHE": "0", "MISE_OVERRIDE_CONFIG_FILENAMES": strings.Join(patterns, ":"),
		"MISE_TRUSTED_CONFIG_PATHS": strings.Join(append(filepath.SplitList(envValue(base, "MISE_TRUSTED_CONFIG_PATHS")), session.dirs...), string(os.PathListSeparator)),
	}, true)
	return session, nil
}

// Preserve normal config precedence, then add an invocation-only metadata layer.
// Active profiles still take precedence in mise. verifyBindings rejects a lost
// binding rather than executing a task without its requested environment.
func configPatterns(env []string) []string {
	if value := envValue(env, "MISE_OVERRIDE_CONFIG_FILENAMES"); value != "" {
		return strings.Split(value, ":")
	}
	out := []string{".config/mise/conf.d/*.toml", ".config/mise/config.toml", ".config/mise/mise.toml", ".config/mise.toml", ".mise/conf.d/*.toml", ".mise/config.toml", "mise/conf.d/*.toml", "mise/config.toml", "mise.toml"}
	if custom := envValue(env, "MISE_DEFAULT_CONFIG_FILENAME"); custom != "" {
		out = append(out, custom)
	}
	return append(out, ".mise.toml", ".config/mise/config.local.toml", ".config/mise/mise.local.toml", ".config/mise.local.toml", ".mise/config.local.toml", "mise/config.local.toml", "mise.local.toml", ".mise.local.toml")
}

func envValue(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		k, v, _ := strings.Cut(env[i], "=")
		if k == key || runtime.GOOS == "windows" && strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// Install a content-addressed bundled plugin alongside mise's other plugins.
// Atomic publication supports simultaneous One invocations and never replaces
// an existing user plugin. No invocation metadata or values live here.
func installEnvironmentPlugin(env []string) (string, error) {
	sum := sha256.Sum256([]byte(environmentPlugin))
	name := "one-env-" + hex.EncodeToString(sum[:6])
	root := envValue(env, "MISE_PLUGINS_DIR")
	if root == "" {
		data := envValue(env, "MISE_DATA_DIR")
		if data == "" {
			data = envValue(env, "XDG_DATA_HOME")
			if data == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return "", err
				}
				data = filepath.Join(home, ".local", "share")
				if runtime.GOOS == "windows" {
					data = envValue(env, "LOCALAPPDATA")
					if data == "" {
						data = filepath.Join(home, "AppData", "Local")
					}
				}
			}
			data = filepath.Join(data, "mise")
		}
		root = filepath.Join(data, "plugins")
	}
	target := filepath.Join(root, name)
	metadata := []byte("PLUGIN = { name = \"" + name + "\", version = \"1.0.0\", description = \"One task environment\" }\n")
	verify := func() bool {
		a, e := os.ReadFile(filepath.Join(target, "metadata.lua"))
		b, f := os.ReadFile(filepath.Join(target, "hooks", "mise_env.lua"))
		return e == nil && f == nil && bytes.Equal(a, metadata) && string(b) == environmentPlugin
	}
	if verify() {
		return name, nil
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	temp, err := os.MkdirTemp(root, ".one-env-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	if err = os.Mkdir(filepath.Join(temp, "hooks"), 0700); err != nil {
		return "", err
	}
	for path, raw := range map[string][]byte{"metadata.lua": metadata, "hooks/mise_env.lua": []byte(environmentPlugin)} {
		if err = os.WriteFile(filepath.Join(temp, filepath.FromSlash(path)), raw, 0600); err != nil {
			return "", err
		}
	}
	if err = os.Rename(temp, target); err != nil && !verify() {
		return "", err
	}
	return name, nil
}

func projectNames(p *Plan) []string {
	set := map[string]bool{}
	for _, task := range p.Tasks {
		if task.Project != "" {
			set[task.Project] = true
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
