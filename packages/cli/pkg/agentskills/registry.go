package agentskills

// Agent describes a single coding-agent skill consumer. Each agent
// has a stable id, a human-readable display name, and the
// per-project + per-machine paths where SKILL.md directories should be
// installed. DetectPath, when non-empty, is the marker directory
// whose existence implies the agent is installed on this machine.
type Agent struct {
	ID          string
	DisplayName string
	ProjectPath string // relative to workspace root
	GlobalPath  string // home-relative ("~/" prefix at runtime)
	DetectPath  string // home-relative; empty means "no auto-detection"
}

// agents is the canonical agent table. Mirrors the supported-agents
// section of vercel-labs/skills' README (commit-pinned by humans, not
// scraped — when their list changes, run `task sync-agent-paths`).
//
// Rows where multiple agent ids share the same path triple are split
// here into one entry per id so callers can target each individually.
var agents = []Agent{
	{ID: "aider-desk", DisplayName: "AiderDesk",
		ProjectPath: ".aider-desk/skills/", GlobalPath: "~/.aider-desk/skills/",
		DetectPath: "~/.aider-desk"},

	// ---- Shared `.agents/skills/` project + `~/.config/agents/skills/` global
	{ID: "amp", DisplayName: "Amp",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.config/agents/skills/",
		DetectPath: ""},
	{ID: "kimi-cli", DisplayName: "Kimi Code CLI",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.config/agents/skills/",
		DetectPath: ""},
	{ID: "replit", DisplayName: "Replit",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.config/agents/skills/",
		DetectPath: ""},
	{ID: "universal", DisplayName: "Universal",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.config/agents/skills/",
		DetectPath: ""},

	{ID: "antigravity", DisplayName: "Antigravity",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.gemini/antigravity/skills/",
		DetectPath: "~/.gemini/antigravity"},

	{ID: "augment", DisplayName: "Augment",
		ProjectPath: ".augment/skills/", GlobalPath: "~/.augment/skills/",
		DetectPath: "~/.augment"},

	{ID: "bob", DisplayName: "IBM Bob",
		ProjectPath: ".bob/skills/", GlobalPath: "~/.bob/skills/",
		DetectPath: "~/.bob"},

	{ID: "claude-code", DisplayName: "Claude Code",
		ProjectPath: ".claude/skills/", GlobalPath: "~/.claude/skills/",
		DetectPath: "~/.claude"},

	{ID: "openclaw", DisplayName: "OpenClaw",
		ProjectPath: "skills/", GlobalPath: "~/.openclaw/skills/",
		DetectPath: "~/.openclaw"},

	// ---- Shared `.agents/skills/` project + `~/.agents/skills/` global
	{ID: "cline", DisplayName: "Cline",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.agents/skills/",
		DetectPath: "~/.agents"},
	{ID: "dexto", DisplayName: "Dexto",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.agents/skills/",
		DetectPath: ""},
	{ID: "warp", DisplayName: "Warp",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.agents/skills/",
		DetectPath: ""},

	{ID: "codearts-agent", DisplayName: "CodeArts Agent",
		ProjectPath: ".codeartsdoer/skills/", GlobalPath: "~/.codeartsdoer/skills/",
		DetectPath: "~/.codeartsdoer"},

	{ID: "codebuddy", DisplayName: "CodeBuddy",
		ProjectPath: ".codebuddy/skills/", GlobalPath: "~/.codebuddy/skills/",
		DetectPath: "~/.codebuddy"},

	{ID: "codemaker", DisplayName: "Codemaker",
		ProjectPath: ".codemaker/skills/", GlobalPath: "~/.codemaker/skills/",
		DetectPath: "~/.codemaker"},

	{ID: "codestudio", DisplayName: "Code Studio",
		ProjectPath: ".codestudio/skills/", GlobalPath: "~/.codestudio/skills/",
		DetectPath: "~/.codestudio"},

	{ID: "codex", DisplayName: "Codex",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.codex/skills/",
		DetectPath: "~/.codex"},

	{ID: "command-code", DisplayName: "Command Code",
		ProjectPath: ".commandcode/skills/", GlobalPath: "~/.commandcode/skills/",
		DetectPath: "~/.commandcode"},

	{ID: "continue", DisplayName: "Continue",
		ProjectPath: ".continue/skills/", GlobalPath: "~/.continue/skills/",
		DetectPath: "~/.continue"},

	{ID: "cortex", DisplayName: "Cortex Code",
		ProjectPath: ".cortex/skills/", GlobalPath: "~/.snowflake/cortex/skills/",
		DetectPath: "~/.snowflake/cortex"},

	{ID: "crush", DisplayName: "Crush",
		ProjectPath: ".crush/skills/", GlobalPath: "~/.config/crush/skills/",
		DetectPath: "~/.config/crush"},

	{ID: "cursor", DisplayName: "Cursor",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.cursor/skills/",
		DetectPath: "~/.cursor"},

	{ID: "deepagents", DisplayName: "Deep Agents",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.deepagents/agent/skills/",
		DetectPath: "~/.deepagents"},

	{ID: "devin", DisplayName: "Devin for Terminal",
		ProjectPath: ".devin/skills/", GlobalPath: "~/.config/devin/skills/",
		DetectPath: "~/.config/devin"},

	{ID: "droid", DisplayName: "Droid",
		ProjectPath: ".factory/skills/", GlobalPath: "~/.factory/skills/",
		DetectPath: "~/.factory"},

	{ID: "firebender", DisplayName: "Firebender",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.firebender/skills/",
		DetectPath: "~/.firebender"},

	{ID: "forgecode", DisplayName: "ForgeCode",
		ProjectPath: ".forge/skills/", GlobalPath: "~/.forge/skills/",
		DetectPath: "~/.forge"},

	{ID: "gemini-cli", DisplayName: "Gemini CLI",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.gemini/skills/",
		DetectPath: "~/.gemini"},

	{ID: "github-copilot", DisplayName: "GitHub Copilot",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.copilot/skills/",
		DetectPath: "~/.copilot"},

	{ID: "goose", DisplayName: "Goose",
		ProjectPath: ".goose/skills/", GlobalPath: "~/.config/goose/skills/",
		DetectPath: "~/.config/goose"},

	{ID: "junie", DisplayName: "Junie",
		ProjectPath: ".junie/skills/", GlobalPath: "~/.junie/skills/",
		DetectPath: "~/.junie"},

	{ID: "iflow-cli", DisplayName: "iFlow CLI",
		ProjectPath: ".iflow/skills/", GlobalPath: "~/.iflow/skills/",
		DetectPath: "~/.iflow"},

	{ID: "kilo", DisplayName: "Kilo Code",
		ProjectPath: ".kilocode/skills/", GlobalPath: "~/.kilocode/skills/",
		DetectPath: "~/.kilocode"},

	{ID: "kiro-cli", DisplayName: "Kiro CLI",
		ProjectPath: ".kiro/skills/", GlobalPath: "~/.kiro/skills/",
		DetectPath: "~/.kiro"},

	{ID: "kode", DisplayName: "Kode",
		ProjectPath: ".kode/skills/", GlobalPath: "~/.kode/skills/",
		DetectPath: "~/.kode"},

	{ID: "mcpjam", DisplayName: "MCPJam",
		ProjectPath: ".mcpjam/skills/", GlobalPath: "~/.mcpjam/skills/",
		DetectPath: "~/.mcpjam"},

	{ID: "mistral-vibe", DisplayName: "Mistral Vibe",
		ProjectPath: ".vibe/skills/", GlobalPath: "~/.vibe/skills/",
		DetectPath: "~/.vibe"},

	{ID: "mux", DisplayName: "Mux",
		ProjectPath: ".mux/skills/", GlobalPath: "~/.mux/skills/",
		DetectPath: "~/.mux"},

	{ID: "opencode", DisplayName: "OpenCode",
		ProjectPath: ".agents/skills/", GlobalPath: "~/.config/opencode/skills/",
		DetectPath: "~/.config/opencode"},

	{ID: "openhands", DisplayName: "OpenHands",
		ProjectPath: ".openhands/skills/", GlobalPath: "~/.openhands/skills/",
		DetectPath: "~/.openhands"},

	{ID: "pi", DisplayName: "Pi",
		ProjectPath: ".pi/skills/", GlobalPath: "~/.pi/agent/skills/",
		DetectPath: "~/.pi"},

	{ID: "qoder", DisplayName: "Qoder",
		ProjectPath: ".qoder/skills/", GlobalPath: "~/.qoder/skills/",
		DetectPath: "~/.qoder"},

	{ID: "qwen-code", DisplayName: "Qwen Code",
		ProjectPath: ".qwen/skills/", GlobalPath: "~/.qwen/skills/",
		DetectPath: "~/.qwen"},

	{ID: "rovodev", DisplayName: "Rovo Dev",
		ProjectPath: ".rovodev/skills/", GlobalPath: "~/.rovodev/skills/",
		DetectPath: "~/.rovodev"},

	{ID: "roo", DisplayName: "Roo Code",
		ProjectPath: ".roo/skills/", GlobalPath: "~/.roo/skills/",
		DetectPath: "~/.roo"},

	{ID: "tabnine-cli", DisplayName: "Tabnine CLI",
		ProjectPath: ".tabnine/agent/skills/", GlobalPath: "~/.tabnine/agent/skills/",
		DetectPath: "~/.tabnine"},

	{ID: "trae", DisplayName: "Trae",
		ProjectPath: ".trae/skills/", GlobalPath: "~/.trae/skills/",
		DetectPath: "~/.trae"},

	{ID: "trae-cn", DisplayName: "Trae CN",
		ProjectPath: ".trae/skills/", GlobalPath: "~/.trae-cn/skills/",
		DetectPath: "~/.trae-cn"},

	{ID: "windsurf", DisplayName: "Windsurf",
		ProjectPath: ".windsurf/skills/", GlobalPath: "~/.codeium/windsurf/skills/",
		DetectPath: "~/.codeium/windsurf"},

	{ID: "zencoder", DisplayName: "Zencoder",
		ProjectPath: ".zencoder/skills/", GlobalPath: "~/.zencoder/skills/",
		DetectPath: "~/.zencoder"},

	{ID: "neovate", DisplayName: "Neovate",
		ProjectPath: ".neovate/skills/", GlobalPath: "~/.neovate/skills/",
		DetectPath: "~/.neovate"},

	{ID: "pochi", DisplayName: "Pochi",
		ProjectPath: ".pochi/skills/", GlobalPath: "~/.pochi/skills/",
		DetectPath: "~/.pochi"},

	{ID: "adal", DisplayName: "AdaL",
		ProjectPath: ".adal/skills/", GlobalPath: "~/.adal/skills/",
		DetectPath: "~/.adal"},
}

// All returns every registered agent in the same order as the
// upstream README. Callers should treat the returned slice as
// read-only.
func All() []Agent {
	return agents
}

// GetByID returns the agent with the given id, or false if none
// matches.
func GetByID(id string) (Agent, bool) {
	for _, a := range agents {
		if a.ID == id {
			return a, true
		}
	}
	return Agent{}, false
}
