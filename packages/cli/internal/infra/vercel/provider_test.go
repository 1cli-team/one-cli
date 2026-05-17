package vercel

import (
	"context"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// Registry: the package's init() must register a "vercel" provider so
// deploycmd's `deploy.Get("vercel")` resolves without manual wiring.
func TestInitRegistersVercelProvider(t *testing.T) {
	p, ok := deploy.Get("vercel")
	if !ok {
		t.Fatal("deploy.Get(\"vercel\") returned !ok; init() did not register")
	}
	if p.ID() != "vercel" {
		t.Fatalf("provider ID = %q, want vercel", p.ID())
	}
}

// envForProject: returns the per-project Env pin verbatim, or empty
// when the manifest does not declare one. Empty defaults to production
// at the ops layer via isProduction.
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
			name: "project without vercel config yields empty",
			m: &workspace.Manifest{Projects: []workspace.ManifestProject{
				{Name: "web", Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{Kind: "vercel"},
				}},
			}},
			want: "",
		},
		{
			name: "explicit env=prod returns prod",
			m:    manifestWithVercelEnv(t, "prod"),
			want: "prod",
		},
		{
			name: "explicit env=staging returns staging",
			m:    manifestWithVercelEnv(t, "staging"),
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

// isProduction collapses the user-facing env name to the production /
// preview tier the upstream Vercel CLI exposes.
func TestIsProduction(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"", true},     // unset default = production
		{"prod", true}, // explicit prod = production
		{"dev", false}, // anything else = preview
		{"staging", false},
		{"qa", false},
		{"  prod  ", true}, // trimmed
	}
	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			if got := isProduction(c.env); got != c.want {
				t.Fatalf("isProduction(%q) = %v, want %v", c.env, got, c.want)
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
func TestProviderApplyMissingProfileSurfacesVercelProfileInvalid(t *testing.T) {
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
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "VERCEL_PROFILE_INVALID" {
		t.Fatalf("error = %v, want VERCEL_PROFILE_INVALID", err)
	}
}

// Provider.Apply also rejects a resolved profile whose Credentials
// pointer is nil or whose APIToken is empty — same VERCEL_PROFILE_INVALID
// code, different remediation phrasing inside the message.
func TestProviderApplyEmptyTokenSurfacesVercelProfileInvalid(t *testing.T) {
	p := providerImpl{}
	_, err := p.Apply(context.Background(), deploy.ApplyInput{
		ProjectRoot: t.TempDir(),
		Project:     workspace.Project{Name: "web", RelativeDir: "apps/web"},
		Manifest:    &workspace.Manifest{},
		Resolved: &profile.Resolved{
			Name:    "default",
			Profile: profile.Profile{Backend: "vercel", Vercel: &profile.VercelProfile{}},
		},
		DryRun: true,
	})
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "VERCEL_PROFILE_INVALID" {
		t.Fatalf("error = %v, want VERCEL_PROFILE_INVALID", err)
	}
}

// Provider.Apply happy path (dry-run): a resolved profile with token +
// team threads through to the underlying Apply, returns the deploy.ApplyResult
// envelope with masked argv.
func TestProviderApplyDryRunReturnsMaskedEnvelope(t *testing.T) {
	p := providerImpl{}
	res, err := p.Apply(context.Background(), deploy.ApplyInput{
		ProjectRoot: t.TempDir(),
		Project:     workspace.Project{Name: "web", RelativeDir: "apps/web"},
		Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{
			{Name: "web", Domains: &workspace.ProjectDomains{
				Deploy: &workspace.ProjectDeployBackend{Kind: "vercel"},
			}},
		}},
		Resolved: &profile.Resolved{
			Name: "default",
			Profile: profile.Profile{
				Backend: "vercel",
				Vercel: &profile.VercelProfile{
					Team:        "acme",
					Credentials: &profile.VercelCredentials{APIToken: "secret-tok-1"},
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

// manifestWithVercelEnv builds a one-project manifest with a
// per-project Vercel deploy section pinned to the given env name.
// Test-only helper that exercises EncodeProjectConfig so the wire shape
// is identical to what the provider writes during real invocations.
func manifestWithVercelEnv(t *testing.T, env string) *workspace.Manifest {
	t.Helper()
	p := workspace.ManifestProject{Name: "web"}
	if err := EncodeProjectConfig(&p, &ProjectConfig{Env: env}); err != nil {
		t.Fatalf("EncodeProjectConfig: %v", err)
	}
	return &workspace.Manifest{Projects: []workspace.ManifestProject{p}}
}
