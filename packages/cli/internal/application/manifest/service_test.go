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
	config, err := json.Marshal(map[string]string{"projectName": "old-web"})
	if err != nil {
		t.Fatal(err)
	}
	value := true
	manifest := &workspacecore.Manifest{
		Version:      workspacecore.ManifestVersion,
		Workspace:    &workspacecore.ManifestWorkspace{ID: "demo", Name: "demo"},
		Environments: &workspacecore.Environments{Names: []string{"dev", "preview", "prod"}, Default: "dev"},
		Domains: &workspacecore.WorkspaceDomains{
			Env: &workspacecore.BackendRef{Kind: workspacecore.EnvBackendInfisical, Config: config},
		},
		Projects: []workspacecore.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
			BuildVersion: "1.0.0",
			Domains: &workspacecore.ProjectDomains{
				Dev: &workspacecore.ProjectDevOverride{Command: "pnpm dev"},
				Env: &workspacecore.ProjectEnvOverride{Path: "/apps/web", Inherits: &value, Keys: []string{"API_URL"}},
				Container: &workspacecore.ProjectContainerOverride{
					Kind: "docker", Image: "web:latest", Namespace: "one",
				},
				Deploy: &workspacecore.ProjectDeployBackend{Kind: "vercel", Config: config},
			},
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
			Container: &ProjectContainerPatch{
				Enabled: true, Backend: "ghcr", Image: "ghcr.io/acme/web:2.1.0", Namespace: "acme",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 3 || result.Revision == revision {
		t.Fatalf("result = %#v", result)
	}
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	project := manifest.Projects[0]
	if project.BuildVersion != "2.1.0" || project.Domains.Dev.Command != "pnpm start" {
		t.Fatalf("general patch = %#v", project)
	}
	if project.Domains.Env.Path != "/frontend" || *project.Domains.Env.Inherits || !project.Domains.Env.Disabled {
		t.Fatalf("environment patch = %#v", project.Domains.Env)
	}
	if len(project.Domains.Env.Keys) != 1 || project.Domains.Env.Keys[0] != "API_URL" {
		t.Fatalf("environment keys were not preserved: %#v", project.Domains.Env.Keys)
	}
	if project.Domains.Container.Kind != "ghcr" || project.Domains.Container.Namespace != "acme" {
		t.Fatalf("container patch = %#v", project.Domains.Container)
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
			name: "unknown container backend",
			change: ProjectManifestPatch{Project: "web", Container: &ProjectContainerPatch{
				Enabled: true, Backend: "unknown",
			}},
		},
		{
			name: "unsafe environment path",
			change: ProjectManifestPatch{Project: "web", Environment: &ProjectEnvironmentPatch{
				Path: "../../shared", Inherits: true,
			}},
		},
		{
			name: "undeclared deploy config",
			change: ProjectManifestPatch{Project: "web", Deploy: &ProjectDeployPatch{
				Backend: "vercel", Config: map[string]any{"apiToken": "must-not-enter-manifest"},
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
	if afterManifest.Projects[0].Domains == nil || afterManifest.Projects[0].Domains.Dev == nil || afterManifest.Projects[0].Domains.Dev.Command != "pnpm preview" {
		t.Fatalf("preview after dev command mismatch: %#v", afterManifest.Projects[0].Domains)
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
