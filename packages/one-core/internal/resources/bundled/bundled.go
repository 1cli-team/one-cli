// Package bundled embeds templates, the template registry and agent skills.
// Generate with mise run sync-bundled; desktop assets belong to Tauri.
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
