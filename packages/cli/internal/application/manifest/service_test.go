package manifest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func seedManifest(t *testing.T) (string, *Service, string) {
	t.Helper()
	root := t.TempDir()
	value := true
	manifest := &workspacecore.Manifest{
		Version:      workspacecore.ManifestVersion,
		Workspace:    &workspacecore.ManifestWorkspace{ID: "demo", Name: "demo"},
		Environments: &workspacecore.Environments{Names: []string{"dev", "preview", "prod"}, Default: "dev"},

		Env: &workspacecore.EnvironmentConfig{ProjectName: "old-web"},

		Projects: []workspacecore.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
			BuildVersion: "1.0.0",

			Dev: &workspacecore.ProjectDevOverride{Command: "pnpm dev"},
			Env: &workspacecore.ProjectEnvOverride{Path: "/apps/web", Inherits: &value, Keys: []string{"API_URL"}},
		}},
	}
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(catalog.Builtin())
	if err != nil {
		t.Fatal(err)
	}
	_, revision, err := workspacecore.ReadManifestSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, service, revision
}

func TestApplyManifestDraftPublishesAllowlistedFieldsAtomically(t *testing.T) {
	root, service, revision := seedManifest(t)
	result, err := service.ApplyManifestDraft(context.Background(), root, ApplyManifestInput{
		Revision: revision,
		Changes: []ProjectManifestPatch{{
			Project:     "web",
			General:     &ProjectGeneralPatch{BuildVersion: "v2.1.0", DevCommand: "pnpm start"},
			Environment: &ProjectEnvironmentPatch{Path: "/frontend", Inherits: false, Disabled: true},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 2 || result.Revision == revision {
		t.Fatalf("result = %#v", result)
	}
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	project := manifest.Projects[0]
	if project.BuildVersion != "2.1.0" || project.Dev.Command != "pnpm start" {
		t.Fatalf("general patch = %#v", project)
	}
	if project.Env.Path != "/frontend" || *project.Env.Inherits || !project.Env.Disabled {
		t.Fatalf("environment patch = %#v", project.Env)
	}
	if len(project.Env.Keys) != 1 || project.Env.Keys[0] != "API_URL" {
		t.Fatalf("environment keys were not preserved: %#v", project.Env.Keys)
	}
}

func TestApplyManifestDraftRejectsStaleRevisionWithoutWriting(t *testing.T) {
	root, service, revision := seedManifest(t)
	path := filepath.Join(root, workspacecore.ManifestFilename)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ApplyManifestDraft(context.Background(), root, ApplyManifestInput{
		Revision: revision + "-stale",
		Changes: []ProjectManifestPatch{{
			Project: "web", General: &ProjectGeneralPatch{BuildVersion: "9.9.9"},
		}},
	})
	if !errors.Is(err, ErrManifestConflict) {
		t.Fatalf("error = %v", err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatal("stale draft changed the manifest")
	}
}

func TestApplyManifestDraftRejectsUnknownFieldsAndUnsafeValues(t *testing.T) {
	for _, test := range []struct {
		name   string
		change ProjectManifestPatch
	}{
		{
			name:   "empty changes",
			change: ProjectManifestPatch{Project: "web"},
		},
		{
			name: "unsafe environment path",
			change: ProjectManifestPatch{Project: "web", Environment: &ProjectEnvironmentPatch{
				Path: "../../shared", Inherits: true,
			}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, service, revision := seedManifest(t)
			before, err := os.ReadFile(filepath.Join(root, workspacecore.ManifestFilename))
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.ApplyManifestDraft(context.Background(), root, ApplyManifestInput{
				Revision: revision, Changes: []ProjectManifestPatch{test.change},
			})
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v", err)
			}
			after, readErr := os.ReadFile(filepath.Join(root, workspacecore.ManifestFilename))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(after) != string(before) {
				t.Fatal("invalid draft changed the manifest")
			}
		})
	}
}

func TestPreviewManifestDraftReturnsCanonicalBeforeAfterWithoutWriting(t *testing.T) {
	root, service, revision := seedManifest(t)
	path := filepath.Join(root, workspacecore.ManifestFilename)
	beforeOnDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.PreviewManifestDraft(context.Background(), root, PreviewManifestInput{
		Revision: revision,
		Changes: []ProjectManifestPatch{{
			Project: "web",
			General: &ProjectGeneralPatch{BuildVersion: "v9.9.9", DevCommand: "pnpm preview"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Schema != ManifestPreviewSchema || result.Revision != revision {
		t.Fatalf("result = %#v", result)
	}

	beforeManifest, _, err := workspacecore.ReadManifestSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	beforeCanonical, err := workspacecore.MarshalManifest(beforeManifest)
	if err != nil {
		t.Fatal(err)
	}
	if result.Before != string(beforeCanonical) {
		t.Fatalf("before preview is not canonical:\nwant:\n%s\n\ngot:\n%s", string(beforeCanonical), result.Before)
	}

	var afterManifest workspacecore.Manifest
	if err := json.Unmarshal([]byte(result.After), &afterManifest); err != nil {
		t.Fatalf("preview after is not valid json: %v", err)
	}
	afterCanonical, err := workspacecore.MarshalManifest(&afterManifest)
	if err != nil {
		t.Fatal(err)
	}
	if result.After != string(afterCanonical) {
		t.Fatalf("after preview is not canonical:\nwant:\n%s\n\ngot:\n%s", string(afterCanonical), result.After)
	}
	if len(afterManifest.Projects) != 1 || afterManifest.Projects[0].BuildVersion != "9.9.9" {
		t.Fatalf("preview after did not include requested patch: %#v", afterManifest.Projects)
	}
	if afterManifest.Projects[0].Dev == nil || afterManifest.Projects[0].Dev.Command != "pnpm preview" {
		t.Fatalf("preview after dev command mismatch: %#v", afterManifest.Projects[0].Dev)
	}

	afterOnDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterOnDisk) != string(beforeOnDisk) {
		t.Fatal("preview draft changed the manifest on disk")
	}
}

func TestPreviewManifestDraftRejectsStaleRevisionWithoutWriting(t *testing.T) {
	root, service, revision := seedManifest(t)
	path := filepath.Join(root, workspacecore.ManifestFilename)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.PreviewManifestDraft(context.Background(), root, PreviewManifestInput{
		Revision: revision + "-stale",
		Changes: []ProjectManifestPatch{{
			Project: "web",
			General: &ProjectGeneralPatch{BuildVersion: "9.9.9"},
		}},
	})
	if !errors.Is(err, ErrManifestConflict) {
		t.Fatalf("error = %v", err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatal("stale preview changed the manifest")
	}
}

func TestWorkspaceBindingAndProjectChangesPublishTogether(t *testing.T) {
	root, service, revision := seedManifest(t)
	id, name, site := "remote-project", "Shared", "https://app.infisical.com"
	binding := &WorkspaceManifestPatch{Environment: &WorkspaceEnvironmentPatch{Backend: "infisical", ProjectID: &id, ProjectName: &name, SiteURL: &site}}
	before, _ := os.ReadFile(workspacecore.ManifestPath(root))
	_, err := service.ApplyManifestDraft(context.Background(), root, ApplyManifestInput{Revision: revision, Workspace: binding, Changes: []ProjectManifestPatch{{Project: "missing", General: &ProjectGeneralPatch{BuildVersion: "2.0.0"}}}})
	if err == nil {
		t.Fatal("invalid project accepted")
	}
	after, _ := os.ReadFile(workspacecore.ManifestPath(root))
	if string(before) != string(after) {
		t.Fatal("failed project patch partially wrote workspace binding")
	}
	_, err = service.PreviewManifestDraft(context.Background(), root, PreviewManifestInput{Revision: revision, Workspace: binding})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ApplyManifestDraft(context.Background(), root, ApplyManifestInput{Revision: revision, Workspace: binding})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	config := manifest.Env
	if config == nil || config.ProjectID != id || config.SiteURL != site || config.ProjectName != name {
		t.Fatalf("binding: %#v", config)
	}
}

func TestDevURLCanExistWithoutOverrideAndSurvivesLegacyPatches(t *testing.T) {
	root, service, revision := seedManifest(t)
	url := "http://localhost:3000/"
	apply := func(patch *ProjectGeneralPatch) error {
		result, err := service.ApplyManifestDraft(context.Background(), root, ApplyManifestInput{Revision: revision, Changes: []ProjectManifestPatch{{Project: "web", General: patch}}})
		if err == nil {
			revision = result.Revision
		}
		return err
	}
	if err := apply(&ProjectGeneralPatch{DevURL: &url}); err != nil {
		t.Fatal(err)
	}
	if err := apply(&ProjectGeneralPatch{DevCommand: "pnpm start"}); err != nil {
		t.Fatal(err)
	}
	m, err := workspacecore.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.Projects[0].Dev.URL != url || m.Projects[0].Dev.Command != "pnpm start" {
		t.Fatal(m.Projects[0].Dev)
	}
	unsafe := "https://external.example/?token=x"
	if err := apply(&ProjectGeneralPatch{DevURL: &unsafe}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	empty := ""
	if err := apply(&ProjectGeneralPatch{DevURL: &empty}); err != nil {
		t.Fatal(err)
	}
	m, _ = workspacecore.ReadManifest(root)
	if m.Projects[0].Dev != nil {
		t.Fatal("empty dev override not removed")
	}
}
