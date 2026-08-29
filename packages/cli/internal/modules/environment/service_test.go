package environment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestServiceResolvesScopeFromManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Environments: &workspace.Environments{
			Names: []string{"dev", "production"}, Default: "dev",
		},
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendInfisical},
		},
	}); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t)
	base := execution.NewScope(context.Background(), root)
	resolution, err := service.resolve(resolveInput{
		Scope: base, Requested: "production", Capability: catalog.CapabilityEnvGet, Verb: "get",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Scope.WorkspaceRoot() != root ||
		resolution.Scope.Backend().String() != "env/infisical" ||
		resolution.Scope.Environment() != "production" {
		t.Fatalf("resolution scope = %#v", resolution.Scope)
	}
	if base.WorkspaceRoot() != "" || base.Backend().String() != "" {
		t.Fatal("environment resolution mutated its parent scope")
	}
}

func TestPlanSetOwnsEnvironmentAndProjectSelection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	projectDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Environments: &workspace.Environments{
			Names: []string{"dev"}, Default: "dev",
		},
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendDotenv},
		},
		Projects: []workspace.ManifestProject{{Name: "web", RelativeDir: "apps/web"}},
	}); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t)
	plan, err := service.PlanSet(PlanSetInput{
		Scope: execution.NewScope(context.Background(), root), Environment: "qa",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Environment != "qa" || !plan.NeedsEnvironmentCreation ||
		len(plan.ProjectChoices) != 1 || plan.ProjectChoices[0] != "web" {
		t.Fatalf("root plan = %#v", plan)
	}
	selected := plan.WithProject("web")
	if selected.project != "web" || len(selected.ProjectChoices) != 0 {
		t.Fatalf("selected plan = %#v", selected)
	}

	cwdPlan, err := service.PlanSet(PlanSetInput{
		Scope: execution.NewScope(context.Background(), projectDir), Environment: "dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cwdPlan.NeedsEnvironmentCreation || cwdPlan.project != "web" || len(cwdPlan.ProjectChoices) != 0 {
		t.Fatalf("project plan = %#v", cwdPlan)
	}
}

func TestServiceRejectsMissingCapability(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendDotenv},
		},
	}); err != nil {
		t.Fatal(err)
	}
	_, err := newTestService(t).resolve(resolveInput{
		Scope:      execution.NewScope(context.Background(), root),
		Capability: catalog.CapabilityEnvPull, Verb: "pull",
	})
	if coded, ok := err.(interface{ ErrorCode() string }); !ok ||
		coded.ErrorCode() != "BACKEND_VERB_NOT_SUPPORTED" {
		t.Fatalf("error = %v", err)
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	backendCatalog := catalog.Builtin()
	profiles, err := configureapp.NewProfileService(
		backendCatalog, configureapp.LocalProfileRepository{},
	)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(backendCatalog, profiles)
	if err != nil {
		t.Fatal(err)
	}
	return service
}
