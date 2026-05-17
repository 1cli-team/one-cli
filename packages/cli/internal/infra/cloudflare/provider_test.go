package cloudflare

import (
	"context"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// Registry: the package's init() must register a "cloudflare" provider
// so deploycmd's `deploy.Get("cloudflare")` resolves without manual
// wiring.
func TestInitRegistersCloudflareProvider(t *testing.T) {
	p, ok := deploy.Get("cloudflare")
	if !ok {
		t.Fatal("deploy.Get(\"cloudflare\") returned !ok; init() did not register")
	}
	if p.ID() != "cloudflare" {
		t.Fatalf("provider ID = %q, want cloudflare", p.ID())
	}
}

// envForProject: returns the manifest pin verbatim, or empty when the
// manifest does not declare one. Empty defaults to production at the
// ops layer via resolveEnvName.
func TestEnvForProject(t *testing.T) {
	tests := []struct {
		name string
		m    *workspace.Manifest
		want string
	}{
		{
			name: "nil manifest yields empty (production default)",
			m:    nil,
			want: "",
		},
		{
			name: "missing project yields empty",
			m:    &workspace.Manifest{Projects: []workspace.ManifestProject{{Name: "other"}}},
			want: "",
		},
		{
			name: "project without deploy section yields empty",
			m: &workspace.Manifest{Projects: []workspace.ManifestProject{
				{Name: "web"},
			}},
			want: "",
		},
		{
			name: "project without cloudflare config yields empty",
			m: &workspace.Manifest{Projects: []workspace.ManifestProject{
				{Name: "web", Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{Kind: "cloudflare"},
				}},
			}},
			want: "",
		},
		{
			name: "explicit env=prod returns prod",
			m:    manifestWithCloudflareEnv(t, "prod"),
			want: "prod",
		},
		{
			name: "explicit env=staging returns staging",
			m:    manifestWithCloudflareEnv(t, "staging"),
			want: "staging",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envForProject(tt.m, "web")
			if got != tt.want {
				t.Fatalf("envForProject = %q, want %q", got, tt.want)
			}
		})
	}
}

// resolveEnvName maps the user-facing Env onto wrangler's --env flag.
// Empty / "prod" → "" (no flag); anything else → the value verbatim.
func TestResolveEnvName(t *testing.T) {
	cases := []struct {
		env  string
		want string
	}{
		{"", ""},
		{"prod", ""},
		{"  prod  ", ""}, // trimmed
		{"staging", "staging"},
		{"dev", "dev"},
		{"qa", "qa"},
	}
	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			got := resolveEnvName(ApplyInput{Env: c.env})
			if got != c.want {
				t.Fatalf("resolveEnvName(%q) = %q, want %q", c.env, got, c.want)
			}
		})
	}
}

// projectDirFor honours an explicit TargetDir, otherwise joins
// ProjectRoot + RelativeDir.
func TestProjectDirFor(t *testing.T) {
	withTarget := deploy.ApplyInput{
		ProjectRoot: "/repo",
		Project: workspace.Project{
			Name:        "web",
			RelativeDir: "apps/web",
			TargetDir:   "/some/explicit/path",
		},
	}
	if got := projectDirFor(withTarget); got != "/some/explicit/path" {
		t.Errorf("explicit TargetDir not honoured: got %q", got)
	}
	withoutTarget := deploy.ApplyInput{
		ProjectRoot: "/repo",
		Project: workspace.Project{
			Name:        "web",
			RelativeDir: "apps/web",
		},
	}
	if got := projectDirFor(withoutTarget); got != "/repo/apps/web" {
		t.Errorf("fallback path = %q, want /repo/apps/web", got)
	}
}

// Provider.Apply must reject an input with no resolved profile —
// without the API token there's nothing to authenticate as.
func TestProviderApplyMissingProfileSurfacesCloudflareProfileInvalid(t *testing.T) {
	p := providerImpl{}
	_, err := p.Apply(context.Background(), deploy.ApplyInput{
		ProjectRoot: t.TempDir(),
		Project:     workspace.Project{Name: "web", RelativeDir: "apps/web"},
		Manifest:    &workspace.Manifest{},
		Resolved:    nil,
		DryRun:      true,
	})
	if err == nil {
		t.Fatal("expected error for nil Resolved")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "CLOUDFLARE_PROFILE_INVALID" {
		t.Fatalf("error = %v, want CLOUDFLARE_PROFILE_INVALID", err)
	}
}

// Provider.Apply also rejects a resolved profile whose Credentials
// pointer is nil or whose APIToken is empty.
func TestProviderApplyEmptyTokenSurfacesCloudflareProfileInvalid(t *testing.T) {
	p := providerImpl{}
	_, err := p.Apply(context.Background(), deploy.ApplyInput{
		ProjectRoot: t.TempDir(),
		Project:     workspace.Project{Name: "web", RelativeDir: "apps/web"},
		Manifest:    &workspace.Manifest{},
		Resolved: &profile.Resolved{
			Name:    "default",
			Profile: profile.Profile{Backend: "cloudflare", Cloudflare: &profile.CloudflareProfile{}},
		},
		DryRun: true,
	})
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "CLOUDFLARE_PROFILE_INVALID" {
		t.Fatalf("error = %v, want CLOUDFLARE_PROFILE_INVALID", err)
	}
}

// Provider.Apply happy path (dry-run): a resolved profile with token +
// account threads through to the underlying Apply, returns the
// deploy.ApplyResult envelope. argv must NOT contain the API token —
// auth flows via cmd.Env at exec time.
func TestProviderApplyDryRunReturnsArgvWithoutToken(t *testing.T) {
	p := providerImpl{}
	res, err := p.Apply(context.Background(), deploy.ApplyInput{
		ProjectRoot: t.TempDir(),
		Project:     workspace.Project{Name: "web", RelativeDir: "apps/web"},
		Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{
			{Name: "web", Domains: &workspace.ProjectDomains{
				Deploy: &workspace.ProjectDeployBackend{Kind: "cloudflare"},
			}},
		}},
		Resolved: &profile.Resolved{
			Name: "default",
			Profile: profile.Profile{
				Backend: "cloudflare",
				Cloudflare: &profile.CloudflareProfile{
					AccountID:   "acct-abc",
					Credentials: &profile.CloudflareCredentials{APIToken: "secret-tok-1"},
				},
			},
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	if !res.DryRun {
		t.Errorf("DryRun = false, want true")
	}
	if res.Schema != SchemaApply {
		t.Errorf("Schema = %q, want %q", res.Schema, SchemaApply)
	}
	for _, a := range res.Argv {
		if contains(a, "secret-tok-1") {
			t.Fatalf("argv leaked token: %v", res.Argv)
		}
		if contains(a, "acct-abc") {
			t.Fatalf("argv leaked account id: %v", res.Argv)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func manifestWithCloudflareEnv(t *testing.T, env string) *workspace.Manifest {
	t.Helper()
	p := workspace.ManifestProject{Name: "web"}
	if err := EncodeProjectConfig(&p, &ProjectConfig{Env: env}); err != nil {
		t.Fatalf("EncodeProjectConfig: %v", err)
	}
	return &workspace.Manifest{Projects: []workspace.ManifestProject{p}}
}
