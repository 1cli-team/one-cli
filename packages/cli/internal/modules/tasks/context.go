package tasks

import (
	"context"
	"maps"
	"runtime"
	"sort"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type environmentSession struct {
	env       []string
	snapshots map[string]map[string]string
	uncached  map[string]bool
}

func (s *environmentSession) close() {}

// Batch-load an immutable invocation snapshot. Values never enter a task plan,
// a temporary config, a plugin or a cache. Even an empty snapshot bypasses freshness.
func (s Service) prepareEnvironment(ctx context.Context, w execution.Workspace, p *Plan, base []string) (*environmentSession, error) {
	session := &environmentSession{env: base, snapshots: map[string]map[string]string{}, uncached: map[string]bool{}}
	projectDirs := map[string]string{}
	dirs := []string{}
	for _, task := range p.Tasks {
		if task.Project == "" {
			continue
		}
		project, _ := w.Project(task.Project)
		if !workspace.EnvironmentEnabled(w.Manifest(), project.RelativeDir) {
			continue
		}
		projectDirs[task.Project] = project.RelativeDir
		if !containsDirectory(dirs, project.RelativeDir) {
			dirs = append(dirs, project.RelativeDir)
		}
	}
	if len(dirs) == 0 {
		return session, nil
	}
	provider := workspace.EnvBackend(w.Manifest())
	if s.Loaders == nil {
		return nil, i18n.Errorf("exec.provider_unregistered", provider)
	}
	loader := s.Loaders.Find(provider)
	if loader == nil {
		return nil, i18n.Errorf("exec.provider_unregistered", provider)
	}
	values, err := secrets.LoadProjects(ctx, loader, w.Root(), dirs, p.Environment)
	if err != nil {
		return nil, err
	}
	for name, dir := range projectDirs {
		snapshot, ok := values[dir]
		if !ok {
			return nil, i18n.Errorf("tasks.environment_missing", name)
		}
		session.snapshots[name] = maps.Clone(snapshot)
	}
	for _, task := range p.Tasks {
		if _, ok := session.snapshots[task.Project]; ok {
			session.uncached[task.Name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, task := range p.Tasks {
			if session.uncached[task.Name] {
				continue
			}
			for _, dependency := range append(append([]string(nil), task.Dependencies...), task.waitFor...) {
				if session.uncached[dependency] {
					session.uncached[task.Name] = true
					changed = true
					break
				}
			}
		}
	}
	return session, nil
}
func containsDirectory(dirs []string, dir string) bool {
	for _, value := range dirs {
		if value == dir {
			return true
		}
	}
	return false
}

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
