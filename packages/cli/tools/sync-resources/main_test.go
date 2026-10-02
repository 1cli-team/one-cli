package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncBundledCopiesCanonicalAssetsAndMapsModuleFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "packages/templates/registry.json", "{}")
	writeTestFile(t, root, "packages/templates/go-api/go.mod", "module example")
	writeTestFile(t, root, "packages/templates/go-api/main.go", "package main")
	retiredAssets := []string{
		"AGENTS.md", "CLAUDE.md", "SKILL.md",
		".one/agents/conventions.md", ".agents/skills/example/references/guide.md",
		"nested/AGENTS.md",
		"pnpm-lock.yaml", "nested/package-lock.json", "yarn.lock",
		"bun.lock", "bun.lockb", "npm-shrinkwrap.json",
		"node_modules/p/a.js", "dist/app.js", "apps/ui/build/app.js", "go.work", "go.work.sum", "tsconfig.tsbuildinfo",
	}
	for _, rel := range retiredAssets {
		writeTestFile(t, root, "packages/templates/go-api/"+rel, "retired agent guidance")
	}

	writeTestFile(t, root, "skills/one-cli/SKILL.md", "one-cli skill")
	writeTestFile(t, root, "skills/one-migrate/SKILL.md", "retired skill")
	writeTestFile(t, root, "skills/one-cli/references/old.md", "retired reference")
	writeTestFile(t, root, "packages/agent-skills/skills/one-cli/SKILL.md", "current skill")
	if err := syncBundled(root); err != nil {
		t.Fatalf("syncBundled: %v", err)
	}
	bundled := filepath.Join(root, "packages", "cli", "internal", "resources", "bundled")
	for _, rel := range []string{"registry.json", "_templates/go-api/main.go", "_templates/go-api/_go.mod", "_skills/one-cli/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(bundled, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected %s: %v", rel, err)
		}
	}
	for _, rel := range []string{"_templates/registry.json", "_templates/go-api/go.mod", "_skills/one-migrate", "_skills/one-cli/references/old.md"} {
		if _, err := os.Stat(filepath.Join(bundled, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("expected %s to be stripped, stat err=%v", rel, err)
		}
	}
	for _, rel := range retiredAssets {
		if _, err := os.Stat(filepath.Join(bundled, "_templates", "go-api", filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("retired agent asset was bundled: %s (err=%v)", rel, err)
		}
	}
}

func TestEnsureGeneratedTargetRejectsBroadOrOutsidePaths(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "packages", "cli", "internal", "resources", "bundled")
	if err := ensureGeneratedTarget(root, generated); err == nil {
		t.Fatal("expected generated root itself to be rejected")
	}
	if err := ensureGeneratedTarget(root, filepath.Join(root, "packages")); err == nil {
		t.Fatal("expected outside target to be rejected")
	}
	if err := ensureGeneratedTarget(root, filepath.Join(generated, "_web")); err != nil {
		t.Fatalf("expected generated child to be accepted: %v", err)
	}
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSyncBundledRejectsReservedNamesBeforeReplacingTemplates(t *testing.T) {
	for _, bad := range []string{"_go.mod", "main.go.hbs"} {
		t.Run(bad, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "packages/templates/registry.json", "{}")
			writeTestFile(t, root, "packages/templates/go-api/"+bad, "invalid")
			writeTestFile(t, root, "packages/cli/internal/resources/bundled/_templates/keep", "previous bundle")
			if err := syncBundled(root); err == nil {
				t.Fatal("reserved source name accepted")
			}
			data, err := os.ReadFile(filepath.Join(root, "packages/cli/internal/resources/bundled/_templates/keep"))
			if err != nil || string(data) != "previous bundle" {
				t.Fatal("replaced previous bundle on validation failure")
			}
		})
	}
}
