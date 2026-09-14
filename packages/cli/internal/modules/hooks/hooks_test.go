package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// Git exports repository-local paths when running this suite from a hook.
	// Each fixture and its linked worktrees must resolve their own Git state.
	for _, key := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES"} {
		t.Setenv(key, "") // Register restoration of the original environment.
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	write(t, root, "one.manifest.json", `{"version":1,"workspace":{"id":"hooks","name":"hooks"},"projects":[]}`)
	cmd := exec.Command("git", "init", "-q", root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	return root
}

func write(t *testing.T, root, path, value string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, root, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestConfigureDryRunIdempotentAndUserOverrides(t *testing.T) {
	root := fixture(t)
	ctx := context.Background()
	binary := filepath.Join(t.TempDir(), "one with space", "one")
	preview, err := Configure(ctx, root, binary, true)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || len(preview.GitChanges) != 2 || len(preview.Changes) != 3 {
		t.Fatalf("preview: %+v", preview)
	}
	for _, path := range []string{"hk.pkl", workspace.HooksConfigFilename, workspace.MiseConfigFilename, ".git/hooks/pre-commit"} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote %s", path)
		}
	}
	if _, err := Configure(ctx, root, binary, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, root, workspace.MiseConfigFilename), "aqua:jdx/hk") {
		t.Fatal("hk not declared in mise")
	}
	custom := read(t, root, "hk.pkl") + "\n// Team overrides\nhooks { [\"pre-commit\"] { enabled = false } }\n"
	write(t, root, "hk.pkl", custom)
	again, err := Configure(ctx, root, binary, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Changes) != 0 || len(again.GitChanges) != 0 || read(t, root, "hk.pkl") != custom {
		t.Fatalf("not idempotent: %+v", again)
	}
	if !strings.Contains(read(t, root, ".git/hooks/commit-msg"), shellQuote(filepath.ToSlash(binary))+" hk run commit-msg \"$@\"") {
		t.Fatal("missing quoted fallback launcher")
	}
}

func TestConfigureConflictsDoNotPublish(t *testing.T) {
	for _, file := range []string{"hk.pkl", ".git/hooks/pre-commit", ".git/hooks/commit-msg", "commitlint.config.js", "commitlint.config.ts", ".commitlintrc.json", ".husky/commit-msg", ".husky/pre-push"} {
		t.Run(file, func(t *testing.T) {
			root := fixture(t)
			write(t, root, file, "user-owned content\n")
			if _, err := Configure(context.Background(), root, "", false); err == nil {
				t.Fatal("expected a conflict")
			}
			if read(t, root, file) != "user-owned content\n" {
				t.Fatal("user file changed")
			}
			if _, err := os.Stat(filepath.Join(root, workspace.HooksConfigFilename)); !os.IsNotExist(err) {
				t.Fatal("published a partial configuration")
			}
		})
	}
	t.Run("package commitlint rules", func(t *testing.T) {
		root := fixture(t)
		write(t, root, "package.json", `{"commitlint":{"rules":{"header-max-length":[2,"always",50]}}}`)
		if _, err := Configure(context.Background(), root, "", false); err == nil {
			t.Fatal("expected conflict")
		}
		if _, err := os.Stat(filepath.Join(root, "hk.pkl")); !os.IsNotExist(err) {
			t.Fatal("published a partial configuration")
		}
	})
	t.Run("custom hook directory", func(t *testing.T) {
		root := fixture(t)
		if _, err := gitOutput(context.Background(), root, "config", "--local", "core.hooksPath", ".custom-hooks"); err != nil {
			t.Fatal(err)
		}
		if _, err := Configure(context.Background(), root, "", false); err == nil {
			t.Fatal("expected conflict")
		}
		if value, _ := hooksPath(context.Background(), root); value != ".custom-hooks" {
			t.Fatal("custom hooksPath changed")
		}
	})
}

func TestConfigureLinkedWorktreeUsesCommonGitHooks(t *testing.T) {
	root := fixture(t)
	ctx := context.Background()
	for _, args := range [][]string{
		{"add", "one.manifest.json"},
		{"-c", "user.name=One hook test", "-c", "user.email=test@example.invalid", "commit", "-qm", "test fixture"},
	} {
		if out, err := gitOutput(ctx, root, args...); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	linked := filepath.Join(t.TempDir(), "linked")
	if out, err := gitOutput(ctx, root, "worktree", "add", "--detach", linked); err != nil {
		t.Fatalf("git worktree: %v %s", err, out)
	}
	result, err := Configure(ctx, linked, "", false)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(root, ".git"))
	if err != nil || result.GitDirectory != want {
		t.Fatalf("common Git directory: got %s, want %s, error %v", result.GitDirectory, want, err)
	}
	if !strings.Contains(read(t, root, ".git/hooks/pre-commit"), "hk run pre-commit") {
		t.Fatal("common launcher missing")
	}
	if info, err := os.Stat(filepath.Join(linked, ".git")); err != nil || info.IsDir() {
		t.Fatalf("linked worktree Git file replaced: %v", err)
	}
	if _, err := os.Stat(filepath.Join(linked, "hk.pkl")); err != nil {
		t.Fatal("worktree hk configuration missing")
	}
}

func TestConfigureMigratesOnlyLegacyDefaults(t *testing.T) {
	testLegacyMigration(t, fixture(t))
}

func TestConfigureMigratesLegacyDefaultsThroughSymlink(t *testing.T) {
	root := fixture(t)
	alias := filepath.Join(t.TempDir(), "workspace-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	testLegacyMigration(t, alias)
}

func testLegacyMigration(t *testing.T, root string) {
	t.Helper()
	for path, content := range legacyFiles {
		write(t, root, path, content)
	}
	write(t, root, "package.json", `{"name":"keep","private":true,"packageManager":"pnpm@12.3.4","scripts":{"prepare":"husky","other":"keep-me"},"devDependencies":{"husky":"9.0.0","@commitlint/cli":"19.0.0","@commitlint/config-conventional":"19.0.0","@changesets/cli":"2.0.0"},"custom":{"keep":true}}`)
	write(t, root, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
	if _, err := gitOutput(context.Background(), root, "config", "--local", "core.hooksPath", ".husky/_"); err != nil {
		t.Fatal(err)
	}
	preview, err := Configure(context.Background(), root, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if preview.HooksPath == "" || len(preview.Warnings) == 0 {
		t.Fatalf("missing migration details: %+v", preview)
	}
	if _, err := Configure(context.Background(), root, "", false); err != nil {
		t.Fatal(err)
	}
	for path := range legacyFiles {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("legacy file remains: %s", path)
		}
	}
	var pkg map[string]any
	if err := json.Unmarshal([]byte(read(t, root, "package.json")), &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg["name"] != "keep" || pkg["custom"] == nil || pkg["scripts"].(map[string]any)["other"] != "keep-me" {
		t.Fatal("lost unrelated package settings")
	}
	deps := pkg["devDependencies"].(map[string]any)
	if len(deps) != 1 || deps["@changesets/cli"] == nil {
		t.Fatalf("deps: %+v", deps)
	}
	// Installation resolves the common Git directory, including Windows short
	// paths and workspace symlinks. Compare the same canonical directory here.
	wantPath, err := filepath.EvalSymlinks(filepath.Join(root, ".git", "hooks"))
	if err != nil {
		t.Fatal(err)
	}
	if value, err := hooksPath(context.Background(), root); err != nil || value != filepath.ToSlash(wantPath) {
		t.Fatalf("hooksPath = %q, want %q, error %v", value, filepath.ToSlash(wantPath), err)
	}
	if read(t, root, "pnpm-lock.yaml") != "lockfileVersion: '9.0'\n" {
		t.Fatal("rewrote package manager lockfile")
	}
}

func TestGeneratedLanguageStepsAndConcurrentEdit(t *testing.T) {
	root := fixture(t)
	write(t, root, "apps/web/package.json", `{"devDependencies":{"oxlint":"1.82.0","oxfmt":"0.67.0"}}`)
	m := &workspace.Manifest{Projects: []workspace.ManifestProject{
		{Name: "api", RelativeDir: "services/api", Toolchain: "go"},
		{Name: "web", RelativeDir: "apps/web", Toolchain: "node", PackageManager: "npm"},
	}}
	p := fsutil.NewFilePlan(root)
	if err := PlanFiles(p, m); err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	config := read(t, root, workspace.HooksConfigFilename)
	for _, part := range []string{`["api:format"]`, `dir = "services/api"`, `__hook-gofmt`, `["web:lint"]`, `Builtins.oxfmt`, `"npm", "exec", "--offline", "--"`, `fix = false`, `stage = false`, `stash = "git"`, `check-conventional-commit`} {
		if !strings.Contains(config, part) {
			t.Errorf("missing %s", part)
		}
	}
	if strings.Contains(config, "tidy") {
		t.Fatal("implicit go mod tidy")
	}
	p = fsutil.NewFilePlan(root)
	if err := PlanFiles(p, m); err != nil {
		t.Fatal(err)
	}
	write(t, root, workspace.HooksConfigFilename, config+"// concurrent edit\n")
	if err := p.Apply(context.Background()); err == nil {
		t.Fatal("expected concurrent edit conflict")
	}
	p = fsutil.NewFilePlan(root)
	if err := PlanFiles(p, m); err == nil {
		t.Fatal("modified generated configuration was accepted")
	}
}

func TestRealHKValidatesGeneratedPkl(t *testing.T) {
	binary := os.Getenv("ONE_TEST_HK_BINARY")
	if binary == "" {
		t.Skip("set ONE_TEST_HK_BINARY for native hk integration")
	}
	t.Setenv("PATH", filepath.Dir(binary)+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := fixture(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	write(t, root, "apps/web/package.json", `{"devDependencies":{"oxlint":"1.82.0","oxfmt":"0.67.0"}}`)
	p := fsutil.NewFilePlan(root)
	m := &workspace.Manifest{Projects: []workspace.ManifestProject{{Name: "api", RelativeDir: "services/api", Toolchain: "go"}, {Name: "web", RelativeDir: "apps/web", Toolchain: "node", PackageManager: "pnpm"}}}
	if err := PlanFiles(p, m); err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "validate")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hk validate: %v %s", err, out)
	}
	for _, test := range []struct {
		message string
		valid   bool
	}{{"feat(api): 支持 Go 工作区\n", true}, {"fix!: change API\n\nBREAKING CHANGE: API changed\n", true}, {"fixup! temporary\n", true}, {"not a conventional commit\n", false}, {"unknown: test\n", false}} {
		write(t, root, "commit message.txt", test.message)
		cmd := exec.Command(binary, "run", "commit-msg", filepath.Join(root, "commit message.txt"))
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if (err == nil) != test.valid {
			t.Fatalf("message %q: %v %s", test.message, err, out)
		}
		if !bytes.Equal([]byte(read(t, root, "commit message.txt")), []byte(test.message)) {
			t.Fatal("commit message changed")
		}
	}
}
