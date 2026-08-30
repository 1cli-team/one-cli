package deployment

import (
	"reflect"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	deployport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

func newPlanningService(t *testing.T) *Service {
	t.Helper()
	service, err := NewService(
		catalog.Builtin(),
		deployport.MustRegistry(
			providerStub{id: workspace.DeployBackendVercel},
			providerStub{id: workspace.DeployBackendCloudflare},
		),
		profileResolverStub{},
		secrets.MustRegistry(),
		builderStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func planningFixture(t *testing.T) (string, *workspace.Manifest, *template.Registry) {
	t.Helper()
	root := t.TempDir()
	manifest := &workspace.Manifest{Projects: []workspace.ManifestProject{
		{
			Name: "web", RelativeDir: "apps/web", TemplateID: "web-template",
			Domains: &workspace.ProjectDomains{Deploy: &workspace.ProjectDeployBackend{
				Kind: workspace.DeployBackendVercel,
			}},
		},
		{Name: "api", RelativeDir: "apps/api", TemplateID: "web-template"},
		{Name: "library", RelativeDir: "packages/library", TemplateID: "library-template"},
	}}
	if err := workspace.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	registry := &template.Registry{Templates: []template.Template{
		{ID: "web-template", Compat: map[string][]string{
			"deploy": {workspace.DeployBackendVercel, workspace.DeployBackendCloudflare},
		}},
		{ID: "library-template", Compat: map[string][]string{"deploy": {}}},
	}}
	return root, manifest, registry
}

func TestPlanTargetsUsesAllConfiguredTargetsByDefault(t *testing.T) {
	root, manifest, registry := planningFixture(t)
	plan, err := newPlanningService(t).PlanTargets(PlanRequest{
		ProjectRoot: root, Manifest: manifest, Templates: registry,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Targets) != 1 || plan.Targets[0].Project.Name != "web" ||
		plan.Setup != nil || len(plan.ProjectChoices) != 0 {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanTargetsResolvesExistingTargetByRelativePath(t *testing.T) {
	root, manifest, registry := planningFixture(t)
	plan, err := newPlanningService(t).PlanTargets(PlanRequest{
		ProjectRoot: root, Manifest: manifest, Templates: registry,
		Project: "./apps/web/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Targets) != 1 || plan.Targets[0].Project.Name != "web" || plan.Setup != nil {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanTargetsReturnsDeployableProjectChoices(t *testing.T) {
	root, manifest, registry := planningFixture(t)
	plan, err := newPlanningService(t).PlanTargets(PlanRequest{
		ProjectRoot: root, Manifest: manifest, Templates: registry,
		Backend: workspace.DeployBackendCloudflare,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.ProjectChoices, []string{"web", "api"}) ||
		len(plan.Targets) != 0 || plan.Setup != nil {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanTargetsReturnsFirstDeploymentSetup(t *testing.T) {
	root, manifest, registry := planningFixture(t)
	plan, err := newPlanningService(t).PlanTargets(PlanRequest{
		ProjectRoot: root, Manifest: manifest, Templates: registry,
		Project: "api", Backend: "deploy/cloudflare",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Setup == nil || plan.Setup.Project.Name != "api" ||
		plan.Setup.Backend != workspace.DeployBackendCloudflare ||
		!reflect.DeepEqual(plan.Setup.CompatibleBackends,
			[]string{workspace.DeployBackendVercel, workspace.DeployBackendCloudflare}) {
		t.Fatalf("plan = %#v", plan)
	}
	target, err := plan.Setup.ResolveTarget(root, workspace.DeployBackendCloudflare)
	if err != nil || target.Project.Name != "api" || target.Backend != workspace.DeployBackendCloudflare {
		t.Fatalf("ResolveTarget() = %#v, %v", target, err)
	}
}

func TestPlanTargetsInfersTheOnlyProjectWhenBackendIsExplicit(t *testing.T) {
	root := t.TempDir()
	manifest := &workspace.Manifest{Projects: []workspace.ManifestProject{{
		Name: "web", RelativeDir: "apps/web", TemplateID: "web-template",
	}}}
	if err := workspace.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	registry := &template.Registry{Templates: []template.Template{{
		ID: "web-template", Compat: map[string][]string{
			"deploy": {workspace.DeployBackendVercel},
		},
	}}}
	plan, err := newPlanningService(t).PlanTargets(PlanRequest{
		ProjectRoot: root, Manifest: manifest, Templates: registry,
		Backend: workspace.DeployBackendVercel,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Setup == nil || plan.Setup.Project.Name != "web" ||
		plan.Setup.Backend != workspace.DeployBackendVercel {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanTargetsRejectsUnknownProjectAndIncompatibleBackend(t *testing.T) {
	root, manifest, registry := planningFixture(t)
	service := newPlanningService(t)
	for _, tt := range []struct {
		name    string
		request PlanRequest
		code    string
	}{
		{
			name: "unknown project",
			request: PlanRequest{ProjectRoot: root, Manifest: manifest, Templates: registry,
				Project: "missing"},
			code: "SUBPROJECT_NOT_FOUND",
		},
		{
			name: "incompatible backend",
			request: PlanRequest{ProjectRoot: root, Manifest: manifest, Templates: registry,
				Project: "api", Backend: workspace.DeployBackendKustomize},
			code: "PROFILE_BACKEND_INVALID",
		},
		{
			name: "project is not deployable",
			request: PlanRequest{ProjectRoot: root, Manifest: manifest, Templates: registry,
				Project: "library"},
			code: "BACKEND_NOT_ENABLED",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := service.PlanTargets(tt.request); errorCode(err) != tt.code {
				t.Fatalf("PlanTargets() error = %v, code = %q", err, errorCode(err))
			}
		})
	}
}
