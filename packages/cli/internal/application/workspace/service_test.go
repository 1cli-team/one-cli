package workspace

import (
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestServiceOverviewIsAReadOnlyProjection(t *testing.T) {
	root := seedProjectSettingsWorkspace(t)
	before := snapshotWorkspaceTree(t, root)
	service, err := NewService(catalog.Builtin(), projectProfileStub())
	if err != nil {
		t.Fatal(err)
	}
	overview, err := service.Overview(root, "preview")
	if err != nil {
		t.Fatal(err)
	}
	if !overview.Present || overview.Root != root || len(overview.Projects) != 1 ||
		overview.Projects[0].Name != "web" {
		t.Fatalf("overview = %#v", overview)
	}
	assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)
}

func TestFindProjectDoesNotInventManifestState(t *testing.T) {
	manifest := &workspacecore.Manifest{Projects: []workspacecore.ManifestProject{{Name: "web"}}}
	if findProject(manifest, "web") == nil || findProject(manifest, "ghost") != nil {
		t.Fatalf("findProject returned unexpected result")
	}
}
