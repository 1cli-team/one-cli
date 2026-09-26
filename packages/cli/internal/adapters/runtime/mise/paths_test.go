package mise

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestRuntimePathsDefaultsAndXDG(t *testing.T) {
	home := isolatedRuntimeHome(t)
	paths, err := defaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.runtimeRoot() != filepath.Join(home, "XDG_DATA_HOME", "one", "runtimes", "mise") || paths.legacyRoot() != filepath.Join(home, "XDG_CACHE_HOME", "one", "runtimes", "mise") {
		t.Fatalf("XDG paths: %+v", paths)
	}
	for _, key := range []string{"XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(key, "")
	}
	paths, err = defaultPaths()
	want := runtimePaths{data: filepath.Join(home, ".local", "share", "one"), config: filepath.Join(home, ".config", "one"), state: filepath.Join(home, ".local", "state", "one"), cache: filepath.Join(home, ".cache", "one")}
	if err != nil || paths != want {
		t.Fatalf("default paths: %+v %v", paths, err)
	}
	for _, key := range []string{"XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "relative")
			if _, err := defaultPaths(); err == nil {
				t.Fatal("relative root accepted")
			}
		})
	}
}

func TestManagedEnvironmentOwnsDirectoriesWithoutMutatingCaller(t *testing.T) {
	isolatedRuntimeHome(t)
	paths, err := defaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	env := []string{"PATH=/existing", "KEEP=value", "MISE_DATA_DIR=/external", "MISE_DATA_DIR=/duplicate", "MISE_CONFIG_DIR=/external", "MISE_STATE_DIR=/external", "MISE_CACHE_DIR=/external", "MISE_AUTO_UPDATE=true", "MISE_DISABLE_UPDATE_WARNING=false"}
	before := append([]string(nil), env...)
	got := paths.environment(env)
	for key, value := range map[string]string{
		"MISE_DATA_DIR": filepath.Join(paths.data, "mise"), "MISE_CONFIG_DIR": filepath.Join(paths.config, "mise"),
		"MISE_STATE_DIR": filepath.Join(paths.state, "mise"), "MISE_CACHE_DIR": filepath.Join(paths.cache, "mise"),
		"MISE_AUTO_UPDATE": "false", "MISE_DISABLE_UPDATE_WARNING": "true", "KEEP": "value", "PATH": "/existing",
	} {
		if envValue(got, key) != value {
			t.Errorf("%s = %s, want %s", key, envValue(got, key), value)
		}
	}
	if !reflect.DeepEqual(env, before) || len(got) != len(env)-1 {
		t.Fatalf("environment mutation or duplicate: %q -> %q", env, got)
	}
	if withinPath(paths.cache, paths.runtimeRoot()) || withinPath(paths.cache, filepath.Join(paths.data, "mise")) || withinPath(paths.cache, filepath.Join(paths.state, "mise")) {
		t.Fatal("cache cleanup can delete runtime, tools or trust")
	}
}

func TestRuntimeOwnershipThroughSymlinkAndMissingFile(t *testing.T) {
	isolatedRuntimeHome(t)
	paths, err := defaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	root := paths.runtimeRoot()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "linked root")
	if err := os.Symlink(root, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink privilege unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if !paths.owns(filepath.Join(link, "missing", "mise")) || !paths.owns(filepath.Join(paths.legacyRoot(), "missing", "mise")) {
		t.Fatal("managed missing/symlink path treated as external")
	}
	if paths.owns(root + "-external") {
		t.Fatal("path prefix collision treated as owned")
	}
	outward := filepath.Join(root, "outside")
	if err := os.Symlink(t.TempDir(), outward); err != nil {
		t.Fatal(err)
	}
	if !paths.owns(filepath.Join(outward, "mise")) {
		t.Fatal("symlink inside runtime root bypassed ownership")
	}
}
