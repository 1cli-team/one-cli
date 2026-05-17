package skills_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/skills"
)

// TestInstall_ReplacesUserHomeDir overrides $HOME so the bundled skills land
// in a tempdir, not the developer's real ~/.claude.
func TestInstall_BundledSkillsLandInClaudeSkills(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	res, err := skills.Install()
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	if res.SkillCount == 0 {
		t.Error("expected at least one skill to be installed")
	}
	if len(res.InstalledTo) != 1 {
		t.Errorf("expected 1 install target; got %v", res.InstalledTo)
	}

	dest := filepath.Join(tmpHome, ".claude", "skills")
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("ReadDir(%s) = %v", dest, err)
	}
	if len(entries) != res.SkillCount {
		t.Errorf("on-disk skills=%d, reported=%d", len(entries), res.SkillCount)
	}

	// Smoke check: the unified skill is present (post-Phase 8b
	// consolidation; v0.3.0 shipped 5 separate skills, replaced by one).
	found := false
	for _, e := range entries {
		if e.Name() == "one-cli" {
			found = true
		}
	}
	if !found {
		t.Errorf("skill one-cli missing after Install()")
	}
}

func TestInstall_OverwritesExistingSkillFiles(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Pre-create an old-version stub at the same skill name to simulate
	// drift. Install must clear it before re-materialising the bundled
	// content.
	dest := filepath.Join(tmpHome, ".claude", "skills", "one-cli")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dest, "STALE.md")
	if err := os.WriteFile(stale, []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := skills.Install(); err != nil {
		t.Fatalf("Install() = %v", err)
	}

	if _, err := os.Stat(stale); err == nil {
		t.Errorf("stale file %s should have been removed during install", stale)
	}
}
