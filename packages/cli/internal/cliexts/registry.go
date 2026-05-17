// Package cliexts is the in-process registry that lets per-domain
// modules contribute top-level subcommands to `one` without
// forcing the cli harness (internal/cli) to import them directly.
//
// Why this exists: the cli package already imports each domain module
// for side-effects (registering into pkg/infra, pkg/ci, ...). Adding a
// "and also mount your top-level command" call would require either
// editing root.go for every new family, or having every family expose
// a per-family hook that root.go enumerates. Both create a centralised
// list that is easy to forget to update.
//
// cliexts inverts the dependency: the cli harness calls Mount() once
// at Execute() time, and any package that wants to add a top-level
// command does so by calling Register() in its own init(). The harness
// has no per-domain awareness.
//
// Layering rule: cliexts is internal-only on purpose. It pulls in
// spf13/cobra, which is a CLI-framework choice we do not want to
// leak through pkg/. Future out-of-process extensions (if ever added) will get
// their own protocol; this is for compiled-in contributors.
package cliexts

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// Contributor is the registration callback. It returns the top-level
// commands the contributor wants mounted on root. Returning multiple
// is allowed but discouraged — typically a contributor owns one
// top-level name and arranges its subcommands underneath.
//
// Contributor is invoked exactly once, at Mount() time. It must not
// rely on workspace state (cwd, env vars) — that resolution happens
// at command execution time inside the returned cobra.Command.RunE.
type Contributor func() []*cobra.Command

// registration captures one Register() call so we can attribute
// duplicate-name conflicts to a useful source identifier.
type registration struct {
	source string
	build  Contributor
}

var registry []registration

// Register enrols a contributor. Call this from an init() in the
// package that owns the command. source is a free-form identifier
// surfaced in conflict diagnostics — convention is the package's
// import path tail (e.g. "secrets/infisical", "infra/docker") or
// the family name for family-level commands ("templates").
//
// Register is intentionally not safe for concurrent calls: Go's init
// phase is single-threaded by spec, and Mount() is the only consumer.
func Register(source string, c Contributor) {
	registry = append(registry, registration{source: source, build: c})
}

// Mount realises every registration onto root. Call this exactly
// once, after init() has settled (typically inside cli.Execute()).
//
// Conflict policy: two contributors claiming the same top-level name
// is a programmer error, not user input. Mount returns a non-nil
// error so the harness can surface it via the structured error
// envelope rather than panicking. The conflict report names both
// contributors so the source of the collision is obvious.
//
// Mount preserves a stable order across runs by sorting
// contributors by source before iterating. Without this, command
// listing in --help would shift with init-order quirks across Go
// versions.
func Mount(root *cobra.Command) error {
	sorted := make([]registration, len(registry))
	copy(sorted, registry)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].source < sorted[j].source
	})

	owners := map[string]string{}
	for _, r := range sorted {
		for _, cmd := range r.build() {
			name := cmd.Name()
			if owner, dup := owners[name]; dup {
				return fmt.Errorf(
					"cliexts: top-level command %q claimed by both %s and %s",
					name, owner, r.source,
				)
			}
			owners[name] = r.source
			root.AddCommand(cmd)
		}
	}
	return nil
}

// Reset drops every registration. Test-only: lets per-test setup
// install a fresh registry without leaking state across tests in
// the same package. Production code MUST NOT call this.
func Reset() { registry = nil }
