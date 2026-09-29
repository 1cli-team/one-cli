package miseconfig

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"one.manifest.toml": `version = 2

[workspace]
id = "mise-fixture"
name = "fixture"

[projects."web"]
path = "apps/web"
toolchain = "node"
template = "react-spa"

[projects."api"]
path = "services/api"
toolchain = "go"
template = "go-api"
`,
		"package.json":              `{"packageManager":"pnpm@10.14.0"}`,
		"apps/web/package.json":     `{"scripts":{"dev":"vite","build":"vite build","test":"vitest run"}}`,
		"services/api/go.mod":       "module example.com/api\n\ngo 1.25.0\n",
		"services/api/Taskfile.yml": "version: '3'\ntasks:\n  build:\n    cmds: ['go build ./...']\n  test:\n    cmds: ['go test ./...']\n",
		"mise.toml":                 "# My existing configuration\n[env]\nKEEP = 'yes'\n",
	}
	for rel, raw := range files {
		writeFixture(t, filepath.Join(root, rel), raw)
	}
	return root
}

func writeFixture(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConfigurationGenerationIsAdditiveAndIdempotent(t *testing.T) {
	root := fixture(t)
	userBefore, _ := os.ReadFile(filepath.Join(root, "mise.toml"))
	p, err := Build(root, Options{NodeVersion: "25.4.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes) != 2 || !p.DryRun {
		t.Fatalf("unexpected plan: %+v", p)
	}
	if actual, _ := os.ReadFile(filepath.Join(root, Filename)); string(actual) != string(userBefore) {
		t.Fatal("planning changed a file")
	}
	if err := p.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	rootRaw, _ := os.ReadFile(filepath.Join(root, Filename))
	var rootConfig config
	if err := toml.Unmarshal(rootRaw, &rootConfig); err != nil {
		t.Fatal(err)
	}
	if rootConfig.Tools["node"] != "25.4.0" || rootConfig.Tools["pnpm"] != "10.14.0" {
		t.Fatalf("tools: %+v", rootConfig.Tools)
	}
	if !rootConfig.MonorepoRoot || len(rootConfig.Monorepo.ConfigRoots) != 0 {
		t.Fatalf("expected root-only scheduling: %+v", rootConfig.Monorepo)
	}
	if rootConfig.Tasks["web:build"].Directory != "apps/web" || strings.Contains(string(rootRaw), "vite") {
		t.Fatalf("tasks do not reference the source commands: %s", rootRaw)
	}
	if rootConfig.Tasks["web:dev"].Run != "pnpm run dev" {
		t.Fatal("missing native command")
	}
	for _, dir := range []string{"apps/web", "services/api"} {
		if _, err := os.Stat(filepath.Join(root, dir, Filename)); !os.IsNotExist(err) {
			t.Fatalf("generated a project configuration in %s: %v", dir, err)
		}
	}
	second, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", second.Changes)
	}
	userAfter, _ := os.ReadFile(filepath.Join(root, "mise.toml"))
	for _, line := range strings.Split(strings.TrimSpace(string(userBefore)), "\n") {
		if !strings.Contains(string(userAfter), line) {
			t.Fatalf("lost user text %q", line)
		}
	}
}

func TestGoVersionHonorsMinimumAndToolchain(t *testing.T) {
	root := fixture(t)
	writeFixture(t, filepath.Join(root, "services/api/go.mod"), "module example.com/api\n\ngo 1.25\ntoolchain go1.25.6\n")
	p, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, Filename))
	if err != nil || !strings.Contains(string(raw), "1.25.6") {
		t.Fatalf("toolchain ignored: %v %s", err, raw)
	}
	if _, err := Build(root, Options{GoVersion: "1.24.0"}); err == nil {
		t.Fatal("accepted Go below go.mod minimum")
	}
}

func TestGoWorkspaceVersionIsPinnedAtRoot(t *testing.T) {
	root := fixture(t)
	writeFixture(t, filepath.Join(root, "go.work"), "go 1.27.0\nuse ./services/api\n")
	p, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	goConfig, _ := os.ReadFile(filepath.Join(root, Filename))
	if !strings.Contains(string(goConfig), "go = '1.27.0'") {
		t.Fatalf("workspace version ignored: %s", goConfig)
	}
	if _, err := Build(root, Options{GoVersion: "1.26.0"}); err == nil {
		t.Fatal("accepted override below workspace minimum")
	}
}

func TestManagedConfigurationConflictDoesNotOverwrite(t *testing.T) {
	root := fixture(t)
	p, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, Filename)
	raw, _ := os.ReadFile(path)
	modified := strings.Replace(string(raw), "node = '24.15.0'", "node = '25.0.0'", 1) + "\n# My edit\n"
	writeFixture(t, path, modified)
	if _, err := Build(root, Options{NodeVersion: "26.0.0"}); err == nil {
		t.Fatal("expected same-entry conflict")
	}
	after, _ := os.ReadFile(path)
	if string(after) != modified {
		t.Fatal("conflict overwrote user's edit")
	}
}

func TestPlanRejectsChangedSourceBeforeWriting(t *testing.T) {
	root := fixture(t)
	p, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(root, "apps/web/package.json"), `{"scripts":{"test":"changed"}}`)
	if err := p.Apply(context.Background()); err == nil {
		t.Fatal("expected stale-plan conflict")
	}
	if _, err := os.Stat(filepath.Join(root, "apps/web", Filename)); !os.IsNotExist(err) {
		t.Fatal("stale plan wrote files")
	}
}

func TestPlanDistinguishesEmptySourceFromRemovedSource(t *testing.T) {
	root := fixture(t)
	path := filepath.Join(root, "services/api/Taskfile.yml")
	writeFixture(t, path, "")
	p, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(context.Background()); err == nil {
		t.Fatal("source deletion was ignored")
	}
	if _, err := os.Stat(filepath.Join(root, "apps/web", Filename)); !os.IsNotExist(err) {
		t.Fatal("stale plan wrote files")
	}
}

func TestGeneratorAcceptsEmptyFileAndRejectsSymlink(t *testing.T) {
	t.Run("empty user file", func(t *testing.T) {
		root := fixture(t)
		writeFixture(t, filepath.Join(root, Filename), "")
		if _, err := Build(root, Options{}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("symlink", func(t *testing.T) {
		root := fixture(t)
		if err := os.Remove(filepath.Join(root, Filename)); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(t.TempDir(), "mise.toml"), filepath.Join(root, Filename)); err != nil {
			t.Skip(err)
		}
		if _, err := Build(root, Options{}); err == nil {
			t.Fatal("symlink target must not be written")
		}
	})
}

func TestFailedWritePreservesRootConfiguration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission failure fixture is Unix-specific")
	}
	root := fixture(t)
	p, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	blocked := root
	before, _ := os.ReadFile(filepath.Join(root, Filename))
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })
	if f, err := os.Create(filepath.Join(blocked, "probe")); err == nil {
		f.Close()
		t.Skip("process bypasses Unix directory permissions")
	}
	if err := p.Apply(context.Background()); err == nil {
		t.Fatal("expected write failure")
	}
	after, _ := os.ReadFile(filepath.Join(root, Filename))
	if string(after) != string(before) {
		t.Fatal("failed write changed root configuration")
	}
}

func TestGoWithoutTaskfileDoesNotGenerateBuildFallback(t *testing.T) {
	root := fixture(t)
	if err := os.Remove(filepath.Join(root, "services/api/Taskfile.yml")); err != nil {
		t.Fatal(err)
	}
	plan, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, Filename))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "[tasks.'api:build']") {
		t.Fatal("Go build fallback must not be generated without a Taskfile")
	}
	if strings.Contains(string(raw), "[tasks.'api:test']") {
		t.Fatal("test task must come from Taskfile")
	}
}
