package deployment

import (
	"context"
	"errors"
	"reflect"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	deployport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type providerStub struct {
	id    string
	apply func(context.Context, deployport.ApplyInput) (*deployport.ApplyResult, error)
}

func (p providerStub) ID() string { return p.id }

func (p providerStub) Apply(ctx context.Context, input deployport.ApplyInput) (*deployport.ApplyResult, error) {
	if p.apply != nil {
		return p.apply(ctx, input)
	}
	return &deployport.ApplyResult{Schema: "deploy/test"}, nil
}

type profileResolverStub struct {
	resolved *profile.Resolved
	err      error
	resolve  func(profile.ResolveInput) (*profile.Resolved, error)
}

func (s profileResolverStub) Resolve(input profile.ResolveInput) (*profile.Resolved, error) {
	if s.resolve != nil {
		return s.resolve(input)
	}
	return s.resolved, s.err
}

type builderStub struct {
	build func(context.Context, deployport.BuildInput) ([]string, error)
}

func (s builderStub) Build(ctx context.Context, input deployport.BuildInput) ([]string, error) {
	if s.build == nil {
		return nil, nil
	}
	return s.build(ctx, input)
}

type loaderStub struct {
	id   string
	vars map[string]string
}

func (s loaderStub) ID() string               { return s.id }
func (loaderStub) Priority() secrets.Priority { return secrets.PriorityFilesystem }
func (loaderStub) Available(string) bool      { return true }
func (s loaderStub) Load(context.Context, string, string, string) (map[string]string, error) {
	return s.vars, nil
}

type observerStub struct {
	events  []string
	results []TargetResult
}

func (s *observerStub) TargetStarted(target Target) {
	s.events = append(s.events, "start:"+target.Project.Name)
}

func (s *observerStub) TargetCompleted(result TargetResult) error {
	s.events = append(s.events, "complete:"+result.Target.Project.Name)
	s.results = append(s.results, result)
	return nil
}

func TestServiceDispatchesThroughInjectedRegistry(t *testing.T) {
	service, err := NewService(
		catalog.Builtin(),
		deployport.MustRegistry(providerStub{id: "vercel"}),
		profileResolverStub{},
		secrets.MustRegistry(),
		builderStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.apply(context.Background(), "vercel", deployport.ApplyInput{})
	if err != nil || result.Schema != "deploy/test" {
		t.Fatalf("Apply() = %#v, %v", result, err)
	}
}

func TestExecuteOwnsDeploymentWorkflowOrder(t *testing.T) {
	root := t.TempDir()
	manifest := &workspace.Manifest{
		Workspace:    &workspace.ManifestWorkspace{ID: "ws-demo", Name: "demo"},
		Environments: &workspace.Environments{Names: []string{"dev", "staging"}, Default: "dev"},
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendDotenv},
		},
		Projects: []workspace.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", Toolchain: "node", PackageManager: "pnpm",
			Domains: &workspace.ProjectDomains{
				Deploy: &workspace.ProjectDeployBackend{Kind: workspace.DeployBackendVercel},
			},
		}},
	}
	if err := workspace.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	target := manifestProjectTarget(root, &manifest.Projects[0], workspace.DeployBackendVercel)
	events := []string{}
	resolver := profileResolverStub{resolve: func(input profile.ResolveInput) (*profile.Resolved, error) {
		events = append(events, "profile")
		if input.WorkspaceID != "ws-demo" || input.ProjectName != "web" || input.FlagOverride != "prod" {
			t.Fatalf("profile input = %#v", input)
		}
		return &profile.Resolved{Name: "prod"}, nil
	}}
	builder := builderStub{build: func(_ context.Context, input deployport.BuildInput) ([]string, error) {
		events = append(events, "build")
		if input.Apply.InjectedEnv["TOKEN"] != "secret" || input.Apply.Resolved.Name != "prod" {
			t.Fatalf("build input = %#v", input)
		}
		return []string{"pnpm run build"}, nil
	}}
	provider := providerStub{id: workspace.DeployBackendVercel, apply: func(
		_ context.Context, input deployport.ApplyInput,
	) (*deployport.ApplyResult, error) {
		events = append(events, "apply")
		if input.InjectedEnv["TOKEN"] != "secret" || input.InjectedEnvSource != "dotenv" {
			t.Fatalf("apply input = %#v", input)
		}
		return &deployport.ApplyResult{Schema: "deploy/test", CommandLines: []string{"vercel deploy"}}, nil
	}}
	service, err := NewService(
		catalog.Builtin(), deployport.MustRegistry(provider), resolver,
		secrets.MustRegistry(loaderStub{id: "dotenv", vars: map[string]string{"TOKEN": "secret"}}),
		builder,
	)
	if err != nil {
		t.Fatal(err)
	}
	observer := &observerStub{}
	results, err := service.Execute(context.Background(), ExecuteRequest{
		ProjectRoot: root, Manifest: manifest, Targets: []Target{target},
		Profile: "prod", EnvProvider: "dotenv", Environment: "staging", DryRun: true,
	}, observer)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"profile", "build", "apply"}) {
		t.Fatalf("workflow events = %#v", events)
	}
	if !reflect.DeepEqual(observer.events, []string{"start:web", "complete:web"}) {
		t.Fatalf("observer events = %#v", observer.events)
	}
	if len(results) != 1 || !reflect.DeepEqual(results[0].BuildCommandLines, []string{"pnpm run build"}) {
		t.Fatalf("results = %#v", results)
	}
	if environment, err := readDeployEnvironment(manifest.Projects[0].Domains.Deploy); err != nil || environment != "staging" {
		t.Fatalf("deploy environment = %q, %v", environment, err)
	}
}

func TestResolveProfileTurnsMissingConfigurationIntoNil(t *testing.T) {
	missing := cliErrors.New(cliErrors.PROFILE_NONE_CONFIGURED, "missing")
	service, err := NewService(
		catalog.Builtin(), deployport.MustRegistry(providerStub{id: "vercel"}),
		profileResolverStub{err: missing}, secrets.MustRegistry(), builderStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := service.ResolveProfile(&workspace.Manifest{}, "", Target{Backend: "vercel"})
	if err != nil || resolved != nil {
		t.Fatalf("ResolveProfile() = %#v, %v", resolved, err)
	}

	other := errors.New("read failed")
	service.profiles = profileResolverStub{err: other}
	if _, err := service.ResolveProfile(&workspace.Manifest{}, "", Target{Backend: "vercel"}); !errors.Is(err, other) {
		t.Fatalf("ResolveProfile() error = %v", err)
	}
}
