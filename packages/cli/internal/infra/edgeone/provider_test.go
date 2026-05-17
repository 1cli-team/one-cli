package edgeone

import (
	"context"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// Registry: the package's init() must register an "edgeone" provider so
// deploycmd's `deploy.Get("edgeone")` resolves without manual wiring.
func TestInitRegistersEdgeOneProvider(t *testing.T) {
	p, ok := deploy.Get("edgeone")
	if !ok {
		t.Fatal("deploy.Get(\"edgeone\") returned !ok; init() did not register")
	}
	if p.ID() != "edgeone" {
		t.Fatalf("provider ID = %q, want edgeone", p.ID())
	}
}

// envForProject: returns the manifest pin verbatim, or empty when the
// manifest does not declare one. Empty defaults to production at the
// ops layer via isProduction.
func TestEnvForProject(t *testing.T) {
	tests := []struct {
		name string
		m    *workspace.Manifest
		want string
	}{
		{
			name: "nil manifest yields empty",
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
			name: "project without edgeone config yields empty",
			m: &workspace.Manifest{Projects: []workspace.ManifestProject{
				{Name: "web", Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{Kind: "edgeone"},
				}},
			}},
			want: "",
		},
		{
			name: "explicit env=prod returns prod",
			m:    manifestWithEdgeOneConfig(t, &ProjectConfig{Env: "prod"}),
			want: "prod",
		},
		{
			name: "explicit env=staging returns staging",
			m:    manifestWithEdgeOneConfig(t, &ProjectConfig{Env: "staging"}),
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

// isProduction collapses the env name to EdgeOne's two-state tier:
// empty / "prod" → production; everything else → preview.
func TestIsProduction(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"", true},
		{"prod", true},
		{"dev", false},
		{"staging", false},
		{"qa", false},
		{"  prod  ", true},
	}
	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			if got := isProduction(c.env); got != c.want {
				t.Fatalf("isProduction(%q) = %v, want %v", c.env, got, c.want)
			}
		})
	}
}

func TestProjectNameForProjectReadsManifestPin(t *testing.T) {
	m := manifestWithEdgeOneConfig(t, &ProjectConfig{ProjectName: "demo-eo"})
	if got := projectNameForProject(m, "web"); got != "demo-eo" {
		t.Errorf("projectName = %q, want demo-eo", got)
	}
	if got := projectNameForProject(m, "missing"); got != "" {
		t.Errorf("missing project should give empty name, got %q", got)
	}
	if got := projectNameForProject(nil, "web"); got != "" {
		t.Errorf("nil manifest should give empty name, got %q", got)
	}
}

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

// Provider.Apply must reject inputs missing profile / credentials.
func TestProviderApplyMissingProfileSurfacesEdgeOneProfileInvalid(t *testing.T) {
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
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "EDGEONE_PROFILE_INVALID" {
		t.Fatalf("error = %v, want EDGEONE_PROFILE_INVALID", err)
	}
}

func TestProviderApplyWithoutTokenFallsBackToEdgeOneLogin(t *testing.T) {
	p := providerImpl{}
	res, err := p.Apply(context.Background(), deploy.ApplyInput{
		ProjectRoot: t.TempDir(),
		Project:     workspace.Project{Name: "web", RelativeDir: "apps/web"},
		Manifest:    &workspace.Manifest{},
		Resolved: &profile.Resolved{
			Name:    "default",
			Profile: profile.Profile{Backend: "edgeone", EdgeOne: &profile.EdgeOneProfile{}},
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, a := range res.Argv {
		if a == "--token" {
			t.Fatalf("unexpected token flag without token: %v", res.Argv)
		}
	}
}

// Provider.Apply happy path (dry-run): argv must NOT contain the real token.
func TestProviderApplyDryRunReturnsArgvWithoutSecrets(t *testing.T) {
	p := providerImpl{}
	res, err := p.Apply(context.Background(), deploy.ApplyInput{
		ProjectRoot: t.TempDir(),
		Project:     workspace.Project{Name: "web", RelativeDir: "apps/web"},
		Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{
			{Name: "web", Domains: &workspace.ProjectDomains{
				Deploy: &workspace.ProjectDeployBackend{Kind: "edgeone"},
			}},
		}},
		Resolved: &profile.Resolved{
			Name: "default",
			Profile: profile.Profile{
				Backend: "edgeone",
				EdgeOne: &profile.EdgeOneProfile{
					Region: "ap-guangzhou",
					Credentials: &profile.EdgeOneCredentials{
						APIToken: "edgeone-secret-token",
					},
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
		if contains(a, "edgeone-secret-token") {
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

func manifestWithEdgeOneConfig(t *testing.T, cfg *ProjectConfig) *workspace.Manifest {
	t.Helper()
	p := workspace.ManifestProject{Name: "web"}
	if err := EncodeProjectConfig(&p, cfg); err != nil {
		t.Fatalf("EncodeProjectConfig: %v", err)
	}
	return &workspace.Manifest{Projects: []workspace.ManifestProject{p}}
}
