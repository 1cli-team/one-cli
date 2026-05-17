// Package secrets is the cross-provider hook used by `one run` to
// inject secrets into a child process. The package itself defines no
// providers — it just holds the registry. Concrete providers live in
// sibling packages (internal/secrets/infisical, internal/secrets/dotenv)
// and register themselves at init time.
//
// What the registry IS:
//
//   - The single integration point for `one run`. Without this hook,
//     `one run` would have to import every secrets provider directly
//     and hardcode their priority order — exactly the design we
//     decided NOT to take with the user-facing commands (each
//     provider owns its own top-level command, no shared interface).
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
//   - It is NOT a public extension surface. Top-level
//     commands go through internal/cliexts. This registry only handles
//     "given a workspace + subproject + env name, return the KV map a
//     child process needs in its environment".
//
// Registration is by Priority. `one run --from auto` walks the
// registered loaders highest-priority first and uses the first whose
// Available() returns true. Explicit `--from <id>` skips priority
// and resolves directly via Find().
package secrets

import (
	"context"
	"sort"
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

var registered []Loader

// Register adds a loader to the registry. Call from init() in the
// loader's own package. Sort happens on every call so registration
// order doesn't matter — only Priority does.
func Register(l Loader) {
	registered = append(registered, l)
	sort.SliceStable(registered, func(i, j int) bool {
		return registered[i].Priority() > registered[j].Priority()
	})
}

// Find returns the loader with the given ID, or nil if none registered.
// Used to resolve explicit --from <id>.
func Find(id string) Loader {
	for _, l := range registered {
		if l.ID() == id {
			return l
		}
	}
	return nil
}

// PickAvailable returns the highest-priority loader whose Available()
// returns true for this workspace, or nil. Used by --from auto.
func PickAvailable(projectRoot string) Loader {
	for _, l := range registered {
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
func All() []Loader {
	out := make([]Loader, len(registered))
	copy(out, registered)
	return out
}

// Reset drops the registry. Test-only.
func Reset() { registered = nil }
