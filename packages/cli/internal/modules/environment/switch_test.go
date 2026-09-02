package environment

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestSwitchInfisicalSyncResolvesProfileForEveryProjectEnvironmentTuple(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)
	root := t.TempDir()
	manifest := &workspace.Manifest{
		Version:      workspace.ManifestVersion,
		Workspace:    &workspace.ManifestWorkspace{ID: "workspace-id", Name: "demo"},
		Environments: &workspace.Environments{Names: []string{"dev", "preview"}, Default: "dev"},
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendDotenv},
		},
		Projects: []workspace.ManifestProject{
			{Name: "web", RelativeDir: "apps/web", Toolchain: "node"},
			{Name: "api", RelativeDir: "services/api", Toolchain: "go"},
		},
	}
	if err := workspace.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	for relativePath, content := range map[string]string{
		"apps/web/.env.dev":         "WEB_TOKEN=web-value\n",
		"services/api/.env.preview": "API_TOKEN=api-value\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	profiles := []struct {
		project, environment, name, siteURL, clientID string
	}{
		{
			project: "web", environment: "dev", name: "web-development",
			siteURL: "https://web.infisical.test", clientID: "web-client",
		},
		{
			project: "api", environment: "preview", name: "api-preview",
			siteURL: "https://api.infisical.test", clientID: "api-client",
		},
	}
	for _, entry := range profiles {
		if _, err := profile.Upsert(
			profile.DomainEnv,
			workspace.EnvBackendInfisical,
			entry.name,
			profile.Profile{
				Backend: workspace.EnvBackendInfisical,
				Infisical: &profile.InfisicalProfile{
					SiteURL: entry.siteURL,
					Credentials: &profile.InfisicalCredentials{
						ClientID: entry.clientID, ClientSecret: entry.clientID + "-secret",
					},
				},
			},
			false,
		); err != nil {
			t.Fatal(err)
		}
		if err := profile.BindEnvironmentProfile(
			"workspace-id", "demo", root, entry.project, entry.environment,
			profile.DomainEnv, workspace.EnvBackendInfisical, entry.name,
		); err != nil {
			t.Fatal(err)
		}
	}

	service := newTestService(t)
	var initializedWith string
	service.initInfisical = func(
		_ context.Context, _ string, input infisical.InitInput,
	) (*infisical.InitResult, error) {
		initializedWith = input.ProfileName
		return &infisical.InitResult{}, nil
	}
	type syncCall struct {
		environment, path, key, profileName, siteURL, clientID string
	}
	var calls []syncCall
	service.setInfisical = func(
		_ context.Context, _ string, input infisical.SetInput,
	) (*infisical.SetResult, error) {
		call := syncCall{
			environment: input.Env, path: input.Path, key: input.Key,
		}
		if input.Cfg != nil {
			call.siteURL = input.Cfg.SiteURL
			call.profileName = input.Cfg.ProfileName
		}
		if input.Creds != nil {
			call.clientID = input.Creds.ClientID
		}
		calls = append(calls, call)
		return &infisical.SetResult{}, nil
	}

	plan, err := service.PlanSwitch(
		execution.NewScope(context.Background(), root), workspace.EnvBackendInfisical,
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Entries() != 2 {
		t.Fatalf("plan entries = %d; want 2", plan.Entries())
	}
	result, err := service.Switch(context.Background(), plan, SwitchOptions{Sync: true})
	if err != nil {
		t.Fatal(err)
	}
	if initializedWith != "web-development" {
		t.Fatalf("Init profile = %q; want first tuple's contextual profile", initializedWith)
	}
	want := []syncCall{
		{
			environment: "dev", path: "/apps/web", key: "WEB_TOKEN",
			profileName: "web-development", siteURL: "https://web.infisical.test", clientID: "web-client",
		},
		{
			environment: "preview", path: "/services/api", key: "API_TOKEN",
			profileName: "api-preview", siteURL: "https://api.infisical.test", clientID: "api-client",
		},
	}
	if len(calls) != len(want) {
		t.Fatalf("sync calls = %#v", calls)
	}
	for index := range want {
		if calls[index] != want[index] {
			t.Fatalf("sync call[%d] = %#v; want %#v", index, calls[index], want[index])
		}
	}
	if result.Synced != 2 || result.SkippedSync {
		t.Fatalf("switch result = %#v", result)
	}
	updated, err := workspace.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.EnvBackend(updated) != workspace.EnvBackendInfisical {
		t.Fatalf("env backend = %q", workspace.EnvBackend(updated))
	}
}

func TestSwitchInfisicalWithoutSyncInitializesAndPreservesBinding(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)
	root := t.TempDir()
	manifest := &workspace.Manifest{
		Version:   workspace.ManifestVersion,
		Workspace: &workspace.ManifestWorkspace{ID: "workspace-id", Name: "demo"},
		Environments: &workspace.Environments{
			Names: []string{"dev", "preview"}, Default: "dev",
		},
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendDotenv},
		},
	}
	if err := workspace.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Upsert(
		profile.DomainEnv,
		workspace.EnvBackendInfisical,
		"preview-work",
		profile.Profile{
			Backend: workspace.EnvBackendInfisical,
			Infisical: &profile.InfisicalProfile{
				SiteURL: "https://infisical.test",
				Credentials: &profile.InfisicalCredentials{
					ClientID: "client", ClientSecret: "secret",
				},
			},
		},
		false,
	); err != nil {
		t.Fatal(err)
	}
	if err := profile.BindEnvironmentProfile(
		"workspace-id", "demo", root, "", "preview",
		profile.DomainEnv, workspace.EnvBackendInfisical, "preview-work",
	); err != nil {
		t.Fatal(err)
	}

	service := newTestService(t)
	var initializedWith string
	service.initInfisical = func(
		_ context.Context, projectRoot string, input infisical.InitInput,
	) (*infisical.InitResult, error) {
		initializedWith = input.ProfileName
		config, err := json.Marshal(map[string]any{
			"projectId":    "infisical-project-id",
			"defaultEnv":   "dev",
			"environments": []string{"dev", "preview"},
		})
		if err != nil {
			return nil, err
		}
		if err := workspace.InitWorkspaceEnv(projectRoot, workspace.EnvInit{
			Kind: workspace.EnvBackendInfisical, ConfigJSON: config,
		}); err != nil {
			return nil, err
		}
		return &infisical.InitResult{ProjectID: "infisical-project-id"}, nil
	}

	plan, err := service.PlanSwitch(
		execution.NewScope(context.Background(), root), workspace.EnvBackendInfisical,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Switch(context.Background(), plan, SwitchOptions{
		Environment: "preview",
	})
	if err != nil {
		t.Fatal(err)
	}
	if initializedWith != "preview-work" {
		t.Fatalf("Init profile = %q; want preview Workspace profile", initializedWith)
	}
	if !result.SkippedSync || result.Synced != 0 {
		t.Fatalf("switch result = %#v", result)
	}
	updated, err := workspace.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.EnvBackend(updated) != workspace.EnvBackendInfisical {
		t.Fatalf("env backend = %q", workspace.EnvBackend(updated))
	}
	var config map[string]any
	if err := json.Unmarshal(updated.Domains.Env.Config, &config); err != nil {
		t.Fatal(err)
	}
	if config["projectId"] != "infisical-project-id" {
		t.Fatalf("Infisical binding was overwritten: %#v", config)
	}
}
