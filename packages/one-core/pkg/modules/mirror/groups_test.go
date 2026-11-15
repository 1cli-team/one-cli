package mirror

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

func TestGroupSelectionsAreIndependentAndPersistTogether(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if _, err := Select(t.Context(), EnvGroup, "huawei", false); err != nil {
		t.Fatal(err)
	}
	if _, err := Select(t.Context(), ToolsGroup, "gh-proxy", true); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Tools != nil {
		t.Fatal("preview changed tools")
	}
	if _, err := Select(t.Context(), ToolsGroup, "gh-proxy", false); err != nil {
		t.Fatal(err)
	}
	c, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Env.Preset != "huawei" || c.Tools.Preset != "gh-proxy" {
		t.Fatalf("config=%+v", c)
	}
	resolved, err := Resolve(t.TempDir(), []string{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resolved.MiseSource, githubProxy) {
		t.Fatalf("mise source=%s", resolved.MiseSource)
	}
	var rules map[string]string
	raw, _ := lookup(resolved.Env, "MISE_URL_REPLACEMENTS")
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		t.Fatal(err)
	}
	if rules[goArchivePattern] == "" || len(rules) < 4 {
		t.Fatalf("missing independent rewrite groups: %s", raw)
	}
	if err := Reset(t.Context(), EnvGroup, false); err != nil {
		t.Fatal(err)
	}
	resolved, err = Resolve(t.TempDir(), resolved.Env)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lookup(resolved.Env, "npm_config_registry"); ok {
		t.Fatal("env reset retained registry")
	}
	if !strings.HasPrefix(resolved.MiseSource, githubProxy) {
		t.Fatal("env reset changed tools")
	}
	raw, _ = lookup(resolved.Env, "MISE_URL_REPLACEMENTS")
	if strings.Contains(raw, "dl.google.com") {
		t.Fatal("env reset retained Go rewrite")
	}
	if err := Reset(t.Context(), ToolsGroup, false); err != nil {
		t.Fatal(err)
	}
	c, err = Load()
	if err != nil || c != nil {
		t.Fatalf("reset groups=%+v %v", c, err)
	}
}

func TestConcurrentGroupUpdatesDoNotLoseSelections(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for group, id := range map[string]string{EnvGroup: "public", ToolsGroup: "gh-proxy"} {
		wg.Go(func() { _, err := Select(context.Background(), group, id, false); errs <- err })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Env == nil || c.Tools == nil {
		t.Fatal("one group update overwrote the other")
	}
}

func TestLegacySelectionMigratesWithoutEnablingProxy(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	path, _ := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	sources := Presets(EnvGroup)[1].Sources
	sources["mise"] = "https://mise.jdx.dev"
	old, err := json.Marshal(map[string]any{"version": 1, "preset": "huawei", "sources": sources})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Version != 2 || c.Env.Preset != "huawei" || Current(c, ToolsGroup).Sources["mise"] != "https://mise.jdx.dev" {
		t.Fatalf("migration=%+v", c)
	}
	if Current(c, ToolsGroup).Sources["process-compose"] != composeReleases {
		t.Fatal("migration enabled proxy")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != string(old) {
		t.Fatal("read rewrote legacy config")
	}
	if err := Reset(t.Context(), EnvGroup, false); err != nil {
		t.Fatal(err)
	}
	c, err = Load()
	if err != nil || c.Env != nil || c.Tools == nil {
		t.Fatalf("partial reset=%+v %v", c, err)
	}
}

func TestToolsResetLeavesEnvironmentSources(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if _, err := Select(t.Context(), EnvGroup, "public", false); err != nil {
		t.Fatal(err)
	}
	if _, err := Select(t.Context(), ToolsGroup, "gh-proxy", false); err != nil {
		t.Fatal(err)
	}
	before, err := Resolve(t.TempDir(), []string{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Reset(t.Context(), ToolsGroup, false); err != nil {
		t.Fatal(err)
	}
	after, err := Resolve(t.TempDir(), before.Env)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := lookup(after.Env, "npm_config_registry"); value != "https://registry.npmmirror.com/" {
		t.Fatal("tools reset changed env")
	}
	raw, _ := lookup(after.Env, "MISE_URL_REPLACEMENTS")
	if strings.Contains(raw, "gh-proxy") {
		t.Fatal("tools reset retained proxy")
	}
	if after.MiseSource != Current(nil, ToolsGroup).Sources["mise"] {
		t.Fatal("tools reset did not restore mise")
	}
}

func TestToolProxyIsScopedAndPreservesCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, file, data string
		env              []string
	}{
		{name: "public"},
		{name: "token", env: []string{"GITHUB_TOKEN=example-secret"}},
		{name: "netrc", file: ".netrc", data: "machine github.com login example password secret"},
		{name: "config token", file: ".config/mise/github_tokens.toml", data: "placeholder"},
		{name: "oauth", file: ".local/state/mise/github-oauth-tokens.toml", data: "placeholder"},
		{name: "credential command", file: "mise.toml", data: "[settings.github]\ncredential_command='echo secret'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("ONE_HOME", t.TempDir())
			if tc.file != "" {
				path := filepath.Join(home, tc.file)
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tc.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Select(t.Context(), ToolsGroup, "gh-proxy", false); err != nil {
				t.Fatal(err)
			}
			r, err := Resolve(home, append([]string{}, tc.env...))
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := lookup(r.Env, "MISE_URL_REPLACEMENTS")
			if tc.name != "public" {
				if strings.Contains(raw, "gh-proxy") {
					t.Fatal("credentials were eligible for proxying")
				}
				return
			}
			var rules map[string]string
			if err := json.Unmarshal([]byte(raw), &rules); err != nil {
				t.Fatal(err)
			}
			for _, url := range []string{"https://github.com/private/repo/releases/download/v1/tool.zip", "https://api.github.com/user", "https://github.com/F1bonacc1/process-compose/releases/download/v1/file.zip?token=secret"} {
				for pattern := range rules {
					if regexp.MustCompile(strings.TrimPrefix(pattern, "regex:")).MatchString(url) {
						t.Fatalf("proxy matched unrelated/credential URL %s", url)
					}
				}
			}
		})
	}
}
