// Package bundled exposes the assets the CLI ships with: the template
// registry, the bundled Claude Code skills, the templates themselves,
// and the built `one serve` web UI.
//
// The files in this directory are physical copies of canonical sources
// elsewhere in the monorepo (packages/templates/, packages/skills/,
// apps/dashboard/dist/). Go's embed directive cannot traverse upward
// with "../" and rejects symlinks ("cannot embed irregular file"), so
// the copies have to live inside this package directory.
//
// The whole tree is gitignored. Two tasks regenerate it:
//   - `task sync-bundled` — cheap cp from packages/templates/ +
//     packages/skills/ to registry.json / skills/ / _templates/.
//   - `task sync-web`     — pnpm install + vite build of
//     apps/dashboard/ → _web/.
//
// Both run as deps of `task vet`, `task test`, and `task build`, so
// the normal Taskfile-driven workflow keeps the embed sources in sync
// without committing duplicate state. A fresh clone needs
// `task sync-bundled && task sync-web` once before the Go toolchain
// (gopls / direct `go build`) stops complaining about the missing
// embed paths.
//
// Never hand-edit the files inside this directory.
package bundled

import (
	"embed"
	_ "embed"
)

// RegistryBytes is the raw bytes of registry.json baked into the binary at
// build time. internal/template parses and validates this on first call.
//
//go:embed registry.json
var RegistryBytes []byte

// SkillsFS is the bundled Claude Code skills tree. internal/skills walks
// this filesystem to materialise ~/.claude/skills/ during `one create`.
// The "all:skills" prefix tells go:embed to include hidden files too —
// some skill manifests rely on dot-prefixed config layouts.
//
//go:embed all:skills
var SkillsFS embed.FS

// SkillsRoot is the path inside SkillsFS where skill subdirs live. Use this
// when calling fs.Sub etc. so callers don't hard-code the prefix.
const SkillsRoot = "skills"

// TemplatesFS is the bundled templates tree consumed by `one add` when the
// registry entry uses the local: prefix. internal/template walks this fs
// and renders handlebars files into the user's workspace.
//
// The directory is named "_templates" (leading underscore) so the Go toolchain
// skips it during `go build ./...` / `go test ./...` — the literal *.go files
// inside templates would otherwise fail to compile (they reference deps the
// orchestrator does not pull in: gin, gorm, viper, zap, etc.). The "all:"
// prefix tells go:embed to include the directory anyway, plus any hidden
// files inside (.gitkeep, .editorconfig, etc.).
//
//go:embed all:_templates
var TemplatesFS embed.FS

// TemplatesRoot is the prefix path inside TemplatesFS for the templates
// subtree. Use it as the base when constructing fs paths to a specific
// template directory.
const TemplatesRoot = "_templates"

// WebDistFS is the built React UI for `one serve` (sources at web/, built
// via `task build-web`, copied to internal/bundled/_web by `task sync-bundled`).
// internal/serve walks this filesystem to serve index.html + hashed assets.
//
// The directory is named "_web" (leading underscore) so the Go toolchain
// skips it during `go build ./...` / `go test ./...` — same rationale as
// _templates: the UI source includes TS/TSX that is meaningless to the Go
// toolchain. The "all:" prefix tells go:embed to include hidden files
// (Vite emits .vite/ for some setups).
//
//go:embed all:_web
var WebDistFS embed.FS

// WebDistRoot is the path inside WebDistFS where the built dist lives.
// Use it as the base for fs.Sub when wiring up the http file server.
const WebDistRoot = "_web"
