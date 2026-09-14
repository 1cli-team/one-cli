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
		"one.manifest.json":         `{"version":1,"workspace":{"id":"mise-fixture","name":"fixture"},"projects":[{"name":"web","relativeDir":"apps/web","toolchain":"node","templateId":"react-spa","domains":{"dev":{"command":"pnpm dev"}}},{"name":"api","relativeDir":"services/api","toolchain":"go","templateId":"go-api","domains":{"dev":{"command":"go run ./cmd/server"}}}]}`,
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
	if len(p.Changes) != 3 || !p.DryRun {
		t.Fatalf("unexpected plan: %+v", p)
	}
	if _, err := os.Stat(filepath.Join(root, Filename)); !os.IsNotExist(err) {
		t.Fatal("planning wrote a file")
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
	web, _ := os.ReadFile(filepath.Join(root, "apps/web", Filename))
	if !strings.Contains(string(web), "one:build") || strings.Contains(string(web), "one:lint") || strings.Contains(string(web), "vite") {
		t.Fatalf("tasks do not reference the source commands: %s", web)
	}
	if !strings.Contains(string(web), "__exec --protocol 1 --project web --operation dev") {
		t.Fatal("missing terminal execution leaf")
	}
	second, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", second.Changes)
	}
	userAfter, _ := os.ReadFile(filepath.Join(root, "mise.toml"))
	if string(userBefore) != string(userAfter) {
		t.Fatal("user TOML was modified")
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
	raw, err := os.ReadFile(filepath.Join(root, "services/api", Filename))
	if err != nil || !strings.Contains(string(raw), "1.25.6") {
		t.Fatalf("toolchain ignored: %v %s", err, raw)
	}
	if _, err := Build(root, Options{GoVersion: "1.24.0"}); err == nil {
		t.Fatal("accepted Go below go.mod minimum")
	}
}

func TestGoWorkspaceVersionAppliesToMembersWithoutAddingGoToNode(t *testing.T) {
	root := fixture(t)
	writeFixture(t, filepath.Join(root, "go.work"), "go 1.27.0\nuse ./services/api\n")
	p, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	goConfig, _ := os.ReadFile(filepath.Join(root, "services/api", Filename))
	if !strings.Contains(string(goConfig), "go = '1.27.0'") {
		t.Fatalf("workspace version ignored: %s", goConfig)
	}
	for _, rel := range []string{Filename, "apps/web/" + Filename} {
		b, _ := os.ReadFile(filepath.Join(root, rel))
		if strings.Contains(string(b), "go =") {
			t.Fatalf("Node inherited Go: %s", b)
		}
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
	path := filepath.Join(root, "apps/web", Filename)
	raw, _ := os.ReadFile(path)
	modified := string(raw) + "\n# My edit\n"
	writeFixture(t, path, modified)
	if _, err := Build(root, Options{}); err == nil {
		t.Fatal("expected ownership conflict")
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
	if _, err := os.Stat(filepath.Join(root, Filename)); !os.IsNotExist(err) {
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
	if _, err := os.Stat(filepath.Join(root, Filename)); !os.IsNotExist(err) {
		t.Fatal("stale plan wrote files")
	}
}

func TestGeneratorRejectsSymlinkAndEmptyUserFile(t *testing.T) {
	t.Run("empty user file", func(t *testing.T) {
		root := fixture(t)
		writeFixture(t, filepath.Join(root, Filename), "")
		if _, err := Build(root, Options{}); err == nil {
			t.Fatal("empty user file must not be claimed")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		root := fixture(t)
		if err := os.Symlink(t.TempDir(), filepath.Join(root, ".mise")); err != nil {
			t.Skip(err)
		}
		if _, err := Build(root, Options{}); err == nil {
			t.Fatal("symlink target must not be written")
		}
	})
}

func TestFailedWriteRestoresAlreadyWrittenFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission failure fixture is Unix-specific")
	}
	root := fixture(t)
	p, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(root, "apps/web/.mise/conf.d")
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
	if _, err := os.Stat(filepath.Join(root, Filename)); !os.IsNotExist(err) {
		t.Fatal("partial configuration was not rolled back")
	}
}
