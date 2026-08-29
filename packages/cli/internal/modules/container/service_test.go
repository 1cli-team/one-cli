package container

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	containermodel "github.com/torchstellar-team/one-cli/packages/cli/internal/core/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestServiceRejectsUnknownBackend(t *testing.T) {
	service, err := NewService(catalog.Builtin())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Build(context.Background(), "not-in-catalog", containermodel.BuildInput{}); err == nil {
		t.Fatal("Build() dispatched an unknown backend")
	}
}

func TestServiceRejectsBackendOutsideCompiledOCIFamily(t *testing.T) {
	backendCatalog, err := catalog.New(catalog.BackendSpec{
		ID: catalog.BackendID{Domain: catalog.DomainContainer, Name: "custom"},
		Capabilities: []catalog.Capability{
			catalog.CapabilityContainerInfo,
			catalog.CapabilityContainerBuild,
			catalog.CapabilityContainerPush,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewService(backendCatalog); err == nil {
		t.Fatal("NewService() accepted a backend outside the compiled OCI family")
	}
}

func TestServiceUsesSharedDockerImplementationForCatalogBackends(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{{
			Name:         "api",
			RelativeDir:  "services/api",
			TemplateID:   "go-api",
			Toolchain:    "go",
			BuildVersion: "v0.1.0",
			Domains: &workspace.ProjectDomains{
				Container: &workspace.ProjectContainerOverride{},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}

	service, err := NewService(catalog.Builtin())
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range []string{"docker", "dockerhub", "ghcr", "acr"} {
		result, err := service.Build(context.Background(), backend, containermodel.BuildInput{
			ProjectRoot: root,
			Project:     "api",
			TargetNames: []string{"api"},
			Tag:         "v1.2.3",
			DryRun:      true,
		})
		if err != nil {
			t.Fatalf("Build(%s): %v", backend, err)
		}
		if len(result.Built) != 1 || result.Built[0].Image != "api:v1.2.3" {
			t.Fatalf("Build(%s) = %#v", backend, result)
		}
	}
}

func TestPublishBuildResultOwnsManifestBookkeeping(t *testing.T) {
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version: workspace.ManifestVersion,
		Projects: []workspace.ManifestProject{{
			Name:         "api",
			RelativeDir:  "services/api",
			TemplateID:   "go-api",
			Toolchain:    "go",
			BuildVersion: "v0.1.0",
			Domains: &workspace.ProjectDomains{
				Container: &workspace.ProjectContainerOverride{},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := publishBuildResult(root, "linux/amd64", &containermodel.BuildResult{
		Built: []containermodel.BuildEntry{{Project: "api", Image: "ghcr.io/team/api:v2.3.4"}},
	}); err != nil {
		t.Fatal(err)
	}
	manifest, err := workspace.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	project := manifest.Projects[0]
	if project.Domains.Container.Image != "ghcr.io/team/api:v2.3.4" || project.BuildVersion != "2.3.4" {
		t.Fatalf("published project = %#v", project)
	}
	if got := workspace.ContainerPlatform(manifest); got != "linux/amd64" {
		t.Fatalf("container platform = %q", got)
	}
}
