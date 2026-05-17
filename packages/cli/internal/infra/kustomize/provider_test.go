package kustomize

import (
	"encoding/json"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func TestNormalizeVersionTag(t *testing.T) {
	tests := map[string]string{
		"0.1.0":  "v0.1.0",
		"v1.2.3": "v1.2.3",
		"V2.0.1": "v2.0.1",
		"custom": "custom",
	}
	for in, want := range tests {
		if got := normalizeVersionTag(in); got != want {
			t.Errorf("normalizeVersionTag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCurrentImageSemverTag(t *testing.T) {
	m := manifestWithImage("api", "ghcr.io/acme/api:v0.3.4")
	got, ok := currentImageSemverTag(m, "api")
	if !ok {
		t.Fatal("expected semver image tag")
	}
	if got.major != 0 || got.minor != 3 || got.patch != 4 {
		t.Fatalf("tag = %#v, want v0.3.4", got)
	}
}

func TestResolveBuildPlatformForDeployCanUseCachedPlatformForDryRun(t *testing.T) {
	platformCfg, _ := json.Marshal(map[string]string{"platform": "linux/amd64"})
	m := &workspace.Manifest{
		Domains: &workspace.WorkspaceDomains{
			Container: &workspace.BackendRef{Kind: "docker", Config: platformCfg},
		},
	}
	got, err := resolveBuildPlatformForDeploy(m, Endpoint{
		KubeconfigPath:    "/does/not/exist",
		KubeconfigContext: "missing",
	}, true)
	if err != nil {
		t.Fatalf("resolveBuildPlatformForDeploy: %v", err)
	}
	if got != "linux/amd64" {
		t.Fatalf("platform = %q, want linux/amd64", got)
	}
}

func manifestWithImage(name, image string) *workspace.Manifest {
	return &workspace.Manifest{
		Projects: []workspace.ManifestProject{
			{
				Name: name,
				Domains: &workspace.ProjectDomains{
					Container: &workspace.ProjectContainerOverride{Image: image},
				},
			},
		},
	}
}

// envForProject returns the per-project Kustomize Env pin, or empty when
// the manifest does not declare one.
func TestEnvForProject(t *testing.T) {
	cases := []struct {
		name string
		m    *workspace.Manifest
		want string
	}{
		{name: "nil manifest", m: nil, want: ""},
		{name: "missing project", m: &workspace.Manifest{Projects: []workspace.ManifestProject{{Name: "other"}}}, want: ""},
		{
			name: "project without deploy section",
			m: &workspace.Manifest{Projects: []workspace.ManifestProject{
				{Name: "api"},
			}},
			want: "",
		},
		{
			name: "project without kustomize config",
			m: &workspace.Manifest{Projects: []workspace.ManifestProject{
				{Name: "api", Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{Kind: "kustomize"},
				}},
			}},
			want: "",
		},
		{
			name: "explicit env=staging returns staging",
			m:    manifestWithKustomizeEnv(t, "staging"),
			want: "staging",
		},
		{
			name: "env=prod returns prod (trimmed)",
			m:    manifestWithKustomizeEnv(t, "  prod  "),
			want: "prod",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := envForProject(tc.m, "api"); got != tc.want {
				t.Fatalf("envForProject = %q, want %q", got, tc.want)
			}
		})
	}
}

// endpointFromInput must let a per-project Kustomize Env override the
// workspace-level kustomizationPath, so a `--env staging` flag (which
// applyEnvOverride writes into the deploy config blob) selects the
// staging overlay even when the workspace manifest pinned
// kustomizationPath to prod.
func TestEndpointFromInputEnvOverridesWorkspaceKustomizationPath(t *testing.T) {
	wsCfg, _ := json.Marshal(map[string]string{"kustomizationPath": "kustomize/overlays/prod"})
	m := manifestWithKustomizeEnv(t, "staging")
	m.Domains = &workspace.WorkspaceDomains{
		Deploy: &workspace.BackendRef{Kind: "kustomize", Config: wsCfg},
	}
	ep := endpointFromInput(deploy.ApplyInput{
		Project:  workspace.Project{Name: "api", RelativeDir: "services/api"},
		Manifest: m,
	})
	if ep.KustomizationPath != "kustomize/overlays/staging" {
		t.Fatalf("KustomizationPath = %q, want kustomize/overlays/staging", ep.KustomizationPath)
	}
}

// When the per-project deploy config carries no env, endpointFromInput
// falls back to the workspace-level kustomizationPath.
func TestEndpointFromInputFallsBackToWorkspaceKustomizationPath(t *testing.T) {
	wsCfg, _ := json.Marshal(map[string]string{"kustomizationPath": "infra/overlays/canary"})
	m := &workspace.Manifest{
		Domains: &workspace.WorkspaceDomains{
			Deploy: &workspace.BackendRef{Kind: "kustomize", Config: wsCfg},
		},
		Projects: []workspace.ManifestProject{
			{Name: "api", Domains: &workspace.ProjectDomains{
				Deploy: &workspace.ProjectDeployBackend{Kind: "kustomize"},
			}},
		},
	}
	ep := endpointFromInput(deploy.ApplyInput{
		Project:  workspace.Project{Name: "api", RelativeDir: "services/api"},
		Manifest: m,
	})
	if ep.KustomizationPath != "infra/overlays/canary" {
		t.Fatalf("KustomizationPath = %q, want infra/overlays/canary", ep.KustomizationPath)
	}
}

func manifestWithKustomizeEnv(t *testing.T, env string) *workspace.Manifest {
	t.Helper()
	p := workspace.ManifestProject{Name: "api"}
	if err := EncodeProjectConfig(&p, &ProjectConfig{Env: env}); err != nil {
		t.Fatalf("EncodeProjectConfig: %v", err)
	}
	return &workspace.Manifest{Projects: []workspace.ManifestProject{p}}
}
