package workspace

import (
	"os"
	"path/filepath"
	"testing"
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
		Version:   ManifestVersion,
		Workspace: &ManifestWorkspace{ID: "demo", Name: "demo"},

		Env: &EnvironmentConfig{ProjectID: "remote", Environments: []string{"dev", "prod"}},

		Projects: []ManifestProject{
			{
				Name: "web", RelativeDir: "apps/web", Toolchain: "node",
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
	if ov.Workspace.Domains["env"] != EnvBackendInfisical {
		t.Errorf("workspace env domain = %q", ov.Workspace.Domains["env"])
	}
	if len(ov.Projects[0].Domains) != 1 || ov.Projects[0].Domains["env"] != EnvBackendInfisical {
		t.Errorf("project domains = %#v; want only infisical", ov.Projects[0].Domains)
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
			{Name: "web", RelativeDir: "apps/web", Toolchain: "node"},
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

		Env: &EnvironmentConfig{ProjectID: "remote", Environments: []string{"dev", "staging", "prod"}},

		Projects: []ManifestProject{
			{Name: "utils", RelativeDir: "packages/utils", Toolchain: "node"},
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
