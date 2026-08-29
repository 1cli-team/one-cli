package environment

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareWorkspaceOwnsDotenvSetup(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	result, err := newTestService(t).PrepareWorkspace(
		context.Background(),
		PrepareWorkspaceInput{ProjectRoot: root, ProjectName: "demo", Backend: "env/dotenv"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.InfisicalBound || result.BindWarning != nil {
		t.Fatalf("result = %#v", result)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	for _, line := range []string{".env", ".env.*", "!.env.example"} {
		if !strings.Contains(content, line) {
			t.Fatalf(".gitignore = %q, missing %q", content, line)
		}
	}
}

func TestPrepareWorkspaceRejectsUnknownBackend(t *testing.T) {
	t.Parallel()
	_, err := newTestService(t).PrepareWorkspace(
		context.Background(),
		PrepareWorkspaceInput{ProjectRoot: t.TempDir(), Backend: "unknown"},
	)
	if coded, ok := err.(interface{ ErrorCode() string }); !ok ||
		coded.ErrorCode() != "ENV_BACKEND_INVALID" {
		t.Fatalf("error = %v", err)
	}
}
