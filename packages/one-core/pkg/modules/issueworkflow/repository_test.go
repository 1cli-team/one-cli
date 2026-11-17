package issueworkflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOriginDefaultBranchWithoutLocalTrackingRefs(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root, remote := t.TempDir(), t.TempDir()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run(remote, "init", "--bare", "-b", "trunk")
	run(root, "init", "-b", "scratch")
	run(root, "remote", "add", "origin", remote)
	if _, err := originDefaultBranch(t.Context(), root); err == nil {
		t.Fatal("an empty origin returned a branch")
	}
	seed := t.TempDir()
	run(seed, "init", "-b", "trunk")
	run(seed, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "Initial")
	run(seed, "push", remote, "trunk")
	if _, err := os.Stat(filepath.Join(root, ".git", "refs", "remotes")); !os.IsNotExist(err) {
		t.Fatal("the fixture already has remote tracking refs")
	}
	branch, err := originDefaultBranch(t.Context(), root)
	if err != nil || branch != "trunk" {
		t.Fatalf("default branch = %q, %v", branch, err)
	}
}
