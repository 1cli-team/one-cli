// Package deploy defines the Provider interface and a process-global
// registry that deploycmd dispatches through. Each deploy backend
// (kustomize / s3 / vercel / ...) lives in its own infra/<name>
// package and registers itself in `func init()` so adding a new
// backend never touches deploycmd.go's switch — drop the package
// in, blank-import it from deploycmd, and `manifest.projects[i].deploy.target`
// recognises the new id.
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

	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// Provider is one deploy backend. ID returns the bare backend name
// (e.g. "kustomize") that appears in manifest.projects[i].deploy.target;
// Apply executes the deploy.
type Provider interface {
	ID() string
	Apply(ctx context.Context, in ApplyInput) (*ApplyResult, error)
}

// ApplyInput carries everything deploycmd resolved before dispatch:
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

var registry = map[string]Provider{}

// Register makes p available to deploycmd via Get(p.ID()). Called from
// each provider package's init(). Re-registering the same id panics —
// that's a build-side bug (two packages claiming "vercel"), not a
// runtime condition.
func Register(p Provider) {
	if p == nil {
		panic("deploy: Register(nil)")
	}
	id := p.ID()
	if id == "" {
		panic("deploy: Register provider with empty ID")
	}
	if _, exists := registry[id]; exists {
		panic(fmt.Sprintf("deploy: provider %q already registered", id))
	}
	registry[id] = p
}

// Get returns the provider registered for id, plus a found flag.
func Get(id string) (Provider, bool) {
	p, ok := registry[id]
	return p, ok
}

// IDs returns the registered backend names sorted alphabetically. Used
// by deploycmd's help text + by the `one add` interactive backend
// picker so the order is stable across runs.
func IDs() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
