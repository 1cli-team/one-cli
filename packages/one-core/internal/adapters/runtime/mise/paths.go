package mise

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

type runtimePaths struct {
	data, config, state, cache string
}

// CLI and Desktop use the same One-owned roots. External mise keeps its own environment.
func defaultPaths() (runtimePaths, error) {
	p, err := userdirs.Resolve()
	return runtimePaths{data: p.Data, config: p.Config, state: p.State, cache: p.Cache}, err
}

func (p runtimePaths) runtimeRoot() string { return runtimeRoot(p.data) }

func runtimeRoot(base string) string {
	if base == "" {
		return ""
	}
	return filepath.Join(base, "runtimes", "mise")
}

func (p runtimePaths) owns(path string) bool {
	root := p.runtimeRoot()
	return root != "" && (withinCleanPath(root, path) || withinPath(root, path))
}

func withinPath(root, path string) bool {
	root, path = canonicalPath(root), canonicalPath(path)
	return withinCleanPath(root, path)
}

func withinCleanPath(root, path string) bool {
	if runtime.GOOS == "windows" {
		root, path = strings.ToLower(root), strings.ToLower(path)
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Resolve existing parents too: a missing binary beneath a symlinked data root
// still belongs to One, and macOS /var and /private/var name the same tree.
func canonicalPath(path string) string {
	original := filepath.Clean(path)
	suffix := ""
	for {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			return filepath.Join(resolved, suffix)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return original
		}
		suffix = filepath.Join(filepath.Base(path), suffix)
		path = parent
	}
}

func (p runtimePaths) environment(env []string) []string {
	return replaceEnv(pinnedRuntimeEnv(env),
		"MISE_DATA_DIR="+filepath.Join(p.data, "mise"),
		"MISE_CONFIG_DIR="+filepath.Join(p.config, "mise"),
		"MISE_STATE_DIR="+filepath.Join(p.state, "mise"),
		"MISE_CACHE_DIR="+filepath.Join(p.cache, "mise"),
	)
}

func envKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

func envValue(env []string, key string) string {
	if env == nil {
		env = os.Environ()
	}
	for i := len(env) - 1; i >= 0; i-- {
		k, value, _ := strings.Cut(env[i], "=")
		if envKey(k) == envKey(key) {
			return value
		}
	}
	return ""
}

func replaceEnv(env []string, values ...string) []string {
	if env == nil {
		env = os.Environ()
	}
	keys := make(map[string]bool, len(values))
	for _, value := range values {
		key, _, _ := strings.Cut(value, "=")
		keys[envKey(key)] = true
	}
	result := make([]string, 0, len(env)+len(values))
	for _, value := range env {
		key, _, _ := strings.Cut(value, "=")
		if !keys[envKey(key)] {
			result = append(result, value)
		}
	}
	return append(result, values...)
}
