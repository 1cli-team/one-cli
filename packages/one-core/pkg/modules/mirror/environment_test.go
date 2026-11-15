package mirror

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectionIsOfflineAndResetPreservesOtherSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ONE_HOME", home)
	dir := filepath.Join(home, "config")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	preferences := filepath.Join(dir, "preferences.json")
	if err := os.WriteFile(preferences, []byte(`{"locale":"zh-CN"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Select(t.Context(), EnvGroup, "public", true); err != nil {
		t.Fatal(err)
	}
	if cfg, err := Load(); cfg != nil || err != nil {
		t.Fatalf("dry run wrote config: %+v, %v", cfg, err)
	}
	if _, err := Select(t.Context(), EnvGroup, "public", false); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil || cfg.Env.Preset != "public" {
		t.Fatalf("config = %+v, %v", cfg, err)
	}
	if err := Reset(t.Context(), "", false); err != nil {
		t.Fatal(err)
	}
	if cfg, err := Load(); cfg != nil || err != nil {
		t.Fatalf("reset: %+v, %v", cfg, err)
	}
	if raw, err := os.ReadFile(preferences); err != nil || string(raw) != `{"locale":"zh-CN"}` {
		t.Fatalf("preferences changed: %s, %v", raw, err)
	}
}

func TestEnvironmentPreservesExplicitSources(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ONE_HOME", t.TempDir())
	if _, err := Select(t.Context(), EnvGroup, "public", false); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"mise.local.toml": "[settings.node]\nmirror_url = 'https://node.internal/'\n",
		".npmrc":          "@company:registry=https://company.invalid/\nregistry=https://npm.internal/\n",
		"pyproject.toml":  "[[tool.uv.index]]\nurl = 'https://python.internal/simple'\ndefault = true\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Resolve(root, []string{"GOPROXY=https://go.internal", "PATH=/bin"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"MISE_NODE_MIRROR_URL", "npm_config_registry", "PIP_INDEX_URL", "UV_DEFAULT_INDEX"} {
		if _, ok := lookup(result.Env, key); ok {
			t.Errorf("overrode explicit configuration using %s", key)
		}
	}
	if value, _ := lookup(result.Env, "GOPROXY"); value != "https://go.internal" {
		t.Fatalf("GOPROXY = %s", value)
	}
	if value, _ := lookup(result.Env, "PUB_HOSTED_URL"); value != "https://pub.flutter-io.cn" {
		t.Fatalf("pub = %s", value)
	}
}

func TestGoRewriteRetainsOfficialChecksum(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ONE_HOME", t.TempDir())
	if _, err := Select(t.Context(), EnvGroup, "public", false); err != nil {
		t.Fatal(err)
	}
	result, err := Resolve(t.TempDir(), []string{})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := lookup(result.Env, "MISE_URL_REPLACEMENTS")
	var replacements map[string]string
	if err := json.Unmarshal([]byte(raw), &replacements); err != nil {
		t.Fatal(err)
	}
	if replacements[goArchivePattern] == "" {
		t.Fatalf("rewrites: %s", raw)
	}
	if replacements[flutterPrefix] == "" {
		t.Fatal("Go rewrite suppressed Flutter rewrite")
	}
	if _, ok := lookup(result.Env, "MISE_GO_DOWNLOAD_MIRROR"); ok {
		t.Fatal("Go checksum would incorrectly use the archive mirror")
	}
}

func TestPNPMBackendUsesRegistryUnlessExplicitlyPinned(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ONE_HOME", t.TempDir())
	if _, err := Select(t.Context(), EnvGroup, "public", false); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, file, data string
		env              []string
		want             string
	}{
		{name: "default", want: "npm:pnpm"},
		{name: "environment", env: []string{"MISE_BACKENDS_PNPM=aqua:pnpm/pnpm"}, want: "aqua:pnpm/pnpm"},
		{name: "alias", file: "mise.toml", data: "[tool_alias]\npnpm='aqua:pnpm/pnpm'"},
		{name: "lock", file: "mise.lock", data: "[tools]\npnpm=[{version='12.3.4',backend='aqua:pnpm/pnpm'}]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.file != "" {
				if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			r, err := Resolve(dir, append([]string{}, tc.env...))
			if err != nil {
				t.Fatal(err)
			}
			got, _ := lookup(r.Env, "MISE_BACKENDS_PNPM")
			if got != tc.want {
				t.Fatalf("backend=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestNestedResolutionDropsParentDefaultsAndKeepsUserRules(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ONE_HOME", t.TempDir())
	if _, err := Select(t.Context(), EnvGroup, "public", false); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	parent, err := Resolve(root, []string{`MISE_URL_REPLACEMENTS={"https://example.org/":"https://mirror.internal/"}`})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, ".npmrc"), []byte("registry=https://npm.internal/"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested, err := Resolve(child, parent.Env)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lookup(nested.Env, "npm_config_registry"); ok {
		t.Fatal("parent defaults overrode child registry")
	}
	if err := Reset(t.Context(), "", false); err != nil {
		t.Fatal(err)
	}
	reset, err := Resolve(child, nested.Env)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range managedKeys() {
		if key == "MISE_URL_REPLACEMENTS" {
			continue
		}
		if _, ok := lookup(reset.Env, key); ok {
			t.Errorf("reset retained %s", key)
		}
	}
	rules, _ := lookup(reset.Env, "MISE_URL_REPLACEMENTS")
	if rules != `{"https://example.org/":"https://mirror.internal/"}` {
		t.Fatalf("user rules changed: %s", rules)
	}
}

func TestResetStopsInjectionAndInvalidConfigIsActionable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ONE_HOME", t.TempDir())
	root := t.TempDir()
	if _, err := Select(context.Background(), EnvGroup, "public", false); err != nil {
		t.Fatal(err)
	}
	r, err := Resolve(root, []string{"PATH=/bin"})
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := lookup(r.Env, "npm_config_registry"); value != "https://registry.npmmirror.com/" {
		t.Fatalf("public registry = %q", value)
	}
	if _, err := Select(t.Context(), EnvGroup, "huawei", false); err != nil {
		t.Fatal(err)
	}
	r, err = Resolve(root, r.Env)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"MISE_NODE_MIRROR_URL": "https://mirrors.huaweicloud.com/repository/toolkit/nodejs/",
		"npm_config_registry":  "https://mirrors.huaweicloud.com/repository/npm/",
		"GOPROXY":              "https://mirrors.huaweicloud.com/repository/goproxy/,direct",
		"PIP_INDEX_URL":        "https://mirrors.huaweicloud.com/repository/pypi/simple/",
		"UV_DEFAULT_INDEX":     "https://mirrors.huaweicloud.com/repository/pypi/simple/",
		"PUB_HOSTED_URL":       "https://pub.flutter-io.cn",
	} {
		if value, _ := lookup(r.Env, key); value != want {
			t.Errorf("after switching to huawei: %s = %q, want %q", key, value, want)
		}
	}
	if err := Reset(t.Context(), "", false); err != nil {
		t.Fatal(err)
	}
	r, err = Resolve(root, r.Env)
	if err != nil || len(r.Env) != 1 {
		t.Fatalf("reset injection: %+v %v", r, err)
	}
	path, _ := Path()
	if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(root, nil); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("invalid config error = %v", err)
	}
	if err := Reset(t.Context(), "", false); err != nil {
		t.Fatal(err)
	}
}
