// Package deploy defines the deploy backend contract and an instance-scoped
// registry. Concrete adapters are assembled explicitly by the CLI composition
// root instead of registering themselves through package initialization.
//
// Why an interface instead of a switch
// ------------------------------------
// Two deploy backends fit fine in a switch. Five plus do not. The
// frontend-PaaS class (Vercel / Cloudflare Pages / EdgeOne / Netlify)
// shares a shape — credential profile + per-project id + shell-out to
// a vendor CLI — that benefits from one entry point per package. The
// interface deliberately stays narrow (ID + Apply) so providers do
// not implicitly grow capabilities like Sync / Validate; those stay
// private to each package.
package deploy

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

// Provider is one deploy backend. ID returns the bare backend name
// (e.g. "kustomize") that appears in manifest.projects[i].deploy.target;
// Apply executes the deploy.
type Provider interface {
	ID() string
	Apply(ctx context.Context, in ApplyInput) (*ApplyResult, error)
}

// ApplyInput carries everything the application workflow resolved before dispatch:
// project root, the project entry from the manifest, the resolved
// machine-level profile (nil when the backend doesn't need one), the
// fully-loaded manifest (so providers can read workspace-level fields
// like deploy.namespace without re-loading it), and TTY plumbing.
//
// Fields are intentionally additive: providers may ignore what they
// don't need. Backend-specific scratch data (kustomize's k8s endpoint,
// vercel's project pin) is read off `Manifest` and `Resolved` inside
// the provider, not threaded through here.
type ApplyInput struct {
	ProjectRoot string
	Project     workspace.Project
	Toolchain   string
	Manifest    *workspace.Manifest
	Resolved    *profile.Resolved
	DryRun      bool
	Stdout      io.Writer
	Stderr      io.Writer

	// InjectedEnv carries the project's user-set env vars (from
	// `one env set` → dotenv / Infisical), already resolved by deploycmd
	// at dispatch time. Providers that shell out to a vendor CLI should
	// merge this into cmd.Env before appending their own credential env
	// vars (so credentials always win). nil = no injection (--no-env,
	// no loader available, project-level disabled). Providers must be
	// nil-safe.
	InjectedEnv map[string]string

	// InjectedEnvSource is the secrets loader id ("dotenv" /
	// "infisical") that produced InjectedEnv. Empty when InjectedEnv
	// is nil. Used by providers to populate ApplyResult.InjectedEnvSource
	// for dry-run / wire output.
	InjectedEnvSource string
}

// ApplyResult is the JSON envelope every provider emits on success.
// Schema strings live with each provider so the wire format remains
// versionable per-backend.
type ApplyResult struct {
	Schema       string   `json:"schema"`
	Argv         []string `json:"argv"`
	CommandLines []string `json:"command_lines,omitempty"`
	DryRun       bool     `json:"dry_run"`

	// InjectedEnvKeys lists the KEY names (sorted) that the provider
	// merged into the deploy CLI's child environment. KEY names only —
	// VALUEs MUST NOT be emitted on the wire (dry-run output and JSON
	// envelopes are persisted in CI logs and could leak secrets).
	InjectedEnvKeys []string `json:"injected_env_keys,omitempty"`

	// InjectedEnvSource is the secrets loader id ("dotenv" /
	// "infisical") that produced the injected vars. Empty when nothing
	// was injected.
	InjectedEnvSource string `json:"injected_env_source,omitempty"`
}

type Registry struct {
	providers map[string]Provider
}

func NewRegistry(providers ...Provider) (*Registry, error) {
	registry := &Registry{providers: make(map[string]Provider, len(providers))}
	for _, provider := range providers {
		if provider == nil {
			return nil, fmt.Errorf("deploy: nil provider")
		}
		id := provider.ID()
		if id == "" {
			return nil, fmt.Errorf("deploy: provider with empty ID")
		}
		if _, exists := registry.providers[id]; exists {
			return nil, fmt.Errorf("deploy: provider %q already registered", id)
		}
		registry.providers[id] = provider
	}
	return registry, nil
}

func MustRegistry(providers ...Provider) *Registry {
	registry, err := NewRegistry(providers...)
	if err != nil {
		panic(err)
	}
	return registry
}

func (r *Registry) Get(id string) (Provider, bool) {
	if r == nil {
		return nil, false
	}
	provider, ok := r.providers[id]
	return provider, ok
}

func (r *Registry) IDs() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.providers))
	for id := range r.providers {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
