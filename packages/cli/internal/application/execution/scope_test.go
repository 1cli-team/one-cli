package execution

import (
	"context"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
)

func TestExecutionScopeDeriveIsImmutable(t *testing.T) {
	t.Parallel()

	parent := NewScope(context.Background(), "/work")
	child := parent.Derive(ScopePatch{
		WorkspaceRoot: "/work/demo",
		Environment:   "production",
		Backend:       catalog.BackendID{Domain: catalog.DomainEnv, Name: "infisical"},
	})
	if parent.WorkspaceRoot() != "" || parent.Environment() != "" || parent.Backend().String() != "" {
		t.Fatalf("parent mutated: %#v", parent)
	}
	if child.WorkspaceRoot() != "/work/demo" || child.Environment() != "production" ||
		child.Backend().String() != "env/infisical" {
		t.Fatalf("child = %#v", child)
	}
}
