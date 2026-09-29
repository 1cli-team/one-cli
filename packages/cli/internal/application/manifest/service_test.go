package manifest

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func seedManifest(t *testing.T) (string, *Service, string) {
	t.Helper()
	root := t.TempDir()
	raw := []byte("# Team settings\nversion=2\n[workspace]\nid='demo'\nname='Demo'\n[projects.web]\npath='apps/web'\ntoolchain='node'\n[env.infisical]\nprojectId='old' # binding\nenvironments=['dev','staging','prod']\n")
	if err := os.WriteFile(workspacecore.ManifestPath(root), raw, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := NewService(catalog.Builtin())
	if err != nil {
		t.Fatal(err)
	}
	_, rev, err := workspacecore.ReadManifestSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, s, rev
}
func binding(id string) *WorkspaceManifestPatch {
	return &WorkspaceManifestPatch{Environment: &WorkspaceEnvironmentPatch{Backend: "infisical", ProjectID: &id}}
}
func TestPreviewMatchesPublishedTOMLAndPreservesUserText(t *testing.T) {
	root, s, rev := seedManifest(t)
	ctx := context.Background()
	path := workspacecore.ManifestPath(root)
	original, _ := os.ReadFile(path)
	preview, err := s.PreviewManifestDraft(ctx, root, PreviewManifestInput{Revision: rev, Workspace: binding("new")})
	if err != nil {
		t.Fatal(err)
	}
	afterPreview, _ := os.ReadFile(path)
	if preview.Before != string(original) || !bytes.Equal(original, afterPreview) {
		t.Fatal("preview changed source")
	}
	if !strings.Contains(preview.After, "# Team settings") || !strings.Contains(preview.After, "# binding") || !strings.Contains(preview.After, "path='apps/web'") {
		t.Fatal(preview.After)
	}
	m, err := workspacecore.ParseManifest([]byte(preview.After))
	if err != nil || m.Env.ProjectID != "new" {
		t.Fatalf("invalid preview: %+v %v", m, err)
	}
	result, err := s.ApplyManifestDraft(ctx, root, ApplyManifestInput{Revision: rev, Workspace: binding("new")})
	if err != nil {
		t.Fatal(err)
	}
	published, _ := os.ReadFile(path)
	if string(published) != preview.After || result.Revision == rev || result.Applied != 1 {
		t.Fatalf("preview differs from publication: %+v", result)
	}
}
func TestStaleDraftAndPreviewDoNotWrite(t *testing.T) {
	root, s, rev := seedManifest(t)
	path := workspacecore.ManifestPath(root)
	original, _ := os.ReadFile(path)
	for _, preview := range []bool{false, true} {
		var err error
		if preview {
			_, err = s.PreviewManifestDraft(context.Background(), root, PreviewManifestInput{Revision: rev + "stale", Workspace: binding("new")})
		} else {
			_, err = s.ApplyManifestDraft(context.Background(), root, ApplyManifestInput{Revision: rev + "stale", Workspace: binding("new")})
		}
		if !errors.Is(err, ErrManifestConflict) {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(original, after) {
			t.Fatal("stale edit changed source")
		}
	}
}
func TestInvalidProjectEditDoesNotPartiallyPublishBinding(t *testing.T) {
	root, s, rev := seedManifest(t)
	path := workspacecore.ManifestPath(root)
	original, _ := os.ReadFile(path)
	for _, name := range []string{"web", "missing"} {
		_, err := s.ApplyManifestDraft(context.Background(), root, ApplyManifestInput{Revision: rev, Workspace: binding("new"), Changes: []ProjectManifestPatch{{Project: name}}})
		if err == nil {
			t.Fatal("accepted project edit")
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(original, after) {
			t.Fatal("partially published")
		}
	}
}
func TestInvalidBindingCannotBePublished(t *testing.T) {
	root, s, rev := seedManifest(t)
	path := workspacecore.ManifestPath(root)
	original, _ := os.ReadFile(path)
	for _, site := range []string{"http://external.example", "https://user:pass@example.com", "https://example.com/?token=x"} {
		patch := binding("new")
		patch.Environment.SiteURL = &site
		_, err := s.ApplyManifestDraft(context.Background(), root, ApplyManifestInput{Revision: rev, Workspace: patch})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatal(err)
		}
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(original, after) {
		t.Fatal("invalid binding changed source")
	}
}
