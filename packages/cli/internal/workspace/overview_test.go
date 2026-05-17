package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
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
	deployCfg, _ := json.Marshal(map[string]any{"projectId": "x"})
	m := &Manifest{
		Version:      ManifestVersion,
		Workspace:    &ManifestWorkspace{ID: "demo", Name: "demo"},
		Environments: &Environments{Names: []string{"dev", "prod"}, Default: "dev"},
		Domains: &WorkspaceDomains{
			Env:       &BackendRef{Kind: EnvBackendDotenv},
			Deploy:    &BackendRef{Kind: DeployBackendVercel},
			Container: &BackendRef{Kind: ContainerBackendDocker},
		},
		Projects: []ManifestProject{
			{
				Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
				Domains: &ProjectDomains{
					Deploy:    &ProjectDeployBackend{Kind: DeployBackendVercel, Config: deployCfg},
					Container: &ProjectContainerOverride{Image: "web:latest"},
					Dev:       &ProjectDevOverride{Command: "pnpm dev"},
				},
			},
		},
	}
	if err := WriteManifest(tmp, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	if _, err := profile.Upsert(profile.DomainDeploy, DeployBackendVercel, "prod", profile.Profile{
		Backend: DeployBackendVercel,
		Vercel:  &profile.VercelProfile{Credentials: &profile.VercelCredentials{APIToken: "token"}},
	}, true); err != nil {
		t.Fatalf("upsert vercel profile: %v", err)
	}
	if _, err := profile.Upsert(profile.DomainContainer, ContainerBackendDocker, "prod", profile.Profile{
		Backend: ContainerBackendDocker,
		Container: &profile.ContainerProfile{
			Registry:    "registry.example.com",
			Credentials: &profile.ContainerCredentials{Username: "user", Password: "pass"},
		},
	}, true); err != nil {
		t.Fatalf("upsert container profile: %v", err)
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
	if ov.Projects[0].Domains["deploy"] != DeployBackendVercel {
		t.Errorf("project deploy domain = %q", ov.Projects[0].Domains["deploy"])
	}
}

// apps project with no deploy override and no workspace default → one
// deploy issue. Container is only required once the effective deploy target
// is kustomize.
func TestBuildOverview_AppMissingDeployOnly(t *testing.T) {
	withIsolatedOverviewProfiles(t)
	tmp := t.TempDir()
	m := &Manifest{
		Version:   ManifestVersion,
		Workspace: &ManifestWorkspace{ID: "demo", Name: "demo"},
		Domains: &WorkspaceDomains{
			Env: &BackendRef{Kind: EnvBackendDotenv},
		},
		Projects: []ManifestProject{
			{Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node"},
		},
	}
	if err := WriteManifest(tmp, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	ov, err := BuildOverview(tmp)
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	if len(ov.Issues) != 0 {
		t.Errorf("workspace issues = %+v; want none (env is set)", ov.Issues)
	}
	if got := len(ov.Projects[0].Issues); got != 1 {
		t.Fatalf("project issues = %d; want 1 (deploy)", got)
	}
	domains := map[string]bool{}
	for _, iss := range ov.Projects[0].Issues {
		domains[iss.Domain] = true
		if iss.Severity != IssueSeverityMissing {
			t.Errorf("severity = %q", iss.Severity)
		}
	}
	if !domains[IssueDomainDeploy] {
		t.Errorf("missing deploy issue")
	}
	if domains[IssueDomainContainer] {
		t.Errorf("container should not be required before kustomize deploy is selected")
	}
}

func TestBuildOverview_CloudflareDeployDoesNotRequireContainer(t *testing.T) {
	withIsolatedOverviewProfiles(t)
	tmp := t.TempDir()
	m := &Manifest{
		Version:   ManifestVersion,
		Workspace: &ManifestWorkspace{ID: "demo", Name: "demo"},
		Domains: &WorkspaceDomains{
			Env:       &BackendRef{Kind: EnvBackendDotenv},
			Container: &BackendRef{Kind: "dockerhub"},
		},
		Projects: []ManifestProject{
			{Name: "site", RelativeDir: "apps/site", TemplateID: "astro-site", Toolchain: "node",
				Domains: &ProjectDomains{
					Deploy: &ProjectDeployBackend{Kind: DeployBackendCloudflare},
					Dev:    &ProjectDevOverride{Command: "pnpm dev"},
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
	p := ov.Projects[0]
	if got := p.Domains[IssueDomainContainer]; got != "" {
		t.Fatalf("cloudflare project container domain = %q, want empty", got)
	}
	for _, iss := range p.Issues {
		if iss.Domain == IssueDomainContainer {
			t.Fatalf("cloudflare project should not require container: %+v", p.Issues)
		}
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
					Deploy:    &ProjectDeployBackend{Kind: DeployBackendVercel},
					Container: &ProjectContainerOverride{Image: "x"},
					Dev:       &ProjectDevOverride{Command: "pnpm dev"},
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

// Workspace-level container/deploy defaults suppress per-project misses
// (project inherits the workspace default).
func TestBuildOverview_WorkspaceDefaultsSuppressProjectMisses(t *testing.T) {
	withIsolatedOverviewProfiles(t)
	tmp := t.TempDir()
	m := &Manifest{
		Version:   ManifestVersion,
		Workspace: &ManifestWorkspace{ID: "demo", Name: "demo"},
		Domains: &WorkspaceDomains{
			Env:       &BackendRef{Kind: EnvBackendDotenv},
			Deploy:    &BackendRef{Kind: DeployBackendKustomize},
			Container: &BackendRef{Kind: ContainerBackendDocker},
		},
		Projects: []ManifestProject{
			{Name: "api", RelativeDir: "services/api", TemplateID: "go-api", Toolchain: "go",
				Domains: &ProjectDomains{Dev: &ProjectDevOverride{Command: "go run ."}},
			},
		},
	}
	if err := WriteManifest(tmp, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	if _, err := profile.Upsert(profile.DomainContainer, ContainerBackendDocker, "prod", profile.Profile{
		Backend: ContainerBackendDocker,
		Container: &profile.ContainerProfile{
			Registry:    "registry.example.com",
			Credentials: &profile.ContainerCredentials{Username: "user", Password: "pass"},
		},
	}, true); err != nil {
		t.Fatalf("upsert container profile: %v", err)
	}
	ov, err := BuildOverview(tmp)
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	if len(ov.Issues) != 0 {
		t.Errorf("workspace issues = %+v; want none", ov.Issues)
	}
	if len(ov.Projects[0].Issues) != 0 {
		t.Errorf("project issues = %+v; want none (workspace defaults cover everything)",
			ov.Projects[0].Issues)
	}
	if ov.Projects[0].Kind != ProjectKindService {
		t.Errorf("kind = %q; want %q", ov.Projects[0].Kind, ProjectKindService)
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
