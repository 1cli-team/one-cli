// Package secrets is the cross-provider hook used by `one run` and
// `one deploy` to inject secrets into child processes. Concrete loaders are
// supplied to an instance Registry by the CLI composition root.
//
// What the registry IS:
//
//   - The single integration point for `one run`. Without this hook,
//     callers resolve a loader through one small, typed contract.
//
// What the registry is NOT:
//
//   - It's NOT a generic Provider interface that every secrets backend
//     must implement for init / set / get / list / pull. Those verbs
//     vary too much between backends (Infisical has projects + folders;
//     dotenv has files; Vault has secret engines and versioning) and
//     forcing a unified surface always shapes the abstraction around
//     the first implementation.
//
//   - It is NOT a public extension surface. Top-level commands are assembled
//     by the CLI composition root. This registry only handles
//     "given a workspace + subproject + env name, return the KV map a
//     child process needs in its environment".
//
// Ordering is by Priority. `one run --from auto` walks the registry's
// loaders highest-priority first and uses the first whose
// Available() returns true. Explicit `--from <id>` skips priority
// and resolves directly via Find().
package secrets

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Loader is the minimal contract every secrets backend implements
// for `one run` integration. Backends typically expose much richer
// CLI surfaces (init / set / get / pull) under their own top-level
// commands; this interface is intentionally just the run-injection
// path.
// Priority orders loaders for `--from auto`. Higher means "checked
// first". Provider authors should pick from the named constants below
// rather than typing magic numbers; the constants document intent and
// reserve gaps for future tiers.
type Priority int

// Reserved priority bands. Numeric values are stable across versions
// — provider authors and external consumers can rely on
// `PriorityRemoteBackend > PriorityFilesystem` ordering.
const (
	// PriorityRemoteBackend is for providers that reach out to a
	// remote secrets store (Infisical, Doppler, Vault, AWS Secrets
	// Manager). They typically gate on manifest configuration +
	// credentials being present in env.
	PriorityRemoteBackend Priority = 100

	// PriorityFilesystem is for providers that read from disk only
	// (dotenv). Filesystem providers should be available
	// unconditionally so `--from auto` always has a fallback.
	PriorityFilesystem Priority = 10
)

type Loader interface {
	// ID is the stable provider identifier ("infisical", "dotenv").
	// Used by --from <id> and surfaced in run output as the source.
	ID() string

	// Priority orders loaders for --from auto. Higher = checked first.
	// Implementations should return one of the named Priority constants
	// (PriorityRemoteBackend / PriorityFilesystem) rather than inventing
	// new numbers.
	Priority() Priority

	// Available reports whether this loader can serve a Load() call
	// for the given workspace right now. Should be cheap (no network
	// calls) — it's a gate for --from auto, not a healthcheck.
	Available(projectRoot string) bool

	// Load fetches the KV map for the given subproject. envName is a
	// provider-specific hint (Infisical environment name; dotenv ignores
	// it). Returns a structured cliErrors.Error on failure so the run
	// command can surface the standard envelope.
	Load(ctx context.Context, projectRoot, relativeDir, envName string) (map[string]string, error)
}

// Registry owns one command tree's loader set. It is immutable after
// construction and therefore safe to share between commands in that tree.
type Registry struct {
	loaders []Loader
}

func NewRegistry(loaders ...Loader) (*Registry, error) {
	seen := make(map[string]struct{}, len(loaders))
	copyOfLoaders := make([]Loader, 0, len(loaders))
	for _, loader := range loaders {
		if loader == nil {
			return nil, fmt.Errorf("secrets: nil loader")
		}
		id := strings.TrimSpace(loader.ID())
		if id == "" {
			return nil, fmt.Errorf("secrets: loader with empty ID")
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("secrets: loader %q already registered", id)
		}
		seen[id] = struct{}{}
		copyOfLoaders = append(copyOfLoaders, loader)
	}
	sort.SliceStable(copyOfLoaders, func(i, j int) bool {
		return copyOfLoaders[i].Priority() > copyOfLoaders[j].Priority()
	})
	return &Registry{loaders: copyOfLoaders}, nil
}

func MustRegistry(loaders ...Loader) *Registry {
	registry, err := NewRegistry(loaders...)
	if err != nil {
		panic(err)
	}
	return registry
}

// Find returns the loader with the given ID, or nil if it is not part of this
// registry. Used to resolve explicit --from <id>.
func (r *Registry) Find(id string) Loader {
	if r == nil {
		return nil
	}
	for _, l := range r.loaders {
		if l.ID() == id {
			return l
		}
	}
	return nil
}

// PickAvailable returns the highest-priority loader whose Available()
// returns true for this workspace, or nil. Used by --from auto.
func (r *Registry) PickAvailable(projectRoot string) Loader {
	if r == nil {
		return nil
	}
	for _, l := range r.loaders {
		if l.Available(projectRoot) {
			return l
		}
	}
	return nil
}

// All returns the registered loaders in priority order. Useful for
// `one run --help`-style introspection ("which providers does this
// build know about?"). Returns a copy so callers can't mutate the
// registry in place.
func (r *Registry) All() []Loader {
	if r == nil {
		return nil
	}
	out := make([]Loader, len(r.loaders))
	copy(out, r.loaders)
	return out
}
