package containercmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	containermodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/container"
)

func TestBulkBuildAndPushResolveRegistryPerProjectForSelectedEnvironment(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)
	root := t.TempDir()
	manifest := &workspace.Manifest{
		Version:      workspace.ManifestVersion,
		Workspace:    &workspace.ManifestWorkspace{ID: "workspace-id", Name: "demo"},
		Environments: &workspace.Environments{Names: []string{"dev", "staging", "prod"}, Default: "dev"},
		Projects: []workspace.ManifestProject{
			{
				Name: "web", RelativeDir: "apps/web", Toolchain: "node",
				Domains: &workspace.ProjectDomains{Container: &workspace.ProjectContainerOverride{
					Kind: catalog.ContainerGHCR,
				}},
			},
			{
				Name: "api", RelativeDir: "services/api", Toolchain: "go",
				Domains: &workspace.ProjectDomains{Container: &workspace.ProjectContainerOverride{
					Kind: catalog.ContainerDockerHub,
				}},
			},
		},
	}
	if err := workspace.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	for _, relativeDir := range []string{"apps/web", "services/api"} {
		projectDir := filepath.Join(root, filepath.FromSlash(relativeDir))
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	profiles := []struct {
		project, backend, name, namespace string
	}{
		{project: "web", backend: catalog.ContainerGHCR, name: "web-preview", namespace: "web-team"},
		{project: "api", backend: catalog.ContainerDockerHub, name: "api-preview", namespace: "api-team"},
	}
	for _, entry := range profiles {
		if _, err := profile.Upsert(
			profile.DomainContainer,
			entry.backend,
			entry.name,
			profile.Profile{
				Backend: entry.backend,
				Container: &profile.ContainerProfile{
					Namespace: entry.namespace,
					Credentials: &profile.ContainerCredentials{
						Username: entry.project + "-user", Password: "secret",
					},
				},
			},
			false,
		); err != nil {
			t.Fatal(err)
		}
		if err := profile.BindEnvironmentProfile(
			"workspace-id", "demo", root, entry.project, "staging",
			profile.DomainContainer, entry.backend, entry.name,
		); err != nil {
			t.Fatal(err)
		}
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	service, err := containermodule.NewService(catalog.Builtin())
	if err != nil {
		t.Fatal(err)
	}
	deps := Dependencies{Service: service}

	buildOutput := executeContainerCommand(t, newBuildCmd(deps), []string{
		"--dry-run", "--env", "preview", "--build-version", "v1.2.3",
	})
	for _, image := range []string{
		"ghcr.io/web-team/web:v1.2.3",
		"index.docker.io/api-team/api:v1.2.3",
	} {
		if !strings.Contains(buildOutput, "docker build") || !strings.Contains(buildOutput, image) {
			t.Fatalf("bulk build output missing %q:\n%s", image, buildOutput)
		}
	}

	pushOutput := executeContainerCommand(t, newPushCmd(deps), []string{
		"--dry-run", "--env", "preview", "--build-version", "v1.2.3",
	})
	for _, image := range []string{
		"ghcr.io/web-team/web:v1.2.3",
		"index.docker.io/api-team/api:v1.2.3",
	} {
		if !strings.Contains(pushOutput, "docker push "+image) {
			t.Fatalf("bulk push output missing %q:\n%s", image, pushOutput)
		}
	}
}

func TestBulkBuildResolvesKustomizePlatformPerProjectAndEnvironment(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)
	root := t.TempDir()
	manifest := &workspace.Manifest{
		Version:      workspace.ManifestVersion,
		Workspace:    &workspace.ManifestWorkspace{ID: "workspace-id", Name: "demo"},
		Environments: &workspace.Environments{Names: []string{"dev", "preview", "prod"}, Default: "dev"},
		Domains: &workspace.WorkspaceDomains{
			Deploy: &workspace.BackendRef{Kind: workspace.DeployBackendKustomize},
		},
		Projects: []workspace.ManifestProject{
			{
				Name: "web", RelativeDir: "apps/web", Toolchain: "node",
				Domains: &workspace.ProjectDomains{
					Container: &workspace.ProjectContainerOverride{Kind: catalog.ContainerDocker},
					Deploy:    &workspace.ProjectDeployBackend{Kind: workspace.DeployBackendKustomize},
				},
			},
			{
				Name: "api", RelativeDir: "services/api", Toolchain: "go",
				Domains: &workspace.ProjectDomains{
					Container: &workspace.ProjectContainerOverride{Kind: catalog.ContainerDocker},
					Deploy:    &workspace.ProjectDeployBackend{Kind: workspace.DeployBackendKustomize},
				},
			},
		},
	}
	if err := workspace.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, workspace.ManifestFilename)
	manifestBefore, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, relativeDir := range []string{"apps/web", "services/api"} {
		projectDir := filepath.Join(root, filepath.FromSlash(relativeDir))
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	profiles := []struct {
		project, name, kubeconfig, kubeContext string
	}{
		{project: "web", name: "web-preview", kubeconfig: "/clusters/web", kubeContext: "web-context"},
		{project: "api", name: "api-preview", kubeconfig: "/clusters/api", kubeContext: "api-context"},
	}
	for _, entry := range profiles {
		if _, err := profile.Upsert(
			profile.DomainDeploy,
			workspace.DeployBackendKustomize,
			entry.name,
			profile.Profile{
				Backend: workspace.DeployBackendKustomize,
				Kustomize: &profile.KustomizeProfile{
					KubeconfigPath: entry.kubeconfig, KubeconfigContext: entry.kubeContext,
				},
			},
			false,
		); err != nil {
			t.Fatal(err)
		}
		if err := profile.BindEnvironmentProfile(
			"workspace-id", "demo", root, entry.project, "preview",
			profile.DomainDeploy, workspace.DeployBackendKustomize, entry.name,
		); err != nil {
			t.Fatal(err)
		}
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	service, err := containermodule.NewService(catalog.Builtin())
	if err != nil {
		t.Fatal(err)
	}
	type detectorCall struct {
		kubeconfig, kubeContext string
	}
	var detectorCalls []detectorCall
	deps := Dependencies{
		Service: service,
		detectKubeNodePlatform: func(kubeconfig, kubeContext string) string {
			detectorCalls = append(detectorCalls, detectorCall{
				kubeconfig: kubeconfig, kubeContext: kubeContext,
			})
			switch kubeconfig {
			case "/clusters/web":
				return "linux/amd64"
			case "/clusters/api":
				return "linux/arm64"
			default:
				t.Fatalf("unexpected kubeconfig %q", kubeconfig)
				return ""
			}
		},
	}

	buildOutput := executeContainerCommand(t, newBuildCmd(deps), []string{
		"--dry-run", "--env", "preview", "--build-version", "v1.2.3",
	})
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		prefix      string
		relativeDir string
	}{
		{prefix: "docker build --platform linux/amd64 -t web:v1.2.3 ", relativeDir: "apps/web"},
		{prefix: "docker build --platform linux/arm64 -t api:v1.2.3 ", relativeDir: "services/api"},
	} {
		var buildPath string
		for _, line := range strings.Split(strings.TrimSpace(buildOutput), "\n") {
			line = strings.TrimSuffix(line, "\r")
			if strings.HasPrefix(line, want.prefix) {
				buildPath = strings.TrimSpace(strings.TrimPrefix(line, want.prefix))
				break
			}
		}
		if buildPath == "" {
			t.Fatalf("bulk build output missing prefix %q:\n%s", want.prefix, buildOutput)
		}
		canonicalBuildPath, err := filepath.EvalSymlinks(buildPath)
		if err != nil {
			t.Fatal(err)
		}
		wantPath := filepath.Join(canonicalRoot, filepath.FromSlash(want.relativeDir))
		if filepath.Clean(canonicalBuildPath) != filepath.Clean(wantPath) {
			t.Fatalf("bulk build path = %q, want %q", canonicalBuildPath, wantPath)
		}
	}
	wantDetectorCalls := []detectorCall{
		{kubeconfig: "/clusters/web", kubeContext: "web-context"},
		{kubeconfig: "/clusters/api", kubeContext: "api-context"},
	}
	if len(detectorCalls) != len(wantDetectorCalls) {
		t.Fatalf("detector calls = %#v, want %#v", detectorCalls, wantDetectorCalls)
	}
	for i := range wantDetectorCalls {
		if detectorCalls[i] != wantDetectorCalls[i] {
			t.Fatalf("detector call %d = %#v, want %#v", i, detectorCalls[i], wantDetectorCalls[i])
		}
	}
	manifestAfter, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(manifestAfter, manifestBefore) {
		t.Fatal("dry-run bulk build modified one.manifest.json")
	}
}

func executeContainerCommand(t *testing.T, command *cobra.Command, args []string) string {
	t.Helper()
	var output bytes.Buffer
	command.SetArgs(args)
	command.SetOut(&output)
	command.SetErr(&output)
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	return output.String()
}
