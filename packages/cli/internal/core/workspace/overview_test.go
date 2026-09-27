package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
)

func withIsolatedOverviewProfiles(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func TestBuildOverview_EmptyRoot(t *testing.T) {
	ov, err := BuildOverview("")
	if err != nil {
		t.Fatalf("BuildOverview(\"\"): %v", err)
	}
	if ov.Present {
		t.Errorf("present = true; want false for empty root")
	}
	if ov.Schema != OverviewSchema {
		t.Errorf("schema = %q; want %q", ov.Schema, OverviewSchema)
	}
}

func TestBuildOverview_NoManifestAtRoot(t *testing.T) {
	withIsolatedOverviewProfiles(t)
	tmp := t.TempDir()
	ov, err := BuildOverview(tmp)
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	if ov.Present {
		t.Errorf("present = true; want false (no manifest)")
	}
}

// fully populated workspace should land with zero issues — clean state is
// the case where users add things, so it has to look quiet.
func TestBuildOverview_FullyConfigured_NoIssues(t *testing.T) {
	withIsolatedOverviewProfiles(t)
	tmp := t.TempDir()
	m := &Manifest{
		Version:      ManifestVersion,
		Workspace:    &ManifestWorkspace{ID: "demo", Name: "demo"},
		Environments: &Environments{Names: []string{"dev", "prod"}, Default: "dev"},
		Domains: &WorkspaceDomains{
			Env: &BackendRef{Kind: EnvBackendDotenv},
		},
		Projects: []ManifestProject{
			{
				Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
				Domains: &ProjectDomains{
					Dev: &ProjectDevOverride{Command: "pnpm dev"},
				},
			},
		},
	}
	if err := WriteManifest(tmp, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	ov, err := BuildOverview(tmp)
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	if !ov.Present {
		t.Fatalf("present = false; want true")
	}
	if len(ov.Issues) != 0 {
		t.Errorf("workspace issues = %+v; want none", ov.Issues)
	}
	if got := len(ov.Projects); got != 1 {
		t.Fatalf("projects = %d; want 1", got)
	}
	if iss := ov.Projects[0].Issues; len(iss) != 0 {
		t.Errorf("project issues = %+v; want none", iss)
	}
	if ov.Projects[0].Kind != ProjectKindApp {
		t.Errorf("kind = %q; want %q", ov.Projects[0].Kind, ProjectKindApp)
	}
	if ov.Workspace.Domains["env"] != EnvBackendDotenv {
		t.Errorf("workspace env domain = %q", ov.Workspace.Domains["env"])
	}
	if len(ov.Projects[0].Domains) != 1 || ov.Projects[0].Domains["env"] != EnvBackendDotenv {
		t.Errorf("project domains = %#v; want only dotenv", ov.Projects[0].Domains)
	}
}

// Missing workspace env backend → single workspace-level issue, not
// duplicated per project.
func TestBuildOverview_EnvWorkspaceLevelOnly(t *testing.T) {
	withIsolatedOverviewProfiles(t)
	tmp := t.TempDir()
	m := &Manifest{
		Version:   ManifestVersion,
		Workspace: &ManifestWorkspace{ID: "demo", Name: "demo"},
		Projects: []ManifestProject{
			{Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
				Domains: &ProjectDomains{
					Dev: &ProjectDevOverride{Command: "pnpm dev"},
				},
			},
		},
	}
	if err := WriteManifest(tmp, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	ov, err := BuildOverview(tmp)
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	if len(ov.Issues) != 1 || ov.Issues[0].Domain != IssueDomainEnv {
		t.Fatalf("workspace issues = %+v; want one env issue", ov.Issues)
	}
	for _, iss := range ov.Projects[0].Issues {
		if iss.Domain == IssueDomainEnv {
			t.Errorf("env reported as per-project issue too: %+v", iss)
		}
	}
}

// packages projects must not contribute container/deploy noise.
func TestBuildOverview_PackagesSkipDomainChecks(t *testing.T) {
	withIsolatedOverviewProfiles(t)
	tmp := t.TempDir()
	m := &Manifest{
		Version:   ManifestVersion,
		Workspace: &ManifestWorkspace{ID: "demo", Name: "demo"},
		Domains: &WorkspaceDomains{
			Env: &BackendRef{Kind: EnvBackendDotenv},
		},
		Projects: []ManifestProject{
			{Name: "utils", RelativeDir: "packages/utils", TemplateID: "ts-lib", Toolchain: "node"},
		},
	}
	if err := WriteManifest(tmp, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	ov, err := BuildOverview(tmp)
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	p := ov.Projects[0]
	if p.Kind != ProjectKindPackage {
		t.Errorf("kind = %q; want %q", p.Kind, ProjectKindPackage)
	}
	if len(p.Issues) != 0 {
		t.Errorf("package project flagged with issues: %+v", p.Issues)
	}
}

func TestBuildOverview_SelectedBackendMissingCredentials(t *testing.T) {
	withIsolatedOverviewProfiles(t)
	tmp := t.TempDir()
	m := &Manifest{
		Version:   ManifestVersion,
		Workspace: &ManifestWorkspace{ID: "demo", Name: "demo"},
		Domains: &WorkspaceDomains{
			Env: &BackendRef{Kind: EnvBackendInfisical},
		},
		Projects: []ManifestProject{
			{Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node"},
		},
	}
	if err := WriteManifest(tmp, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	if _, err := profile.Upsert(profile.DomainEnv, EnvBackendInfisical, "empty", profile.Profile{
		Backend:   EnvBackendInfisical,
		Infisical: &profile.InfisicalProfile{SiteURL: "https://app.infisical.com"},
	}, true); err != nil {
		t.Fatalf("upsert infisical profile: %v", err)
	}

	ov, err := BuildOverview(tmp)
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	if len(ov.Issues) != 1 {
		t.Fatalf("workspace issues = %+v; want one credential issue", ov.Issues)
	}
	iss := ov.Issues[0]
	if iss.Domain != IssueDomainEnv || iss.Reason != IssueReasonProfile || iss.Backend != EnvBackendInfisical {
		t.Fatalf("issue = %+v; want env profile issue for infisical", iss)
	}
	if iss.Section != "env/infisical" || iss.Profile != "empty" {
		t.Fatalf("issue section/profile = %q/%q", iss.Section, iss.Profile)
	}
}

func TestBuildOverviewResolvesProfileForSelectedEnvironmentWithoutWritingManifest(t *testing.T) {
	withIsolatedOverviewProfiles(t)
	root := t.TempDir()
	manifest := &Manifest{
		Version:      ManifestVersion,
		Workspace:    &ManifestWorkspace{ID: "demo", Name: "demo"},
		Environments: &Environments{Names: []string{"dev", "staging", "prod"}, Default: "dev"},
		Domains: &WorkspaceDomains{
			Env: &BackendRef{Kind: EnvBackendInfisical},
		},
		Projects: []ManifestProject{{
			Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
		}},
	}
	if err := WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Upsert(profile.DomainEnv, EnvBackendInfisical, "broken-default", profile.Profile{
		Backend: EnvBackendInfisical,
		Infisical: &profile.InfisicalProfile{
			SiteURL: "https://app.infisical.com",
		},
	}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Upsert(profile.DomainEnv, EnvBackendInfisical, "production", profile.Profile{
		Backend: EnvBackendInfisical,
		Infisical: &profile.InfisicalProfile{
			SiteURL: "https://app.infisical.com",
			Credentials: &profile.InfisicalCredentials{
				ClientID: "client", ClientSecret: "secret",
			},
		},
	}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Upsert(profile.DomainEnv, EnvBackendInfisical, "legacy-preview", profile.Profile{
		Backend: EnvBackendInfisical,
		Infisical: &profile.InfisicalProfile{
			SiteURL: "https://app.infisical.com",
			Credentials: &profile.InfisicalCredentials{
				ClientID: "preview-client", ClientSecret: "preview-secret",
			},
		},
	}, false); err != nil {
		t.Fatal(err)
	}
	if err := profile.BindEnvironmentProfile(
		"demo", "demo", root, "", "prod",
		profile.DomainEnv, EnvBackendInfisical, "production",
	); err != nil {
		t.Fatal(err)
	}
	if err := profile.BindEnvironmentProfile(
		"demo", "demo", root, "", "staging",
		profile.DomainEnv, EnvBackendInfisical, "legacy-preview",
	); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, ManifestFilename)
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	production, err := BuildOverview(root, "prod")
	if err != nil {
		t.Fatal(err)
	}
	if production.Environment != "prod" || len(production.Issues) != 0 {
		t.Fatalf("prod overview = %#v", production)
	}
	preview, err := BuildOverview(root, "preview")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Environment != "preview" || len(preview.Issues) != 0 {
		t.Fatalf("preview overview = %#v", preview)
	}
	development, err := BuildOverview(root, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(development.Issues) != 1 || development.Issues[0].Profile != "broken-default" {
		t.Fatalf("dev overview = %#v", development)
	}
	after, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("BuildOverview changed one.manifest.json")
	}
}

func TestBuildOverview_RejectsBadManifest(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, ManifestFilename), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := BuildOverview(tmp)
	if err == nil {
		t.Fatal("BuildOverview accepted bogus manifest")
	}
}
