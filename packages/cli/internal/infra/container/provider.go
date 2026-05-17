// Package container defines the Provider interface and a process-global
// registry that containercmd dispatches through. Each container backend
// kind (docker / dockerhub / ghcr / acr-aliyun today; more cloud
// registries tomorrow) registers itself in `func init()` so adding a
// new kind never touches containercmd's wiring — drop the registration
// in, and the Provider becomes available via container.Get(id).
//
// Mirrors infra/deploy/provider.go's split:
//   - Registry / I/O types live here, not in the backend package.
//   - Register/Get/IDs are process-global, populated at init() time.
//   - Backend kinds implement Provider, blank-imported by containercmd.
//
// Today there is one concrete backend package (`infra/docker`) that
// registers four IDs ("docker", "dockerhub", "ghcr", "acr") via the
// s3compat-style parameterized providerImpl{kind} pattern. The four
// kinds share the docker CLI as their transport; they only differ in
// how `host` and default `namespace` are derived from the resolved
// profile.
package container

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Provider is one container backend. ID returns the bare backend kind
// ("docker" / "dockerhub" / "ghcr" / "acr") that namespaces this
// backend in profile domains and manifest fields. Info / Build / Push
// correspond 1:1 to the `one container <verb>` subcommands.
type Provider interface {
	ID() string
	Info(ctx context.Context, in InfoInput) (*InfoResult, error)
	Build(ctx context.Context, in BuildInput) (*BuildResult, error)
	Push(ctx context.Context, in PushInput) (*PushResult, error)
}

// Registry is the resolved OCI registry endpoint + credentials that
// Build / Push compose image tags against. Empty Registry.Registry
// means "use the local container daemon and the bare workload tag" —
// `one container build` falls back to that local path when no profile
// is configured; push paths require a populated Registry.
//
// Namespace is the org / team prefix between the host and the
// workload name. Production code reads per-project namespace from
// projects[i].container.namespace; this struct's Namespace field is
// the fallback resolved from the profile (kind-specific default rule
// applies — see docker.ResolveRegistry).
//
// ProfileName / ProfileSource carry the resolved profile metadata for
// remediation messages on `docker login` failures (so the user is
// pointed at the right `one configure` command).
type Registry struct {
	Registry      string // host, no scheme
	Namespace     string
	Username      string
	Password      string
	ProfileName   string
	ProfileSource string
}

// HasCredentials reports whether the registry is fully populated for a
// `docker login`. Build / Push run their login flow only when this
// returns true; users who already authenticated manually leave
// Username/Password empty and rely on the daemon's existing auth.
func (r *Registry) HasCredentials() bool {
	return r != nil && r.Registry != "" && r.Username != "" && r.Password != ""
}

// InfoInput addresses Info.
type InfoInput struct {
	ProjectRoot string
	TargetNames []string
}

// ProjectInfo is a single line of Info output: where the Dockerfile
// is, whether it exists, and the conventional workload name + image
// override (if any).
type ProjectInfo struct {
	Name          string `json:"name"`
	RelativeDir   string `json:"relative_dir"`
	Backend       string `json:"backend,omitempty"`
	HasArtifact   bool   `json:"has_artifact"`
	ArtifactPath  string `json:"artifact_path,omitempty"`
	WorkloadName  string `json:"workload_name,omitempty"`
	ImageOverride string `json:"image_override,omitempty"`
}

// InfoResult is the full Info envelope (workspace-scoped).
type InfoResult struct {
	Schema           string        `json:"schema"`
	Workspace        string        `json:"workspace"`
	ContainerBackend string        `json:"container_backend"`
	Projects         []ProjectInfo `json:"projects"`
}

// BuildInput addresses Build.
type BuildInput struct {
	ProjectRoot string
	Project     string
	TargetNames []string
	Tag         string
	Platform    string
	DryRun      bool

	// Registry, when non-nil, prefixes the image tag with
	// `<registry>/[<namespace>/]` and (if HasCredentials) runs the
	// backend's login flow before the first build. Nil keeps a local-
	// build fallback: bare `<workload>:<version>` tag, no login.
	Registry *Registry
}

// BuildEntry is one (project → image → argv) record produced by Build.
type BuildEntry struct {
	Project string   `json:"project"`
	Image   string   `json:"image"`
	Argv    []string `json:"argv"`
	DryRun  bool     `json:"dry_run"`
}

// BuildResult is the Build envelope.
type BuildResult struct {
	Schema string       `json:"schema"`
	Built  []BuildEntry `json:"built"`
}

// PushInput addresses Push. Registry is required (no point pushing to
// a bare workload tag); Registry.Registry must be non-empty.
type PushInput struct {
	ProjectRoot string
	Project     string
	TargetNames []string
	Tag         string
	DryRun      bool
	Registry    *Registry
}

// PushEntry is one (project → image → argv) record produced by Push.
type PushEntry struct {
	Project     string   `json:"project"`
	Image       string   `json:"image"`
	SourceImage string   `json:"source_image,omitempty"`
	Retagged    bool     `json:"retagged,omitempty"`
	Argv        []string `json:"argv"`
	DryRun      bool     `json:"dry_run"`
}

// PushResult is the Push envelope.
type PushResult struct {
	Schema string      `json:"schema"`
	Pushed []PushEntry `json:"pushed"`
}

var registry = map[string]Provider{}

// Register makes p available to containercmd via Get(p.ID()). Called
// from each provider package's init(). Re-registering the same id
// panics — that's a build-side bug (two packages claiming "docker"),
// not a runtime condition.
func Register(p Provider) {
	if p == nil {
		panic("container: Register(nil)")
	}
	id := strings.TrimSpace(p.ID())
	if id == "" {
		panic("container: Register provider with empty ID")
	}
	if _, exists := registry[id]; exists {
		panic(fmt.Sprintf("container: provider %q already registered", id))
	}
	registry[id] = p
}

// Get returns the provider registered for id, plus a found flag.
func Get(id string) (Provider, bool) {
	p, ok := registry[id]
	return p, ok
}

// IDs returns the registered backend names sorted alphabetically.
func IDs() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
