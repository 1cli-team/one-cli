package toolchain_test

import (
	"sort"
	"testing"

	// Side-effect import: bundled adapters register themselves on init.
	// We deliberately import the public API only, mirroring how
	// downstream consumers will write tests.
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/toolchain"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

func TestRegistry_BundledAdaptersPresent(t *testing.T) {
	got := toolchain.Registered()
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	want := []toolchain.Toolchain{toolchain.Go, toolchain.Node}
	if len(got) < len(want) {
		t.Fatalf("registry has %d entries, want at least %d (got=%v)", len(got), len(want), got)
	}
	// Spot-check the two we ship by default.
	for _, id := range want {
		a := toolchain.Get(id)
		if a == nil {
			t.Errorf("Get(%q) returned nil; bundled adapter missing", id)
			continue
		}
		if a.ID() != id {
			t.Errorf("Get(%q).ID() = %q, want %q", id, a.ID(), id)
		}
	}
}

func TestRegistry_UnknownDefaultsToNode(t *testing.T) {
	a := toolchain.Get(toolchain.Toolchain("rust"))
	if a == nil {
		t.Fatal("Get(unknown) returned nil; expected fallback to Node")
	}
	if a.ID() != toolchain.Node {
		t.Errorf("Get(unknown).ID() = %q, want %q", a.ID(), toolchain.Node)
	}
}

func TestRegistry_RegisterOverwrites(t *testing.T) {
	// Save existing for restore.
	original := toolchain.Get(toolchain.Node)

	stub := stubAdapter{id: toolchain.Node}
	toolchain.Register(stub)
	if got := toolchain.Get(toolchain.Node); got.ID() != toolchain.Node {
		t.Errorf("expected stub id == Node, got %q", got.ID())
	}
	// Different concrete type than the original confirms overwrite.
	if _, isStub := toolchain.Get(toolchain.Node).(stubAdapter); !isStub {
		t.Errorf("Register did not overwrite existing entry")
	}

	// Restore so other tests aren't affected.
	toolchain.Register(original)
}

type stubAdapter struct{ id toolchain.Toolchain }

func (s stubAdapter) ID() toolchain.Toolchain { return s.id }
func (stubAdapter) UsesPackageManager() bool  { return false }
func (stubAdapter) InstallPlan(toolchain.PlanInput) toolchain.CommandStep {
	return toolchain.CommandStep{}
}
func (stubAdapter) PackageManagerForManifest(p toolchain.PackageManager) toolchain.PackageManager {
	return p
}
func (stubAdapter) ResolveRuntime(toolchain.PlanInput) toolchain.RuntimeResolution {
	return toolchain.RuntimeResolution{}
}
func (stubAdapter) RenderDockerfile(toolchain.DockerfileInput) string { return "" }
func (stubAdapter) RenderWorkflow(toolchain.WorkflowInput) string     { return "" }
