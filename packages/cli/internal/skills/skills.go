// Package skills is the public entrypoint `one create` uses to install
// the bundled agent skills (one-bootstrap / one-add-feature / one-fix /
// one-reference). Since Phase 5 it's a thin shim over internal/skill,
// preserving the legacy JSON shape (`installed_to`, `skill_count`) so
// the create command's payload stays compatible with existing agents
// and snapshot fixtures.
//
// New code should call internal/skill.Install directly.
package skills

import (
	"path/filepath"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/skill"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/agentskills"
)

// InstallResult describes what `Install` did. Embedded in the `create`
// command JSON payload under `skills`.
type InstallResult struct {
	InstalledTo []string `json:"installed_to"`
	SkillCount  int      `json:"skill_count"`
}

// Install installs the bundled skills for every coding agent detected
// on the user's machine, at global scope. Falls back to Claude Code
// when no agents are detected.
//
// Used by `one create` as a side effect on first workspace creation.
// For an explicit, user-driven install entrypoint that lets the caller
// pick which agents, see InstallTo.
//
// Installs always go through the canonical store at
// ~/.one/skills-store/ — multiple agents share one physical copy, each
// agent's directory holds a symlink (or a copy on Windows).
func Install() (*InstallResult, error) {
	agents := agentskills.Detect()
	if len(agents) == 0 {
		// A fresh machine with no agent installed yet still gets the
		// skills written to ~/.claude/skills/. Keeps the install
		// idempotent for users who later install Claude Code.
		fallback, ok := agentskills.GetByID("claude-code")
		if !ok {
			return nil, cliErrors.New(cliErrors.SKILLS_INSTALL_FAILED,
				"claude-code missing from agent registry (build error)")
		}
		agents = []agentskills.Agent{fallback}
	}

	return InstallTo(agents)
}

// InstallTo installs the bundled skills to the explicit list of
// agents. Used by `one skills install` to honour the user's interactive
// selection (or `--agent X` flags). Bypasses the auto-detect /
// fallback logic in Install — caller is responsible for resolving
// targets. Returns SKILLS_INSTALL_FAILED on any underlying error.
func InstallTo(agents []agentskills.Agent) (*InstallResult, error) {
	if len(agents) == 0 {
		return nil, cliErrors.New(cliErrors.SKILLS_INSTALL_FAILED,
			"no target agents")
	}
	res, err := skill.InstallBundled(agents, agentskills.ScopeGlobal, "")
	if err != nil {
		return nil, cliErrors.New(cliErrors.SKILLS_INSTALL_FAILED, err.Error())
	}

	// Compute the list of *unique* parent directories where skills
	// landed. Shape mirrors v0.2.x (single element per agent root)
	// so the create-command JSON envelope stays compatible.
	seen := map[string]bool{}
	parents := []string{}
	for _, e := range res.AgentEntries {
		agent, ok := agentskills.GetByID(e.AgentID)
		if !ok {
			continue
		}
		dest := agentskills.DestinationPath(agent, e.Scope, "")
		if dest == "" {
			continue
		}
		dest = filepath.Clean(dest)
		if !seen[dest] {
			seen[dest] = true
			parents = append(parents, dest)
		}
	}

	return &InstallResult{
		InstalledTo: parents,
		SkillCount:  len(res.Skills),
	}, nil
}
