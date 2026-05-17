package agentskills_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/pkg/agentskills"
)

func TestAll_HasFamiliarAgents(t *testing.T) {
	all := agentskills.All()
	if len(all) < 40 {
		t.Fatalf("registry shrank: got %d agents, expected at least 40 (sync against vercel-labs/skills?)", len(all))
	}
	// Spot-check the agents we depend on most.
	required := []string{"claude-code", "codex", "cursor", "gemini-cli", "github-copilot"}
	for _, id := range required {
		if _, ok := agentskills.GetByID(id); !ok {
			t.Errorf("required agent %q missing from registry", id)
		}
	}
}

func TestGetByID_UnknownReturnsFalse(t *testing.T) {
	if a, ok := agentskills.GetByID("not-a-real-agent"); ok {
		t.Errorf("expected false for unknown id, got %+v", a)
	}
}

func TestDestinationPath_Project(t *testing.T) {
	a, _ := agentskills.GetByID("claude-code")
	got := agentskills.DestinationPath(a, agentskills.ScopeProject, "/workspace")
	want := filepath.Join("/workspace", ".claude", "skills") + string(filepath.Separator)
	// DestinationPath uses filepath.Join which normalises trailing
	// separators. We compare without the trailing slash.
	want = filepath.Clean(want)
	if filepath.Clean(got) != want {
		t.Errorf("project path = %q, want %q", got, want)
	}
}

func TestDestinationPath_Global(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("UserHomeDir failed: %v", err)
	}
	a, _ := agentskills.GetByID("claude-code")
	got := agentskills.DestinationPath(a, agentskills.ScopeGlobal, "/workspace")
	want := filepath.Join(home, ".claude", "skills")
	if got != want {
		t.Errorf("global path = %q, want %q", got, want)
	}
}

func TestDetect_FindsCreatedMarker(t *testing.T) {
	// Set HOME to a temp dir; create the .claude marker so Detect()
	// returns at least Claude Code.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := os.Mkdir(filepath.Join(tmp, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got := agentskills.Detect()
	found := false
	for _, a := range got {
		if a.ID == "claude-code" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Detect() did not surface claude-code despite ~/.claude existing; got %d agents", len(got))
	}
}

func TestDetect_EmptyHomeReturnsNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got := agentskills.Detect()
	if len(got) != 0 {
		ids := []string{}
		for _, a := range got {
			ids = append(ids, a.ID)
		}
		t.Errorf("Detect() on empty home returned %v", ids)
	}
}
