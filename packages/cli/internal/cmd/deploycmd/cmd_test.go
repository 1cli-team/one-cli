package deploycmd

import (
	"encoding/json"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// rawConfig encodes a per-deploy-kind config as JSON so test fixtures can
// stuff it into ProjectDeployBackend.Config. Failure here is a test bug,
// not a production failure mode, so we t.Fatal.
func rawConfig(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("rawConfig: %v", err)
	}
	return raw
}

// projectDeployConfig is the universal env field shared by every per-kind
// deploy config — the only field this test file cares about.
type deployEnvCfg struct {
	Env string `json:"env,omitempty"`
}

func TestResolveDeployProfileFallsBackToBackendDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if _, err := profile.Upsert(profile.DomainDeploy, "aws-s3", "prod", profile.Profile{
		Backend: "aws-s3",
		S3: &profile.S3Profile{
			Endpoint: "http://127.0.0.1:9000",
			Credentials: &profile.S3Credentials{
				AccessKeyID:     "ak",
				AccessKeySecret: "sk",
			},
		},
	}, true); err != nil {
		t.Fatalf("add aws-s3 profile: %v", err)
	}
	if _, err := profile.Upsert(profile.DomainDeploy, "kustomize", "demo-k8s", profile.Profile{
		Backend: "kustomize",
		Kustomize: &profile.KustomizeProfile{
			KubeconfigContext: "prod",
		},
	}, true); err != nil {
		t.Fatalf("add kustomize profile: %v", err)
	}
	if err := workspace.WriteManifest(tmp, &workspace.Manifest{
		Workspace: &workspace.ManifestWorkspace{ID: "ws-demo", Name: "demo"},
		Domains: &workspace.WorkspaceDomains{
			Deploy: &workspace.BackendRef{Kind: "kustomize"},
		},
		Projects: []workspace.ManifestProject{
			{
				Name:        "web",
				RelativeDir: "apps/web",
				Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{Kind: workspace.DeployBackendAWSS3},
				},
			},
		},
	}); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	resolved, err := resolveDeployProfile(tmp, "", deployTarget{
		Project: workspace.Project{Name: "web", RelativeDir: "apps/web"},
		Backend: workspace.DeployBackendAWSS3,
	})
	if err != nil {
		t.Fatalf("resolveDeployProfile: %v", err)
	}
	if resolved.Name != "prod" || resolved.Source != "default" {
		t.Fatalf("resolved = %#v, want default aws-s3 profile prod", resolved)
	}
}

func TestResolveDeployProfileProjectBindingWins(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	for _, seed := range []struct {
		name           string
		endpoint       string
		defaultProfile bool
	}{
		{name: "prod", endpoint: "http://127.0.0.1:9000", defaultProfile: true},
		{name: "web-prod", endpoint: "http://127.0.0.1:9001"},
	} {
		if _, err := profile.Upsert(profile.DomainDeploy, "aws-s3", seed.name, profile.Profile{
			Backend: "aws-s3",
			S3: &profile.S3Profile{
				Endpoint: seed.endpoint,
				Credentials: &profile.S3Credentials{
					AccessKeyID:     "ak",
					AccessKeySecret: "sk",
				},
			},
		}, seed.defaultProfile); err != nil {
			t.Fatalf("add aws-s3 profile %s: %v", seed.name, err)
		}
	}
	if err := workspace.WriteManifest(tmp, &workspace.Manifest{
		Workspace: &workspace.ManifestWorkspace{ID: "ws-demo", Name: "demo"},
		Projects: []workspace.ManifestProject{
			{
				Name:        "web",
				RelativeDir: "apps/web",
				Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{Kind: workspace.DeployBackendAWSS3},
				},
			},
		},
	}); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := profile.BindWorkspaceProfile("ws-demo", "demo", tmp, "web", profile.DomainDeploy, workspace.DeployBackendAWSS3, "web-prod"); err != nil {
		t.Fatalf("bind project profile: %v", err)
	}

	resolved, err := resolveDeployProfile(tmp, "", deployTarget{
		Project: workspace.Project{Name: "web", RelativeDir: "apps/web"},
		Backend: workspace.DeployBackendAWSS3,
	})
	if err != nil {
		t.Fatalf("resolveDeployProfile: %v", err)
	}
	if resolved.Name != "web-prod" || resolved.Source != "workspace-project" {
		t.Fatalf("resolved = %#v, want project-bound web-prod", resolved)
	}
}

func TestResolveDeployProfileFallsBackToNilWhenNoMatchingBindingOrActiveProfile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if _, err := profile.Upsert(profile.DomainDeploy, "kustomize", "demo-k8s", profile.Profile{
		Backend: "kustomize",
		Kustomize: &profile.KustomizeProfile{
			KubeconfigContext: "prod",
		},
	}, true); err != nil {
		t.Fatalf("add kustomize profile: %v", err)
	}
	if err := workspace.WriteManifest(tmp, &workspace.Manifest{
		Workspace: &workspace.ManifestWorkspace{ID: "ws-demo", Name: "demo"},
		Domains: &workspace.WorkspaceDomains{
			Deploy: &workspace.BackendRef{Kind: "kustomize"},
		},
		Projects: []workspace.ManifestProject{
			{
				Name:        "web",
				RelativeDir: "apps/web",
				Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{Kind: workspace.DeployBackendAWSS3},
				},
			},
		},
	}); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := profile.BindWorkspaceProfile("ws-demo", "demo", tmp, "", profile.DomainDeploy, workspace.DeployBackendKustomize, "demo-k8s"); err != nil {
		t.Fatalf("bind workspace profile: %v", err)
	}

	resolved, err := resolveDeployProfile(tmp, "", deployTarget{
		Project: workspace.Project{Name: "web", RelativeDir: "apps/web"},
		Backend: workspace.DeployBackendAWSS3,
	})
	if err != nil {
		t.Fatalf("resolveDeployProfile: %v", err)
	}
	if resolved != nil {
		t.Fatalf("resolved = %#v, want nil when aws-s3 has no default profile", resolved)
	}
}

// projectEnvOf returns the env field embedded in
// projects[i].domains.deploy.config.
func projectEnvOf(t *testing.T, m *workspace.Manifest, name string) string {
	t.Helper()
	for _, p := range m.Projects {
		if p.Name != name || p.Domains == nil || p.Domains.Deploy == nil {
			continue
		}
		if len(p.Domains.Deploy.Config) == 0 {
			return ""
		}
		var cfg deployEnvCfg
		if err := json.Unmarshal(p.Domains.Deploy.Config, &cfg); err != nil {
			t.Fatalf("decode %s deploy config: %v", name, err)
		}
		return cfg.Env
	}
	return ""
}

// applyEnvOverride is a no-op when the flag is empty or the manifest is
// nil; otherwise it stamps the env field on every project's deploy config
// blob so envForProject downstream sees the override.
func TestApplyEnvOverrideStampsEveryProvider(t *testing.T) {
	m := &workspace.Manifest{
		Environments: &workspace.Environments{Names: []string{"dev", "staging", "prod"}},
		Projects: []workspace.ManifestProject{
			{
				Name: "web",
				Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{
						Kind:   workspace.DeployBackendVercel,
						Config: rawConfig(t, deployEnvCfg{Env: "prod"}),
					},
				},
			},
			{
				Name: "api",
				Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{
						Kind:   workspace.DeployBackendCloudflare,
						Config: rawConfig(t, deployEnvCfg{Env: "prod"}),
					},
				},
			},
			{
				Name: "static",
				Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{
						Kind:   workspace.DeployBackendEdgeOne,
						Config: rawConfig(t, deployEnvCfg{Env: "prod"}),
					},
				},
			},
			{
				Name: "backend",
				Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{
						Kind: workspace.DeployBackendKustomize,
					},
				},
			},
		},
	}
	if err := applyEnvOverride(m, "staging"); err != nil {
		t.Fatalf("applyEnvOverride: %v", err)
	}
	for _, name := range []string{"web", "api", "static", "backend"} {
		if got := projectEnvOf(t, m, name); got != "staging" {
			t.Errorf("%s env = %q, want staging", name, got)
		}
	}
}

func TestApplyEnvOverrideEmptyFlagIsNoop(t *testing.T) {
	m := &workspace.Manifest{
		Environments: &workspace.Environments{Names: []string{"dev", "staging", "prod"}},
		Projects: []workspace.ManifestProject{
			{
				Name: "web",
				Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{
						Kind:   workspace.DeployBackendVercel,
						Config: rawConfig(t, deployEnvCfg{Env: "prod"}),
					},
				},
			},
		},
	}
	if err := applyEnvOverride(m, ""); err != nil {
		t.Fatalf("applyEnvOverride: %v", err)
	}
	if got := projectEnvOf(t, m, "web"); got != "prod" {
		t.Errorf("vercel env mutated by empty flag: %q", got)
	}
}

func TestApplyEnvOverrideRejectsUnknownEnv(t *testing.T) {
	m := &workspace.Manifest{
		Environments: &workspace.Environments{Names: []string{"dev", "staging", "prod"}},
		Projects: []workspace.ManifestProject{
			{
				Name: "web",
				Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{
						Kind:   workspace.DeployBackendVercel,
						Config: rawConfig(t, deployEnvCfg{Env: "prod"}),
					},
				},
			},
		},
	}
	err := applyEnvOverride(m, "qa")
	if err == nil {
		t.Fatal("expected error for unknown env name")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "ENV_UNKNOWN_ENVIRONMENT" {
		t.Fatalf("error code = %v, want ENV_UNKNOWN_ENVIRONMENT", err)
	}
}

// Workspaces without an environments declaration accept any env value
// (matches the dotenv-only workspace flow that doesn't require declaring
// envs up front).
func TestApplyEnvOverrideAllowsAnyEnvWhenEnvironmentsUndeclared(t *testing.T) {
	m := &workspace.Manifest{
		Projects: []workspace.ManifestProject{
			{
				Name: "web",
				Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{Kind: workspace.DeployBackendVercel},
				},
			},
		},
	}
	if err := applyEnvOverride(m, "anything-goes"); err != nil {
		t.Fatalf("applyEnvOverride: %v", err)
	}
	if got := projectEnvOf(t, m, "web"); got != "anything-goes" {
		t.Errorf("vercel env = %q, want anything-goes", got)
	}
}

// validateProjectEnvs flags any per-project deploy env that the user has
// pinned in manifest but that is not registered in environments.names.
func TestValidateProjectEnvsRejectsUnknownPin(t *testing.T) {
	m := &workspace.Manifest{
		Environments: &workspace.Environments{Names: []string{"dev", "staging", "prod"}},
		Projects: []workspace.ManifestProject{
			{
				Name: "web",
				Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{
						Kind:   workspace.DeployBackendVercel,
						Config: rawConfig(t, deployEnvCfg{Env: "typo-env"}),
					},
				},
			},
		},
	}
	err := validateProjectEnvs(m)
	if err == nil {
		t.Fatal("expected error for unknown env pinned in manifest")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "ENV_UNKNOWN_ENVIRONMENT" {
		t.Fatalf("error code = %v, want ENV_UNKNOWN_ENVIRONMENT", err)
	}
}

func TestValidateProjectEnvsAcceptsKnownAndEmpty(t *testing.T) {
	m := &workspace.Manifest{
		Environments: &workspace.Environments{Names: []string{"dev", "staging", "prod"}},
		Projects: []workspace.ManifestProject{
			{Name: "web", Domains: &workspace.ProjectDomains{
				Deploy: &workspace.ProjectDeployBackend{
					Kind:   workspace.DeployBackendVercel,
					Config: rawConfig(t, deployEnvCfg{Env: "staging"}),
				},
			}},
			// Empty Env in the config blob is allowed.
			{Name: "api", Domains: &workspace.ProjectDomains{
				Deploy: &workspace.ProjectDeployBackend{Kind: workspace.DeployBackendCloudflare},
			}},
			{Name: "static", Domains: &workspace.ProjectDomains{
				Deploy: &workspace.ProjectDeployBackend{
					Kind:   workspace.DeployBackendEdgeOne,
					Config: rawConfig(t, deployEnvCfg{Env: "prod"}),
				},
			}},
		},
	}
	if err := validateProjectEnvs(m); err != nil {
		t.Fatalf("validateProjectEnvs returned error for valid manifest: %v", err)
	}
}
