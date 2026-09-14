package gowork

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
)

func put(t *testing.T, root, rel, b string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(b), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIncrementalGoWorkspacePreservesUserDirectivesAndExternalMembers(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	put(t, root, "services/api/go.mod", "module example.com/api\ngo 1.24.0\n")
	put(t, parent, "external/go.mod", "module example.com/external\ngo 1.25.0\n")
	put(t, root, "packages/shared lib/go.mod", "module example.com/shared\ngo 1.25.0\n")
	put(t, root, "go.work", "// my workspace\ngo 1.24.0\ntoolchain go1.25.6\nuse (\n ./services/api // keep this\n ../external\n)\nreplace example.com/old => ../external\n")
	p := fsutil.NewFilePlan(root)
	b, info, err := Build(root, []string{"services/api", "packages/shared lib"}, p.Read)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"// my workspace", "// keep this", "toolchain go1.25.6", "../external", "replace example.com/old => ../external", `"./packages/shared lib"`} {
		if !strings.Contains(string(b), fragment) {
			t.Fatalf("lost %s: %s", fragment, b)
		}
	}
	if info.Version != "1.25.6" || len(info.Modules) != 3 {
		t.Fatalf("info = %+v", info)
	}
	put(t, root, "go.work", string(b))
	second, _, err := Build(root, []string{"services/api", "packages/shared lib"}, fsutil.NewFilePlan(root).Read)
	if err != nil || string(second) != string(b) {
		t.Fatalf("not idempotent: %s %v", second, err)
	}
}

func TestDuplicateModulesAndRemovedMembersAreDiagnosed(t *testing.T) {
	root := t.TempDir()
	put(t, root, "a/go.mod", "module example.com/same\ngo 1.25.0\n")
	put(t, root, "b/go.mod", "module example.com/same\ngo 1.25.0\n")
	if _, _, err := Build(root, []string{"a", "b"}, fsutil.NewFilePlan(root).Read); err == nil || !strings.Contains(err.Error(), "duplicate module") {
		t.Fatalf("error = %v", err)
	}
	put(t, root, "go.work", "go 1.25.0\nuse ./removed\n")
	if _, _, err := Build(root, []string{"a"}, fsutil.NewFilePlan(root).Read); err == nil || !strings.Contains(err.Error(), "missing go.mod") {
		t.Fatalf("error = %v", err)
	}
}
