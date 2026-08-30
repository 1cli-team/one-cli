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

func TestBulkPullResolvesEnvironmentProfilePerProjectAndAggregatesResults(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)
	root := t.TempDir()
	infisicalConfig, err := json.Marshal(map[string]string{"projectId": "remote-project"})
	if err != nil {
		t.Fatal(err)
	}
	manifest := &workspace.Manifest{
		Version:      workspace.ManifestVersion,
		Workspace:    &workspace.ManifestWorkspace{ID: "workspace-id", Name: "demo"},
		Environments: &workspace.Environments{Names: []string{"dev", "preview", "prod"}, Default: "dev"},
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendInfisical, Config: infisicalConfig},
		},
		Projects: []workspace.ManifestProject{
			{Name: "web", RelativeDir: "apps/web", Toolchain: "node"},
			{Name: "api", RelativeDir: "services/api", Toolchain: "go"},
		},
	}
	if err := workspace.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	for _, relativeDir := range []string{"apps/web", "services/api"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(relativeDir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	manifestPath := filepath.Join(root, workspace.ManifestFilename)
	manifestBefore, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	profiles := []struct {
		project, name, siteURL, clientID string
	}{
		{project: "web", name: "web-preview", siteURL: "https://web.infisical.test", clientID: "web-client"},
		{project: "api", name: "api-preview", siteURL: "https://api.infisical.test", clientID: "api-client"},
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
			"workspace-id", "demo", root, entry.project, "preview",
			profile.DomainEnv, workspace.EnvBackendInfisical, entry.name,
		); err != nil {
			t.Fatal(err)
		}
	}

	type pullCall struct {
		project, environment, profileName, siteURL, clientID string
	}
	var calls []pullCall
	service := newTestService(t)
	service.pullInfisical = func(
		_ context.Context, projectRoot string, input infisical.PullInput,
	) (*infisical.PullResult, error) {
		call := pullCall{project: input.Project, environment: input.Env}
		if input.Cfg != nil {
			call.profileName = input.Cfg.ProfileName
			call.siteURL = input.Cfg.SiteURL
		}
		if input.Creds != nil {
			call.clientID = input.Creds.ClientID
		}
		calls = append(calls, call)
		status := "written"
		written, skipped := 1, 0
		if input.Project == "api" {
			status, written, skipped = "unchanged", 0, 1
		}
		return &infisical.PullResult{
			Schema: "one-cli/env-pull/v1", Env: input.Env, DryRun: input.DryRun,
			WrittenCount: written, SkippedCount: skipped,
			PerSubproject: []infisical.PullEntry{{
				Name: input.Project, RelativeDir: input.Project,
				EnvFilePath: filepath.Join(projectRoot, input.Project, ".env"), Status: status,
			}},
		}, nil
	}

	result, err := service.Pull(context.Background(), PullInput{
		Scope:       execution.NewScope(context.Background(), root),
		Environment: "preview", DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := []pullCall{
		{
			project: "web", environment: "preview", profileName: "web-preview",
			siteURL: "https://web.infisical.test", clientID: "web-client",
		},
		{
			project: "api", environment: "preview", profileName: "api-preview",
			siteURL: "https://api.infisical.test", clientID: "api-client",
		},
	}
	if len(calls) != len(wantCalls) {
		t.Fatalf("pull calls = %#v", calls)
	}
	for index := range wantCalls {
		if calls[index] != wantCalls[index] {
			t.Fatalf("pull call[%d] = %#v; want %#v", index, calls[index], wantCalls[index])
		}
	}
	if result.WrittenCount != 1 || result.SkippedCount != 1 ||
		len(result.PerSubproject) != 2 || result.Environment != "preview" || !result.DryRun {
		t.Fatalf("aggregated result = %#v", result)
	}
	afterBulk, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterBulk) != string(manifestBefore) {
		t.Fatal("bulk pull changed one.manifest.json")
	}

	calls = nil
	if _, err := service.Pull(context.Background(), PullInput{
		Scope:       execution.NewScope(context.Background(), root),
		Environment: "preview", Project: "api", DryRun: true,
	}); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != wantCalls[1] {
		t.Fatalf("single-project pull calls = %#v; want api only", calls)
	}
}
