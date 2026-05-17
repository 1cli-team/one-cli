package agentskills

import (
	"os"
	"path/filepath"
	"strings"
)

// expandHome resolves a "~/..." path against the current user's home
// directory. Returns the input unchanged when there's no leading "~/".
// Empty input returns "".
func expandHome(p string) string {
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "~/") && p != "~" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~/"))
}

// ResolvePath expands the given home-relative path against the user's
// home dir. Exposed for callers that build install destinations from
// agentskills.Agent.ProjectPath / GlobalPath.
func ResolvePath(p string) string {
	return expandHome(p)
}

// DestinationPath computes the absolute install directory for a given
// agent + scope + workspace. For ScopeProject, workspaceRoot is joined
// with the agent's project path. For ScopeGlobal, the agent's global
// path is expanded against the user's home directory.
func DestinationPath(a Agent, scope Scope, workspaceRoot string) string {
	switch scope {
	case ScopeProject:
		return filepath.Join(workspaceRoot, filepath.FromSlash(a.ProjectPath))
	case ScopeGlobal:
		return expandHome(a.GlobalPath)
	default:
		return ""
	}
}

// Detect returns every agent whose DetectPath exists on the local
// filesystem — i.e. agents that appear to be installed. Agents with
// an empty DetectPath are never returned (they can only be targeted
// via an explicit --agent flag).
//
// The returned slice preserves registration order.
func Detect() []Agent {
	out := []Agent{}
	for _, a := range agents {
		if a.DetectPath == "" {
			continue
		}
		path := expandHome(a.DetectPath)
		if path == "" {
			continue
		}
		if st, err := os.Stat(path); err == nil && st.IsDir() {
			out = append(out, a)
		}
	}
	return out
}
