package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManifest_RoundTrip(t *testing.T) {
	tmp := t.TempDir()

	disabled := true
	inheritsTrue := true
	infisicalCfg, err := json.Marshal(map[string]any{
		"projectId":   "proj-123",
		"rootPath":    "/",
		"projectName": "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	original := &Manifest{
		Version:   ManifestVersion,
		Workspace: &ManifestWorkspace{ID: "demo-id", Name: "demo"},
		Environments: &Environments{
			Names:   []string{"dev", "prod"},
			Default: "dev",
		},
		Domains: &WorkspaceDomains{
			Env: &BackendRef{
				Kind:   EnvBackendInfisical,
				Config: infisicalCfg,
			},
		},
		Projects: []ManifestProject{
			{
				Name:        "api",
				RelativeDir: "services/api",
				TemplateID:  "go-api",
				Toolchain:   "go",
				Domains: &ProjectDomains{
					Env: &ProjectEnvOverride{Disabled: disabled},
				},
			},
			{
				Name:           "web",
				RelativeDir:    "apps/web",
				TemplateID:     "react-spa",
				Toolchain:      "node",
				PackageManager: "pnpm",
				Domains: &ProjectDomains{
					Env: &ProjectEnvOverride{
						Path:     "/web-secrets",
						Inherits: &inheritsTrue,
					},
				},
			},
		},
	}

	if err := WriteManifest(tmp, original); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	got, err := ReadManifest(tmp)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	if got.Version != ManifestVersion {
		t.Errorf("version = %d; want %d", got.Version, ManifestVersion)
	}
	if got.Workspace == nil || got.Workspace.ID != "demo-id" || got.Workspace.Name != "demo" {
		t.Errorf("workspace identity not preserved: %+v", got.Workspace)
	}
	if got.Environments == nil || len(got.Environments.Names) != 2 || got.Environments.Default != "dev" {
		t.Errorf("environments not preserved: %+v", got.Environments)
	}
	if got.Domains == nil || got.Domains.Env == nil || got.Domains.Env.Kind != EnvBackendInfisical {
		t.Errorf("env backend not preserved: %+v", got.Domains)
	}
	if EnvConfigRaw(got) == nil {
		t.Errorf("env config not preserved")
	}
	if len(got.Projects) != 2 {
		t.Fatalf("projects count = %d; want 2", len(got.Projects))
	}
	// Sorted by relativeDir, so apps/web comes first.
	web := got.Projects[0]
	if web.Domains == nil || web.Domains.Env == nil || web.Domains.Env.Path != "/web-secrets" {
		t.Errorf("apps/web env override not preserved: %+v", web.Domains)
	}
	api := got.Projects[1]
	if api.Domains == nil || api.Domains.Env == nil || !api.Domains.Env.Disabled {
		t.Errorf("services/api env.disabled not preserved: %+v", api.Domains)
	}
}

func TestManifest_LegacyFieldsRejected(t *testing.T) {
	tmp := t.TempDir()
	v1 := []byte(`{"version":1,"updatedAt":"2026-01-01T00:00:00.000Z","subprojects":[]}` + "\n")
	if err := os.WriteFile(filepath.Join(tmp, ManifestFilename), v1, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(tmp); err == nil {
		t.Fatal("expected ReadManifest to reject legacy fields; got nil err")
	}
}

func TestManifest_V2Rejected(t *testing.T) {
	tmp := t.TempDir()
	// v2 manifest used the old `subprojects` key and is no longer accepted
	// after the v0.7 hard rename to `projects`.
	v2 := []byte(`{"version":2,"subprojects":[]}` + "\n")
	if err := os.WriteFile(filepath.Join(tmp, ManifestFilename), v2, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(tmp); err == nil {
		t.Fatal("expected ReadManifest to reject v2; got nil err")
	}
}

func TestManifest_V4Rejected(t *testing.T) {
	tmp := t.TempDir()
	v4 := []byte(`{"version":4,"projects":[]}` + "\n")
	if err := os.WriteFile(filepath.Join(tmp, ManifestFilename), v4, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(tmp); err == nil {
		t.Fatal("expected ReadManifest to reject v4; got nil err")
	}
}

func TestInitWorkspaceEnv_PreservesProjects(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteManifest(tmp, &Manifest{
		Version: ManifestVersion,
		Projects: []ManifestProject{{
			Name: "api", RelativeDir: "services/api",
			TemplateID: "go-api", Toolchain: "go",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	cfgRaw, _ := json.Marshal(map[string]string{"projectId": "p1"})
	if err := InitWorkspaceEnv(tmp, EnvInit{
		Kind:             EnvBackendInfisical,
		ConfigJSON:       cfgRaw,
		EnvironmentNames: []string{"dev", "prod"},
		DefaultEnv:       "dev",
	}); err != nil {
		t.Fatalf("InitWorkspaceEnv: %v", err)
	}

	got, err := ReadManifest(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if EnvBackend(got) != EnvBackendInfisical {
		t.Errorf("env backend not written: %+v", got.Domains)
	}
	if got.Environments == nil || got.Environments.Default != "dev" {
		t.Errorf("environments not written: %+v", got.Environments)
	}
	if len(got.Projects) != 1 {
		t.Fatalf("projects wiped: %d", len(got.Projects))
	}
}

func TestUpsertManifestProject_PreservesDomainsOverride(t *testing.T) {
	tmp := t.TempDir()

	// Seed a project with an env override.
	if err := WriteManifest(tmp, &Manifest{
		Version: ManifestVersion,
		Projects: []ManifestProject{{
			Name:        "api",
			RelativeDir: "services/api",
			TemplateID:  "go-api",
			Toolchain:   "go",
			Domains: &ProjectDomains{
				Env: &ProjectEnvOverride{Disabled: true},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	// Re-upsert (e.g. `one add` re-running on an existing project).
	if err := UpsertManifestProject(tmp, ManifestProjectInput{
		Name:        "api",
		RelativeDir: "services/api",
		TemplateID:  "go-api",
		Toolchain:   "go",
	}); err != nil {
		t.Fatalf("UpsertManifestProject: %v", err)
	}

	got, _ := ReadManifest(tmp)
	if len(got.Projects) != 1 {
		t.Fatalf("projects count = %d; want 1", len(got.Projects))
	}
	if got.Projects[0].Domains == nil || got.Projects[0].Domains.Env == nil ||
		!got.Projects[0].Domains.Env.Disabled {
		t.Errorf("env override lost on upsert: %+v", got.Projects[0].Domains)
	}
}

func TestResolveRootDirs_AlwaysReturnsDefaults(t *testing.T) {
	got, err := ResolveRootDirs(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "apps" || got[1] != "services" || got[2] != "packages" {
		t.Errorf("ResolveRootDirs = %v; want defaults", got)
	}
}

func TestResolveRootDirs_HonorsExplicitOverride(t *testing.T) {
	got, err := ResolveRootDirs(t.TempDir(), []string{"frontend", "backend"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "frontend" || got[1] != "backend" {
		t.Errorf("ResolveRootDirs override = %v; want [frontend backend]", got)
	}
}
