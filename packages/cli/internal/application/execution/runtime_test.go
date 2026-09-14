package execution

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestRuntimeSelectionPreservesLegacyAndUsesGeneratedConfig(t *testing.T) {
	t.Setenv("ONE_RUNTIME", "")
	root := t.TempDir()
	if got, err := RuntimeKind(root); err != nil || got != "builtin" {
		t.Fatalf("legacy: %q %v", got, err)
	}
	path := filepath.Join(root, workspace.MiseConfigFilename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("min_version = '2026.9.7'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := RuntimeKind(root); err != nil || got != "mise" {
		t.Fatalf("configured: %q %v", got, err)
	}
	t.Setenv("ONE_RUNTIME", "builtin")
	if got, err := RuntimeKind(root); err != nil || got != "builtin" {
		t.Fatalf("override: %q %v", got, err)
	}
	t.Setenv("ONE_RUNTIME", "typo")
	if _, err := RuntimeKind(root); err == nil {
		t.Fatal("invalid runtime was ignored")
	}
}
