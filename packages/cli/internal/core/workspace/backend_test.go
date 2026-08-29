package workspace

// Locks the read/write helpers for the current deploy/container fields:
//
//   - projects[i].domains.container.namespace  (per-project)
//   - projects[i].domains.deploy.config.bucket (per-project, s3 only)
//   - manifest.domains.deploy.config.namespace (workspace, kustomize only)
//   - manifest.domains.deploy.config.kustomizationPath (workspace, kustomize only)

import (
	"encoding/json"
	"testing"
)

// rawConfig is a small helper that JSON-encodes a struct into a
// json.RawMessage suitable for stuffing into BackendRef.Config /
// ProjectDeployBackend.Config. Test-only; production sites use the typed
// per-kind config Encode helpers.
func rawConfig(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("rawConfig: %v", err)
	}
	return raw
}

func TestContainerNamespaceForProject_ReadsManifest(t *testing.T) {
	m := &Manifest{
		Version: ManifestVersion,
		Projects: []ManifestProject{
			{
				Name:        "api",
				RelativeDir: "services/api",
				Domains: &ProjectDomains{
					Container: &ProjectContainerOverride{Namespace: "acme-corp"},
				},
			},
			{Name: "lib", RelativeDir: "packages/lib"},
		},
	}
	if got := ContainerNamespaceForProject(m, "api"); got != "acme-corp" {
		t.Errorf("api: got %q, want acme-corp", got)
	}
	if got := ContainerNamespaceForProject(m, "lib"); got != "" {
		t.Errorf("lib has no container section, got %q", got)
	}
	if got := ContainerNamespaceForProject(m, "missing"); got != "" {
		t.Errorf("missing project should return empty, got %q", got)
	}
	if got := ContainerNamespaceForProject(nil, "api"); got != "" {
		t.Errorf("nil manifest should return empty, got %q", got)
	}
}

func TestContainerKindForProject(t *testing.T) {
	m := &Manifest{
		Version: ManifestVersion,
		Domains: &WorkspaceDomains{
			Container: &BackendRef{Kind: "dockerhub"},
		},
		Projects: []ManifestProject{
			{
				Name:        "api",
				RelativeDir: "services/api",
				Domains: &ProjectDomains{
					Container: &ProjectContainerOverride{Kind: "acr"},
				},
			},
			{
				Name:        "web",
				RelativeDir: "apps/web",
				Domains: &ProjectDomains{
					Container: &ProjectContainerOverride{}, // explicit but no Kind
				},
			},
			{Name: "lib", RelativeDir: "packages/lib"},
		},
	}
	// per-project pin wins over workspace default
	if got := ContainerKindForProject(m, "api"); got != "acr" {
		t.Errorf("api: got %q, want acr", got)
	}
	// empty per-project Kind falls back to workspace-level
	if got := ContainerKindForProject(m, "web"); got != "dockerhub" {
		t.Errorf("web: got %q, want dockerhub", got)
	}
	// no container section at all → workspace-level (still dockerhub)
	if got := ContainerKindForProject(m, "lib"); got != "dockerhub" {
		t.Errorf("lib: got %q, want dockerhub", got)
	}
	// nil manifest falls back to "docker"
	if got := ContainerKindForProject(nil, "api"); got != ContainerBackendDocker {
		t.Errorf("nil manifest: got %q, want %q", got, ContainerBackendDocker)
	}
	// manifest with no workspace-level container falls back to "docker"
	empty := &Manifest{Version: ManifestVersion}
	if got := ContainerKindForProject(empty, "api"); got != ContainerBackendDocker {
		t.Errorf("empty manifest: got %q, want %q", got, ContainerBackendDocker)
	}
}

func TestDeployBucketForProject_ReadsManifest(t *testing.T) {
	m := &Manifest{
		Version: ManifestVersion,
		Workspace: &ManifestWorkspace{
			ID:   "demo-abc123",
			Name: "demo",
		},
		Projects: []ManifestProject{
			{
				Name:        "web",
				RelativeDir: "apps/web",
				Domains: &ProjectDomains{
					Deploy: &ProjectDeployBackend{
						Kind:   "aws-s3",
						Config: rawConfig(t, map[string]string{"bucket": "web-prod"}),
					},
				},
			},
			{
				Name:        "api",
				RelativeDir: "services/api",
				Domains: &ProjectDomains{
					Deploy: &ProjectDeployBackend{Kind: "kustomize"},
				},
			},
			{
				Name:        "docs",
				RelativeDir: "apps/docs",
				Domains: &ProjectDomains{
					Deploy: &ProjectDeployBackend{Kind: "aws-s3"},
				},
			},
		},
	}
	if got := DeployBucketForProject(m, "web"); got != "web-prod" {
		t.Errorf("web: got %q, want web-prod", got)
	}
	if got := ExplicitDeployBucketForProject(m, "web"); got != "web-prod" {
		t.Errorf("web explicit: got %q, want web-prod", got)
	}
	if got := DeployBucketForProject(m, "docs"); got != "demo-abc123" {
		t.Errorf("docs fallback: got %q, want demo-abc123", got)
	}
	if got := ExplicitDeployBucketForProject(m, "docs"); got != "" {
		t.Errorf("docs explicit bucket should be empty, got %q", got)
	}
	// kustomize project: bucket is unset, helper returns empty.
	if got := DeployBucketForProject(m, "api"); got != "" {
		t.Errorf("api (kustomize): got %q, want empty", got)
	}
	if got := DeployBucketForProject(m, "missing"); got != "" {
		t.Errorf("missing: got %q, want empty", got)
	}
}

func TestDeployNamespaceAndPath_ReadsManifest(t *testing.T) {
	m := &Manifest{
		Version: ManifestVersion,
		Workspace: &ManifestWorkspace{
			ID:   "demo-abc123",
			Name: "demo",
		},
		Domains: &WorkspaceDomains{
			Deploy: &BackendRef{
				Kind: "kustomize",
				Config: rawConfig(t, workspaceDeployConfigShape{
					Namespace:         "default",
					KustomizationPath: "k8s/overlays/prod",
				}),
			},
		},
	}
	if got := DeployNamespace(m); got != "default" {
		t.Errorf("namespace: got %q, want default", got)
	}
	if got := DeployKustomizationPath(m); got != "k8s/overlays/prod" {
		t.Errorf("path: got %q, want k8s/overlays/prod", got)
	}
	if got := DeployNamespace(&Manifest{
		Version: ManifestVersion,
		Workspace: &ManifestWorkspace{
			ID:   "demo-abc123",
			Name: "demo",
		},
	}); got != "demo-abc123" {
		t.Errorf("missing deploy namespace: got %q, want workspace id", got)
	}
	if got := DeployNamespace(&Manifest{Version: ManifestVersion}); got != "" {
		t.Errorf("missing deploy namespace and workspace id: got %q, want empty", got)
	}
}

func TestSetProjectContainerNamespace_PreservesOtherFields(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteManifest(tmp, &Manifest{
		Version: ManifestVersion,
		Projects: []ManifestProject{{
			Name:        "api",
			RelativeDir: "services/api",
			TemplateID:  "nestjs-api",
			Toolchain:   "node",
			Domains: &ProjectDomains{
				Container: &ProjectContainerOverride{
					Kind:  "acr",
					Image: "myorg/api:custom",
				},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetProjectContainerNamespace(tmp, "api", "acme-corp"); err != nil {
		t.Fatalf("SetProjectContainerNamespace: %v", err)
	}
	got, _ := ReadManifest(tmp)
	c := got.Projects[0].Domains.Container
	if c == nil || c.Namespace != "acme-corp" {
		t.Errorf("namespace not written: %+v", c)
	}
	if c.Image != "myorg/api:custom" || c.Kind != "acr" {
		t.Errorf("existing container fields lost: %+v", c)
	}
}

func TestSetProjectContainerNamespace_CreatesSectionIfMissing(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteManifest(tmp, &Manifest{
		Version: ManifestVersion,
		Projects: []ManifestProject{{
			Name:        "api",
			RelativeDir: "services/api",
			TemplateID:  "nestjs-api",
			Toolchain:   "node",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetProjectContainerNamespace(tmp, "api", "acme-corp"); err != nil {
		t.Fatalf("SetProjectContainerNamespace: %v", err)
	}
	got, _ := ReadManifest(tmp)
	if got.Projects[0].Domains == nil || got.Projects[0].Domains.Container == nil {
		t.Fatalf("container section not created")
	}
	if got.Projects[0].Domains.Container.Namespace != "acme-corp" {
		t.Errorf("namespace not written: %+v", got.Projects[0].Domains.Container)
	}
}

func TestSetProjectDeployBucket_RequiresExistingDeploySection(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteManifest(tmp, &Manifest{
		Version: ManifestVersion,
		Projects: []ManifestProject{{
			Name:        "web",
			RelativeDir: "apps/web",
			TemplateID:  "web-vite",
			Toolchain:   "node",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	// No deploy section yet -> bucket setter should refuse rather than
	// silently create an empty deploy block.
	if err := SetProjectDeployBucket(tmp, "web", "web-prod"); err == nil {
		t.Errorf("expected error when deploy section is missing")
	}
}

func TestSetProjectDeployBucket_PreservesOtherFields(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteManifest(tmp, &Manifest{
		Version: ManifestVersion,
		Projects: []ManifestProject{{
			Name:        "web",
			RelativeDir: "apps/web",
			TemplateID:  "web-vite",
			Toolchain:   "node",
			Domains: &ProjectDomains{
				Deploy: &ProjectDeployBackend{
					Kind: "aws-s3",
				},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetProjectDeployBucket(tmp, "web", "web-prod"); err != nil {
		t.Fatalf("SetProjectDeployBucket: %v", err)
	}
	got, _ := ReadManifest(tmp)
	d := got.Projects[0].Domains.Deploy
	if d.Kind != "aws-s3" {
		t.Errorf("existing deploy fields lost: %+v", d)
	}
	if got := ExplicitDeployBucketForProject(got, "web"); got != "web-prod" {
		t.Errorf("bucket not written: %+v", d)
	}
}

func TestSetWorkspaceDeployTarget_CreatesSection(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteManifest(tmp, &Manifest{
		Version:  ManifestVersion,
		Projects: []ManifestProject{},
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetWorkspaceDeployTarget(tmp, "default", "kustomize/overlays/prod"); err != nil {
		t.Fatalf("SetWorkspaceDeployTarget: %v", err)
	}
	got, _ := ReadManifest(tmp)
	if got.Domains == nil || got.Domains.Deploy == nil {
		t.Fatalf("deploy section not created")
	}
	if ns := DeployNamespace(got); ns != "default" {
		t.Errorf("namespace = %q, want default", ns)
	}
	if path := DeployKustomizationPath(got); path != "kustomize/overlays/prod" {
		t.Errorf("path = %q", path)
	}
}

func TestSetWorkspaceDeployTarget_PreservesKind(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteManifest(tmp, &Manifest{
		Version:  ManifestVersion,
		Projects: []ManifestProject{},
		Domains: &WorkspaceDomains{
			Deploy: &BackendRef{Kind: "kustomize"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetWorkspaceDeployTarget(tmp, "staging", "k8s/overlays/staging"); err != nil {
		t.Fatalf("SetWorkspaceDeployTarget: %v", err)
	}
	got, _ := ReadManifest(tmp)
	if got.Domains == nil || got.Domains.Deploy == nil || got.Domains.Deploy.Kind != "kustomize" {
		t.Errorf("deploy kind lost: %+v", got.Domains)
	}
}
