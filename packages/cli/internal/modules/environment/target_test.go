package environment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestResolveInfisicalUsesEnvironmentProjectProfileBinding(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version:   workspace.ManifestVersion,
		Workspace: &workspace.ManifestWorkspace{ID: "workspace-id", Name: "demo"},
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendInfisical},
		},
		Projects: []workspace.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", Toolchain: "node",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Upsert(profile.DomainEnv, workspace.EnvBackendInfisical, "preview-web", profile.Profile{
		Backend: workspace.EnvBackendInfisical,
		Infisical: &profile.InfisicalProfile{
			SiteURL: "https://example.infisical.test",
			Credentials: &profile.InfisicalCredentials{
				ClientID: "preview-client", ClientSecret: "preview-secret",
			},
		},
	}, false); err != nil {
		t.Fatal(err)
	}
	if err := profile.BindEnvironmentProfile(
		"workspace-id", "demo", root, "web", "preview",
		profile.DomainEnv, workspace.EnvBackendInfisical, "preview-web",
	); err != nil {
		t.Fatal(err)
	}
	activeWorkspace := executionWorkspaceAt(t, root, root)
	config, credentials, err := newTestService(t).resolveInfisical(
		activeWorkspace, "", "preview", "web",
	)
	if err != nil {
		t.Fatal(err)
	}
	if config == nil || config.SiteURL != "https://example.infisical.test" ||
		credentials == nil || credentials.ClientID != "preview-client" ||
		credentials.ClientSecret != "preview-secret" {
		t.Fatalf("resolved config=%#v credentials=%#v", config, credentials)
	}
}

func TestEnsureInfisicalBoundPassesContextualProfileToInit(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version:      workspace.ManifestVersion,
		Workspace:    &workspace.ManifestWorkspace{ID: "workspace-id", Name: "demo"},
		Environments: &workspace.Environments{Names: []string{"dev", "staging", "prod"}, Default: "dev"},
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendInfisical},
		},
		Projects: []workspace.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", Toolchain: "node",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Upsert(profile.DomainEnv, workspace.EnvBackendInfisical, "preview-web", profile.Profile{
		Backend: workspace.EnvBackendInfisical,
		Infisical: &profile.InfisicalProfile{
			SiteURL: "https://example.infisical.test",
			Credentials: &profile.InfisicalCredentials{
				ClientID: "preview-client", ClientSecret: "preview-secret",
			},
		},
	}, false); err != nil {
		t.Fatal(err)
	}
	if err := profile.BindEnvironmentProfile(
		"workspace-id", "demo", root, "web", "staging",
		profile.DomainEnv, workspace.EnvBackendInfisical, "preview-web",
	); err != nil {
		t.Fatal(err)
	}

	service := newTestService(t)
	var initRoot string
	var initInput infisical.InitInput
	service.initInfisical = func(
		_ context.Context, projectRoot string, input infisical.InitInput,
	) (*infisical.InitResult, error) {
		initRoot, initInput = projectRoot, input
		return &infisical.InitResult{}, nil
	}
	activeWorkspace := executionWorkspaceAt(t, root, root)
	if err := service.ensureInfisicalBound(
		context.Background(), activeWorkspace, "", "preview", "web",
	); err != nil {
		t.Fatal(err)
	}
	if initRoot != root || initInput.ProfileName != "preview-web" {
		t.Fatalf("Init(%q, %#v); want contextual profile preview-web", initRoot, initInput)
	}
}

func TestResolveInfisicalFolderPath(t *testing.T) {
	root := t.TempDir()
	if err := workspace.SetManifestWorkspaceIdentity(root, "id", "demo"); err != nil {
		t.Fatal(err)
	}
	for _, input := range []workspace.ManifestProjectInput{
		{Name: "api", RelativeDir: "services/api", TemplateID: "go-api", Toolchain: "go"},
		{Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node"},
	} {
		if err := workspace.UpsertManifestProject(root, input); err != nil {
			t.Fatal(err)
		}
	}
	config := &infisical.WorkspaceConfig{ProjectID: "x", RootPath: "/"}
	previous, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(previous) })
	if err := os.Chdir(filepath.Dir(root)); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t)
	for _, test := range []struct {
		selector string
		want     string
	}{
		{want: "/"},
		{selector: "api", want: "/services/api"},
		{selector: "apps/web", want: "/apps/web"},
		{selector: "./apps/web", want: "/apps/web"},
		{selector: "/shared", want: "/shared"},
	} {
		activeWorkspace := resolveTestWorkspace(t, root)
		got, err := service.resolveInfisicalFolderPath(activeWorkspace, config, test.selector)
		if err != nil {
			t.Fatalf("selector %q: %v", test.selector, err)
		}
		if got != test.want {
			t.Fatalf("selector %q: path = %q, want %q", test.selector, got, test.want)
		}
	}
	activeWorkspace := resolveTestWorkspace(t, root)
	if _, err := service.resolveInfisicalFolderPath(activeWorkspace, config, "missing"); errorCode(err) != "SUBPROJECT_NOT_FOUND" {
		t.Fatalf("unknown selector error = %v", err)
	}
	projectDir := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}
	activeWorkspace = resolveTestWorkspace(t, root)
	if got, err := service.resolveInfisicalFolderPath(activeWorkspace, config, ""); err != nil || got != "/services/api" {
		t.Fatalf("cwd path = %q, err = %v", got, err)
	}
}

func TestResolveSetTarget(t *testing.T) {
	root := t.TempDir()
	if err := workspace.SetManifestWorkspaceIdentity(root, "id", "demo"); err != nil {
		t.Fatal(err)
	}
	if err := workspace.UpsertManifestProject(root, workspace.ManifestProjectInput{
		Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
	}); err != nil {
		t.Fatal(err)
	}
	activeWorkspace := executionWorkspaceAt(t, root, root)
	project, target := resolveSetTarget(activeWorkspace, "web")
	if project == nil || project.Name != "web" || target != "apps/web" {
		t.Fatalf("declared target = (%#v, %q)", project, target)
	}
	project, target = resolveSetTarget(activeWorkspace, " shared ")
	if project != nil || target != "shared" {
		t.Fatalf("raw target = (%#v, %q)", project, target)
	}
}

func resolveTestWorkspace(t *testing.T, root string) execution.Workspace {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return executionWorkspaceAt(t, root, cwd)
}

func executionWorkspaceAt(t *testing.T, root, cwd string) execution.Workspace {
	t.Helper()
	scope := execution.NewScope(context.Background(), cwd).Derive(execution.ScopePatch{WorkspaceRoot: root})
	activeWorkspace, err := execution.ResolveWorkspaceScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	return activeWorkspace
}

func errorCode(err error) string {
	if coded, ok := err.(interface{ ErrorCode() string }); ok {
		return coded.ErrorCode()
	}
	return ""
}
