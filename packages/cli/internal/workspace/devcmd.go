package workspace

// devcmd.go owns the script-name → dev-command heuristic used at
// `one add` time. The resolved string is persisted to
// projects[].domains.dev.command in the manifest; `one dev` later reads
// the manifest directly without re-scanning package.json. Keeping this
// logic here (rather than under the dev-runner package) lets future
// `one add` variants and migration scripts call into the same source
// of truth without pulling in the supervisor implementation.

import "strings"

// ResolveDevCommand picks the dev command for a freshly-scaffolded
// subproject. For Node-style projects (anything that ships a non-empty
// scripts map) it tries the conventional script names in priority
// order: `dev` (most templates) → `start:dev` (NestJS) → `start`
// (Expo / generic). Falls back to a Go runner for the Go toolchain.
//
// Returns "" when nothing resolves — the caller MUST NOT persist a
// placeholder; an empty Command in the manifest means "this project
// does not participate in `one dev`".
func ResolveDevCommand(scripts map[string]string, toolchain string) string {
	for _, key := range []string{"dev", "start:dev", "start"} {
		if v, ok := scripts[key]; ok && strings.TrimSpace(v) != "" {
			return "pnpm run " + key
		}
	}
	if toolchain == "go" {
		return "go run ./cmd/server"
	}
	return ""
}
