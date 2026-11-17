package issueworkflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInstalledCodexAppServer(t *testing.T) {
	if os.Getenv("ONE_GATEWAY_CODEX_SMOKE") != "1" {
		t.Skip("set ONE_GATEWAY_CODEX_SMOKE=1 to use the installed Codex App Server")
	}
	executable, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	stateDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(stateDir, "checkout")
	if err := os.Mkdir(worktree, 0700); err != nil {
		t.Fatal(err)
	}
	c := Codex{Executable: executable, StateDir: stateDir}
	// The isolated command exercises the real stdio transport and permissions
	// without requesting a model turn or using GitHub or Linear.
	job := Job{Worktree: worktree, Project: Project{Check: []string{
		"/bin/sh", "-c", "printf ok > verification-result",
	}}}
	if err := c.Verify(t.Context(), job, ""); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(worktree, "verification-result"))
	if err != nil || string(got) != "ok" {
		t.Fatalf("verification output = %q, %v", got, err)
	}
}
