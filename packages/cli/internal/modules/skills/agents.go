// Package skills installs the bundled one-cli skill into coding agents' user directories.
package skills

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/userdirs"
)

// Agent is a resolved user-level installation target.
type Agent struct {
	ID          string `json:"agent_id"`
	DisplayName string `json:"display_name"`
	GlobalPath  string `json:"global_path"`
}

type agentSpec struct {
	id, name, skillsPath, detectPath string
}

// Keep the previously supported IDs. Paths were checked against
// https://github.com/vercel-labs/skills/blob/main/src/agents.ts.
// Detection uses user configuration directories, independent of the workspace.
var agentSpecs = []agentSpec{
	{"aider-desk", "AiderDesk", ".aider-desk/skills", ".aider-desk"},
	{"amp", "Amp", ".config/agents/skills", ".config/amp"},
	{"kimi-cli", "Kimi Code CLI", ".agents/skills", ".kimi"},
	{"replit", "Replit", ".config/agents/skills", ""},
	{"universal", "Universal", ".config/agents/skills", ""},
	{"antigravity", "Antigravity", ".gemini/antigravity/skills", ".gemini/antigravity"},
	{"augment", "Augment", ".augment/skills", ".augment"},
	{"bob", "IBM Bob", ".bob/skills", ".bob"},
	{"claude-code", "Claude Code", ".claude/skills", ".claude"},
	{"openclaw", "OpenClaw", ".openclaw/skills", ".openclaw"},
	{"cline", "Cline", ".agents/skills", ".cline"},
	{"dexto", "Dexto", ".agents/skills", ".dexto"},
	{"warp", "Warp", ".agents/skills", ".warp"},
	{"codearts-agent", "CodeArts Agent", ".codeartsdoer/skills", ".codeartsdoer"},
	{"codebuddy", "CodeBuddy", ".codebuddy/skills", ".codebuddy"},
	{"codemaker", "Codemaker", ".codemaker/skills", ".codemaker"},
	{"codestudio", "Code Studio", ".codestudio/skills", ".codestudio"},
	{"codex", "Codex", ".codex/skills", ".codex"},
	{"command-code", "Command Code", ".commandcode/skills", ".commandcode"},
	{"continue", "Continue", ".continue/skills", ".continue"},
	{"cortex", "Cortex Code", ".snowflake/cortex/skills", ".snowflake/cortex"},
	{"crush", "Crush", ".config/crush/skills", ".config/crush"},
	{"cursor", "Cursor", ".cursor/skills", ".cursor"},
	{"deepagents", "Deep Agents", ".deepagents/agent/skills", ".deepagents"},
	{"devin", "Devin for Terminal", ".config/devin/skills", ".config/devin"},
	{"droid", "Droid", ".factory/skills", ".factory"},
	{"firebender", "Firebender", ".firebender/skills", ".firebender"},
	{"forgecode", "ForgeCode", ".forge/skills", ".forge"},
	{"gemini-cli", "Gemini CLI", ".gemini/skills", ".gemini"},
	{"github-copilot", "GitHub Copilot", ".copilot/skills", ".copilot"},
	{"goose", "Goose", ".config/goose/skills", ".config/goose"},
	{"junie", "Junie", ".junie/skills", ".junie"},
	{"iflow-cli", "iFlow CLI", ".iflow/skills", ".iflow"},
	{"kilo", "Kilo Code", ".kilo/skills", ".kilo"},
	{"kiro-cli", "Kiro CLI", ".kiro/skills", ".kiro"},
	{"kode", "Kode", ".kode/skills", ".kode"},
	{"mcpjam", "MCPJam", ".mcpjam/skills", ".mcpjam"},
	{"mistral-vibe", "Mistral Vibe", ".vibe/skills", ".vibe"},
	{"mux", "Mux", ".mux/skills", ".mux"},
	{"opencode", "OpenCode", ".config/opencode/skills", ".config/opencode"},
	{"openhands", "OpenHands", ".openhands/skills", ".openhands"},
	{"pi", "Pi", ".pi/agent/skills", ".pi"},
	{"qoder", "Qoder", ".qoder/skills", ".qoder"},
	{"qwen-code", "Qwen Code", ".qwen/skills", ".qwen"},
	{"rovodev", "Rovo Dev", ".rovodev/skills", ".rovodev"},
	{"roo", "Roo Code", ".roo/skills", ".roo"},
	{"tabnine-cli", "Tabnine CLI", ".tabnine/agent/skills", ".tabnine"},
	{"trae", "Trae", ".trae/skills", ".trae"},
	{"trae-cn", "Trae CN", ".trae-cn/skills", ".trae-cn"},
	{"windsurf", "Windsurf", ".codeium/windsurf/skills", ".codeium/windsurf"},
	{"zencoder", "Zencoder", ".zencoder/skills", ".zencoder"},
	{"neovate", "Neovate", ".neovate/skills", ".neovate"},
	{"pochi", "Pochi", ".pochi/skills", ".pochi"},
	{"adal", "AdaL", ".adal/skills", ".adal"},
}

// AgentIDs provides the same registry to help and target validation.
func AgentIDs() []string {
	ids := make([]string, 0, len(agentSpecs))
	for _, spec := range agentSpecs {
		ids = append(ids, spec.id)
	}
	return ids
}

// ResolveTargets validates every explicit ID before any write. Without IDs it
// selects agents whose configuration directories exist; there is no fallback.
func ResolveTargets(ids []string) ([]Agent, error) {
	home, err := userdirs.Home()
	if err != nil {
		return nil, err
	}
	available := make(map[string]agentSpec, len(agentSpecs))
	for _, spec := range agentSpecs {
		available[spec.id] = spec
	}
	selected := make([]agentSpec, 0, len(agentSpecs))
	if len(ids) > 0 {
		seen := map[string]bool{}
		for _, id := range ids {
			spec, ok := available[id]
			if !ok {
				return nil, i18n.Errorf("skills.agent_unknown", id)
			}
			if !seen[id] {
				selected = append(selected, spec)
				seen[id] = true
			}
		}
	} else {
		for _, spec := range agentSpecs {
			if spec.detectPath == "" {
				continue
			}
			marker, err := agentPath(home, spec.detectPath)
			if err != nil {
				return nil, err
			}
			info, err := os.Stat(marker)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, i18n.Errorf("skills.detect_failed", spec.id, err)
			}
			if info.IsDir() {
				selected = append(selected, spec)
			}
		}
	}
	out := make([]Agent, 0, len(selected))
	for _, spec := range selected {
		dest, err := agentPath(home, spec.skillsPath)
		if err != nil {
			return nil, err
		}
		out = append(out, Agent{ID: spec.id, DisplayName: spec.name, GlobalPath: dest})
	}
	if len(out) == 0 {
		return nil, i18n.Errorf("skills.agents_missing")
	}
	return out, nil
}

// Honor the agent's own configuration root, including XDG on all platforms.
func agentPath(home, relative string) (string, error) {
	for _, override := range []struct{ prefix, env string }{
		{".codex", "CODEX_HOME"}, {".claude", "CLAUDE_CONFIG_DIR"},
		{".config", "XDG_CONFIG_HOME"}, {".vibe", "VIBE_HOME"},
	} {
		if relative != override.prefix && !strings.HasPrefix(relative, override.prefix+"/") {
			continue
		}
		root := strings.TrimSpace(os.Getenv(override.env))
		if root == "" {
			break
		}
		if root == "~" {
			root = home
		} else if strings.HasPrefix(root, "~/") {
			root = filepath.Join(home, filepath.FromSlash(root[2:]))
		}
		if !filepath.IsAbs(root) {
			return "", i18n.Errorf("path.absolute_directory_required", override.env)
		}
		suffix := strings.TrimPrefix(strings.TrimPrefix(relative, override.prefix), "/")
		return filepath.Join(root, filepath.FromSlash(suffix)), nil
	}
	return filepath.Abs(filepath.Join(home, filepath.FromSlash(relative)))
}
