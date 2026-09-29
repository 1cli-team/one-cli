package infisical

import (
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestLoadWorkspaceConfig_FromManifest(t *testing.T) {
	tmp := t.TempDir()
	if err := workspace.WriteManifest(tmp, &workspace.Manifest{
		Version: workspace.ManifestVersion,

		Env: &workspace.EnvironmentConfig{ProjectID: "proj-x", Environments: []string{"dev", "prod"}},

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
	if cfg.SiteURL != DefaultSiteURL {
		t.Fatalf("omitted siteUrl resolved to %q", cfg.SiteURL)
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

func TestFixedDefaultsIndependentOfOrder(t *testing.T) {
	cfg := &WorkspaceConfig{Environments: []string{"prod", "dev"}}
	if cfg.DefaultEnvOrFallback() != "dev" || cfg.RootPathOrDefault() != "/" {
		t.Fatal(cfg)
	}
}

func TestOmittedSiteCannotUseAnotherSessionInstance(t *testing.T) {
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{Version: 2, Env: &workspace.EnvironmentConfig{ProjectID: "p", Environments: []string{"dev"}}}); err != nil {
		t.Fatal(err)
	}
	_, _, err := resolveCfgAndCreds(root, &WorkspaceConfig{SiteURL: "https://other.example"}, &Credentials{AccessToken: "synthetic"})
	if err == nil {
		t.Fatal("omitted siteUrl followed a different instance")
	}
}
