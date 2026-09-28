package workspace

// Resolve initial development commands when scaffolding. Task adapters read
// the manifest override on each invocation and fall back to native dev tasks.

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveDevCommand picks the dev command for a freshly-scaffolded
// subproject. For Node-style projects (anything that ships a non-empty
// scripts map) it tries the conventional script names in priority
// order: `dev` (most templates) → `start:dev` (NestJS) → `start`
// (Expo / generic). Falls back to a Go runner for the Go toolchain.
//
// Returns "" when nothing resolves — the caller MUST NOT persist a
// placeholder; an empty Command allows native dev task discovery.
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

// ResolveScaffoldDevCommand applies the generic command heuristic to a
// freshly rendered project and verifies toolchain-specific entrypoints that
// cannot be inferred from package scripts. In particular, Go libraries and
// Go services share a toolchain, but only services contain cmd/server.
func ResolveScaffoldDevCommand(scripts map[string]string, toolchain, projectDir string) string {
	command := ResolveDevCommand(scripts, toolchain)
	if command == "" || toolchain != "go" {
		return command
	}
	info, err := os.Stat(filepath.Join(projectDir, "cmd", "server"))
	if err != nil || !info.IsDir() {
		return ""
	}
	return command
}
