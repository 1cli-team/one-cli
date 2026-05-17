package infisical

import (
	"encoding/json"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func TestLoadWorkspaceConfig_FromManifest(t *testing.T) {
	tmp := t.TempDir()
	cfgRaw, _ := json.Marshal(map[string]string{"projectId": "proj-x"})
	if err := workspace.WriteManifest(tmp, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Environments: &workspace.Environments{
			Names:   []string{"dev", "prod"},
			Default: "dev",
		},
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{
				Kind:   workspace.EnvBackendInfisical,
				Config: cfgRaw,
			},
		},
		Projects: []workspace.ManifestProject{},
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadWorkspaceConfig(tmp)
	if err != nil {
		t.Fatalf("LoadWorkspaceConfig: %v", err)
	}
	if cfg == nil || cfg.ProjectID != "proj-x" {
		t.Fatalf("cfg = %+v; want ProjectID=proj-x", cfg)
	}
	if cfg.DefaultEnvOrFallback() != "dev" {
		t.Errorf("default env = %q; want dev", cfg.DefaultEnvOrFallback())
	}
}

func TestLoadWorkspaceConfig_NilWhenNoEnv(t *testing.T) {
	tmp := t.TempDir()
	if err := workspace.WriteManifest(tmp, &workspace.Manifest{
		Version:  workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{},
	}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadWorkspaceConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Errorf("expected nil cfg when manifest has no env block; got %+v", cfg)
	}
}

func TestLoadWorkspaceConfig_NilWhenEnvIsDotenv(t *testing.T) {
	tmp := t.TempDir()
	if err := workspace.WriteManifest(tmp, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendDotenv},
		},
		Projects: []workspace.ManifestProject{},
	}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadWorkspaceConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Errorf("expected nil cfg when env backend is dotenv; got %+v", cfg)
	}
}

func TestLoadSubprojectConfig_FromManifestEntry(t *testing.T) {
	tmp := t.TempDir()
	inherits := false
	if err := workspace.WriteManifest(tmp, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{{
			Name:        "api",
			RelativeDir: "services/api",
			TemplateID:  "go-api",
			Toolchain:   "go",
			Domains: &workspace.ProjectDomains{
				Env: &workspace.ProjectEnvOverride{
					Path:     "/custom/api",
					Inherits: &inherits,
				},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := LoadSubprojectConfig(tmp, "services/api")
	if err != nil {
		t.Fatalf("LoadSubprojectConfig: %v", err)
	}
	if got == nil {
		t.Fatal("got nil; expected SubprojectConfig with Path=/custom/api")
	}
	if got.Path != "/custom/api" {
		t.Errorf("Path = %q; want /custom/api", got.Path)
	}
	if got.Inherits == nil || *got.Inherits != false {
		t.Errorf("Inherits = %v; want pointer to false", got.Inherits)
	}
}

func TestLoadSubprojectConfig_DisabledRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	if err := workspace.WriteManifest(tmp, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{{
			Name:        "web",
			RelativeDir: "apps/web",
			TemplateID:  "react-spa",
			Toolchain:   "node",
			Domains: &workspace.ProjectDomains{
				Env: &workspace.ProjectEnvOverride{Disabled: true},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := LoadSubprojectConfig(tmp, "apps/web")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || !got.Disabled {
		t.Errorf("expected Disabled=true; got %+v", got)
	}
}

func TestLoadSubprojectConfig_NilForUnknownDir(t *testing.T) {
	tmp := t.TempDir()
	if err := workspace.WriteManifest(tmp, &workspace.Manifest{
		Version:  workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSubprojectConfig(tmp, "apps/missing")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil for unknown dir; got %+v", got)
	}
}
