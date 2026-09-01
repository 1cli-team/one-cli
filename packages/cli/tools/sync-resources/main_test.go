package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncBundledCopiesCanonicalAssetsAndStripsNestedModules(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "packages/templates/registry.json", "{}")
	writeTestFile(t, root, "packages/templates/go-api/go.mod", "module example")
	writeTestFile(t, root, "packages/templates/go-api/main.go", "package main")
	writeTestFile(t, root, "packages/skills/one-cli/SKILL.md", "---\nname: one-cli\n---")

	if err := syncBundled(root); err != nil {
		t.Fatalf("syncBundled: %v", err)
	}
	bundled := filepath.Join(root, "packages", "cli", "internal", "resources", "bundled")
	for _, rel := range []string{"registry.json", "skills/one-cli/SKILL.md", "_templates/go-api/main.go"} {
		if _, err := os.Stat(filepath.Join(bundled, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected %s: %v", rel, err)
		}
	}
	for _, rel := range []string{"_templates/registry.json", "_templates/go-api/go.mod"} {
		if _, err := os.Stat(filepath.Join(bundled, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("expected %s to be stripped, stat err=%v", rel, err)
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
