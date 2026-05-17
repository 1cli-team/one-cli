// Package agentskills mirrors the install-target conventions used by
// vercel-labs/skills (https://github.com/vercel-labs/skills) — the
// "open agent skills ecosystem" CLI. one-cli uses the same path table
// and scope semantics so a skill installed by one tool is visible to
// every agent the other tool supports.
//
// Stability: this package's exported surface is intended to be
// consumed by external tools (e.g. third-party install drivers).
// Refer to vercel-labs/skills' README for the canonical agent table;
// any additions there should be mirrored here via task sync-agent-paths.
package agentskills

// Scope distinguishes per-project install (committed with the repo)
// from per-machine install (available across all projects for the
// current user). vercel-labs/skills uses "project" as the default;
// one-cli's `one create` flow uses "global" as default because the
// bundled skills it installs (one-bootstrap, one-fix, ...) teach the
// agent how to use one-cli the tool, which is a per-machine concern.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeGlobal  Scope = "global"
)

// Method describes how a skill is materialised on disk.
//   - MethodSymlink (default): a single canonical copy lives in the
//     one-cli store; each agent's path is a symlink pointing at it.
//     One physical copy, instant updates across agents.
//   - MethodCopy: each agent path holds an independent physical copy.
//     Used as Windows fallback or when symlinks aren't supported.
type Method string

const (
	MethodSymlink Method = "symlink"
	MethodCopy    Method = "copy"
)
