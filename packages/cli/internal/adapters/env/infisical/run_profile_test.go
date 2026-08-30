package infisical

import (
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestResolveProfileCredsForContextUsesEnvironmentProjectBinding(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)
	root := t.TempDir()
	manifest := &workspace.Manifest{
		Version:      workspace.ManifestVersion,
		Workspace:    &workspace.ManifestWorkspace{ID: "workspace-id", Name: "demo"},
		Environments: &workspace.Environments{Names: []string{"dev", "prod"}, Default: "dev"},
		Projects: []workspace.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", Toolchain: "node",
		}},
	}
	if err := workspace.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"development", "production"} {
		if _, err := profile.Upsert(profile.DomainEnv, "infisical", name, profile.Profile{
			Backend: "infisical",
			Infisical: &profile.InfisicalProfile{
				SiteURL: "https://example.infisical.test",
				Credentials: &profile.InfisicalCredentials{
					ClientID: "client-" + name, ClientSecret: "secret-" + name,
				},
			},
		}, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := profile.BindEnvironmentProfile(
		"workspace-id", "demo", root, "", "prod",
		profile.DomainEnv, "infisical", "development",
	); err != nil {
		t.Fatal(err)
	}
	if err := profile.BindEnvironmentProfile(
		"workspace-id", "demo", root, "web", "prod",
		profile.DomainEnv, "infisical", "production",
	); err != nil {
		t.Fatal(err)
	}

	name, credentials, siteURL, err := resolveProfileCredsForContext(
		root, "", "prod", "web",
	)
	if err != nil {
		t.Fatal(err)
	}
	if name != "production" || credentials == nil ||
		credentials.ClientID != "client-production" ||
		credentials.ClientSecret != "secret-production" ||
		siteURL != "https://example.infisical.test" {
		t.Fatalf("resolved = name=%q credentials=%#v siteURL=%q", name, credentials, siteURL)
	}
}

func TestManifestProjectNameUsesStableManifestName(t *testing.T) {
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", Toolchain: "node",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := manifestProjectName(root, "apps/web"); got != "web" {
		t.Fatalf("manifestProjectName() = %q, want web", got)
	}
	if got := manifestProjectName(root, "apps/unknown"); got != "" {
		t.Fatalf("unknown manifestProjectName() = %q", got)
	}
}
