package workspace

import (
	"encoding/json"
	"strings"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
)

// backend.go is the read-side projection for "what backend has this workspace
// selected for each domain". Identity remains owned by core/backend; the
// aliases below preserve existing workspace callers without another literal
// vocabulary.
//
// All helpers read the current manifest fields (Manifest.Domains, ManifestProject.Domains).

// Compatibility aliases for existing workspace callers.
const (
	EnvBackendDotenv    = catalog.EnvDotenv
	EnvBackendInfisical = catalog.EnvInfisical

	DeployBackendKustomize  = catalog.DeployKustomize
	DeployBackendAliyunOSS  = catalog.DeployAliyunOSS
	DeployBackendTencentCOS = catalog.DeployTencentCOS
	DeployBackendAWSS3      = catalog.DeployAWSS3
	DeployBackendMinIO      = catalog.DeployMinIO
	DeployBackendRustFS     = catalog.DeployRustFS
	DeployBackendR2         = catalog.DeployR2
	DeployBackendVercel     = catalog.DeployVercel
	DeployBackendCloudflare = catalog.DeployCloudflare
	DeployBackendEdgeOne    = catalog.DeployEdgeOne

	ContainerBackendDocker = catalog.ContainerDocker
)

// IsS3CompatibleDeploy reports whether `kind` names one of the
// S3-protocol-compatible deploy backends. All six share the same on-disk
// profile shape and the same Apply implementation in adapters/deploy/s3compat;
// only the user-facing id, defaults, and prompts differ.
func IsS3CompatibleDeploy(kind string) bool {
	spec, ok := catalog.Builtin().Lookup(catalog.DomainDeploy, kind)
	return ok && spec.HasTrait(catalog.TraitS3Compatible)
}

// EnvBackend returns the bare backend name selected for the workspace's
// env domain ("dotenv" / "infisical"), or "" if unset.
func EnvBackend(m *Manifest) string {
	if m == nil || m.Domains == nil || m.Domains.Env == nil {
		return ""
	}
	return m.Domains.Env.Kind
}

// EnvConfigRaw returns the raw JSON of the workspace env backend's
// kind-specific config, suitable for unmarshalling into a typed config
// struct (DotenvConfig / InfisicalConfig). Returns nil when no config has
// been written.
func EnvConfigRaw(m *Manifest) json.RawMessage {
	if m == nil || m.Domains == nil || m.Domains.Env == nil {
		return nil
	}
	return m.Domains.Env.Config
}

// DeploySelection is the deploy backend chosen for one project.
type DeploySelection struct {
	Backend string // bare backend name, e.g. "kustomize" or "s3"
}

// DeployForProject returns the deploy backend configured for a named
// project, or the zero value when nothing is configured.
func DeployForProject(m *Manifest, projectName string) DeploySelection {
	dep := projectDeploy(m, projectName)
	if dep == nil || dep.Kind == "" {
		return DeploySelection{}
	}
	return DeploySelection{Backend: dep.Kind}
}

// DeployConfigRawForProject returns the raw JSON of a project's deploy
// backend kind-specific config, suitable for unmarshalling into a typed
// per-kind config struct. Returns nil when no deploy section / no config
// has been written.
func DeployConfigRawForProject(m *Manifest, projectName string) json.RawMessage {
	dep := projectDeploy(m, projectName)
	if dep == nil {
		return nil
	}
	return dep.Config
}

// ContainerForProject reports whether the named project has a container
// backend configured (i.e. its Dockerfile is owned by this workspace).
// Also returns any per-project image override.
func ContainerForProject(m *Manifest, projectName string) (enabled bool, imageOverride string) {
	c := projectContainer(m, projectName)
	if c == nil {
		return false, ""
	}
	return true, c.Image
}

// ContainerKindForProject returns the container backend kind for the
// named project. Resolution order:
//  1. projects[i].domains.container.kind
//  2. manifest.domains.container.kind (workspace-level default)
//  3. ContainerBackendDocker ("docker") as the implicit fallback
//
// Empty / unknown values normalise to "docker" so callers can pass the
// result straight to the container service without nil-checking.
func ContainerKindForProject(m *Manifest, projectName string) string {
	if c := projectContainer(m, projectName); c != nil {
		if kind := strings.TrimSpace(c.Kind); kind != "" {
			return kind
		}
	}
	if m != nil && m.Domains != nil && m.Domains.Container != nil {
		if kind := strings.TrimSpace(m.Domains.Container.Kind); kind != "" {
			return kind
		}
	}
	return ContainerBackendDocker
}

// ContainerNamespaceForProject returns the registry namespace configured
// for the named project (projects[i].domains.container.namespace), or ""
// when unset. Used to compose the image tag
// `<registry>/[<namespace>/]<workload>:<version>`.
func ContainerNamespaceForProject(m *Manifest, projectName string) string {
	c := projectContainer(m, projectName)
	if c == nil {
		return ""
	}
	return c.Namespace
}

// ExplicitDeployBucketForProject returns the S3 bucket explicitly
// configured at projects[i].domains.deploy.config.bucket, or "" when
// unset.
func ExplicitDeployBucketForProject(m *Manifest, projectName string) string {
	dep := projectDeploy(m, projectName)
	if dep == nil || len(dep.Config) == 0 {
		return ""
	}
	cfg := struct {
		Bucket string `json:"bucket,omitempty"`
	}{}
	if err := json.Unmarshal(dep.Config, &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.Bucket)
}

// DeployBucketForProject returns the effective S3 bucket for the named
// project. Explicit projects[i].domains.deploy.config.bucket wins; when
// the project targets an S3-compatible deploy backend (aliyun-oss /
// tencent-cos / aws-s3 / minio / rustfs / r2) and no bucket is set,
// workspace.id is used so S3-backed templates do not need a second
// bucket prompt.
func DeployBucketForProject(m *Manifest, projectName string) string {
	dep := projectDeploy(m, projectName)
	if dep == nil || !IsS3CompatibleDeploy(dep.Kind) {
		return ""
	}
	if bucket := ExplicitDeployBucketForProject(m, projectName); bucket != "" {
		return bucket
	}
	return WorkspaceID(m)
}

// WorkspaceID returns the workspace identity id, or "" when older
// manifests have not been back-filled yet.
func WorkspaceID(m *Manifest) string {
	if m == nil || m.Workspace == nil {
		return ""
	}
	return strings.TrimSpace(m.Workspace.ID)
}

// ExplicitDeployNamespace returns the workspace-level k8s namespace
// configured at manifest.domains.deploy.config.namespace, or "" when
// unset.
func ExplicitDeployNamespace(m *Manifest) string {
	cfg := workspaceDeployConfig(m)
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Namespace)
}

// DeployNamespace returns the effective k8s namespace. An explicit
// manifest.domains.deploy.config.namespace wins; otherwise the
// workspace.id is used so new workspaces do not need a second namespace
// prompt.
func DeployNamespace(m *Manifest) string {
	if ns := ExplicitDeployNamespace(m); ns != "" {
		return ns
	}
	return WorkspaceID(m)
}

// DeployKustomizationPath returns the workspace-level kustomize overlay
// base path (manifest.domains.deploy.config.kustomizationPath), or "" when
// unset (callers fall back to the kustomize package's default).
func DeployKustomizationPath(m *Manifest) string {
	cfg := workspaceDeployConfig(m)
	if cfg == nil {
		return ""
	}
	return cfg.KustomizationPath
}

// ContainerPlatform returns the workspace-level target image platform
// used by `one container build`, e.g. "linux/amd64".
func ContainerPlatform(m *Manifest) string {
	cfg := workspaceContainerConfig(m)
	if cfg == nil {
		return ""
	}
	return cfg.Platform
}

// SelectionForProject collapses the workspace-level domain selections and
// any per-project container / deploy overrides into a single map keyed by
// domain ("container" / "deploy" / "env"). Empty values are dropped so
// callers can range without nil-checking. Used by infra.SyncProject to
// know which backend to run per domain. CI is not part of this map and is
// not generated implicitly; dev commands live on each project record.
//
// Values are namespaced ids ("env/dotenv", "deploy/kustomize", ...) for
// compatibility with the existing infra dispatch. The dispatch strips the
// prefix before switching on the bare name.
func SelectionForProject(m *Manifest, project *ManifestProject) map[string]string {
	out := map[string]string{}
	if m == nil {
		return out
	}
	if backend := EnvBackend(m); backend != "" {
		out["env"] = "env/" + backend
	}
	if project != nil {
		if enabled, _ := ContainerForProject(m, project.Name); enabled {
			out["container"] = "container/" + ContainerKindForProject(m, project.Name)
		}
		sel := DeployForProject(m, project.Name)
		if sel.Backend != "" {
			out["deploy"] = "deploy/" + sel.Backend
		}
	}
	return out
}

// findProject returns a pointer into m.Projects matching projectName, or
// nil when not found. Internal helper for the per-project read accessors
// below — keeps each helper compact.
func findProject(m *Manifest, projectName string) *ManifestProject {
	if m == nil {
		return nil
	}
	for i := range m.Projects {
		if m.Projects[i].Name == projectName {
			return &m.Projects[i]
		}
	}
	return nil
}

func projectDomains(m *Manifest, projectName string) *ProjectDomains {
	p := findProject(m, projectName)
	if p == nil {
		return nil
	}
	return p.Domains
}

func projectDeploy(m *Manifest, projectName string) *ProjectDeployBackend {
	d := projectDomains(m, projectName)
	if d == nil {
		return nil
	}
	return d.Deploy
}

func projectContainer(m *Manifest, projectName string) *ProjectContainerOverride {
	d := projectDomains(m, projectName)
	if d == nil {
		return nil
	}
	return d.Container
}

// ProjectEnv returns the per-project env override, or nil when unset.
// Exported because secrets backends (dotenv path resolution, infisical
// disabled-flag check) read it directly.
func ProjectEnv(m *Manifest, projectName string) *ProjectEnvOverride {
	d := projectDomains(m, projectName)
	if d == nil {
		return nil
	}
	return d.Env
}

// ProjectDev returns the dev command for projectName, or "" when there
// is no domains.dev block or its Command is empty. Used by `one dev` to
// build its supervisor entry list.
func ProjectDev(m *Manifest, projectName string) string {
	d := projectDomains(m, projectName)
	if d == nil || d.Dev == nil {
		return ""
	}
	return d.Dev.Command
}

// workspaceDeployConfig is the typed view over manifest.domains.deploy.config
// for fields the workspace-level kustomize backend cares about. Returns nil
// when no config blob has been written. Lives here (rather than in a
// per-backend package) because the deploy domain is the only one whose
// workspace-level config has shared fields used by multiple kinds (today
// only kustomize, but s3 deployment may eventually want a workspace-level
// namespace too).
type workspaceDeployConfigShape struct {
	Namespace         string `json:"namespace,omitempty"`
	KustomizationPath string `json:"kustomizationPath,omitempty"`
}

func workspaceDeployConfig(m *Manifest) *workspaceDeployConfigShape {
	if m == nil || m.Domains == nil || m.Domains.Deploy == nil || len(m.Domains.Deploy.Config) == 0 {
		return nil
	}
	var cfg workspaceDeployConfigShape
	if err := json.Unmarshal(m.Domains.Deploy.Config, &cfg); err != nil {
		return nil
	}
	return &cfg
}

type workspaceContainerConfigShape struct {
	Platform string `json:"platform,omitempty"`
}

func workspaceContainerConfig(m *Manifest) *workspaceContainerConfigShape {
	if m == nil || m.Domains == nil || m.Domains.Container == nil || len(m.Domains.Container.Config) == 0 {
		return nil
	}
	var cfg workspaceContainerConfigShape
	if err := json.Unmarshal(m.Domains.Container.Config, &cfg); err != nil {
		return nil
	}
	return &cfg
}
