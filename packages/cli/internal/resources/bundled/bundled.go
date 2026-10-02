// Package bundled exposes the assets the CLI ships with: the template
// registry, the templates themselves,
// development skills, and the built `one serve` web UI.
//
// The files in this directory are physical copies of canonical sources
// elsewhere in the monorepo (packages/templates/, packages/agent-skills/skills/,
// apps/dashboard/dist/). Go's embed directive cannot traverse upward
// with "../" and rejects symlinks ("cannot embed irregular file"), so
// the copies have to live inside this package directory.
//
// The whole tree is gitignored. Two tasks regenerate it:
//   - `mise run sync-bundled` — copy templates, registry.json, and skills.
//   - `mise run sync-web`     — pnpm install + vite build of
//     apps/dashboard/ → _web/.
//
// Both run as deps of `mise run vet`, `mise run test`, and `mise run build`, so
// the normal mise workflow keeps the embed sources in sync
// without committing duplicate state. A fresh clone needs
// `mise run sync-bundled ::: sync-web` once before the Go toolchain
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
// build time. internal/core/template parses and validates this on first call.
//
//go:embed registry.json
var RegistryBytes []byte

// TemplatesFS is the bundled templates tree consumed by `one add` when the
// registry entry uses the local: prefix. internal/core/template walks this fs
// and copies runnable starter projects into the user's workspace. Go module
// files travel as _go.mod so embed does not cross a nested module boundary.
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

// SkillsFS contains One's usage/layout and template adaptation skills, kept
// separately from runnable templates. Business skills are never bundled.
//
//go:embed all:_skills
var SkillsFS embed.FS

const SkillsRoot = "_skills"

// WebDistFS is the built React UI for `one serve` (sources at apps/dashboard,
// built via `mise run build-web`, copied into this package by `mise run sync-web`).
// internal/transport/http walks this filesystem to serve index.html + hashed assets.
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
