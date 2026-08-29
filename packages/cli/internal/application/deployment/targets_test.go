package deployment

import (
	"path/filepath"
	"reflect"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestConfiguredTargetsUsesManifestDeploymentState(t *testing.T) {
	root := t.TempDir()
	manifest := &workspace.Manifest{Projects: []workspace.ManifestProject{
		{Name: "web", RelativeDir: "apps/web", Toolchain: "node", PackageManager: "pnpm", Domains: &workspace.ProjectDomains{
			Deploy: &workspace.ProjectDeployBackend{Kind: workspace.DeployBackendVercel},
		}},
		{Name: "library", RelativeDir: "packages/library", Toolchain: "go"},
	}}
	if err := workspace.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	targets, err := configuredTargets(root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Project.TargetDir != filepath.Join(root, "apps", "web") ||
		targets[0].Backend != workspace.DeployBackendVercel {
		t.Fatalf("targets = %#v", targets)
	}
}

func TestCompatibleProvidersIntersectsTemplateAndCatalog(t *testing.T) {
	projectTemplate := &template.Template{Compat: map[string][]string{
		"deploy": {"vercel", "not-registered", "cloudflare"},
	}}
	got := compatibleBackends(catalog.Builtin(), projectTemplate)
	if !reflect.DeepEqual(got, []string{"vercel", "cloudflare"}) {
		t.Fatalf("compatibleBackends() = %#v", got)
	}
}
