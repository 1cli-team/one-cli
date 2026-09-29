package creation

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runTestGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func isolateGit(t *testing.T) {
	t.Helper()
	// Commit hooks export paths for the invoking repository. Fixtures and
	// linked worktrees must discover their own Git metadata instead.
	for _, key := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES"} {
		t.Setenv(key, "") // Restore the original value when the test finishes.
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func TestCreateInEmptyGitRepositoryPreservesIdentityAndHooks(t *testing.T) {
	isolateGit(t)
	root := t.TempDir()
	runTestGit(t, root, "init", "-b", "feature")
	runTestGit(t, root, "remote", "add", "origin", "https://example.invalid/team/demo.git")
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\n# team hook\n"), 0755); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, ".git", "config"))
	result, err := newCreationService(t).CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo", CreatedInPlace: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.HooksWarn == nil {
		t.Fatal("custom hook conflict was not reported")
	}
	after, _ := os.ReadFile(filepath.Join(root, ".git", "config"))
	if string(before) != string(after) {
		t.Fatal("Git config changed")
	}
	if runTestGit(t, root, "symbolic-ref", "--short", "HEAD") != "feature" {
		t.Fatal("branch changed")
	}
	got, _ := os.ReadFile(hook)
	if string(got) != "#!/bin/sh\n# team hook\n" {
		t.Fatal("custom hook changed")
	}
}

func TestEmptyTargetGitValidation(t *testing.T) {
	isolateGit(t)
	for _, kind := range []string{"initialized", "cloned", "invalid", "file", "tracked-deletion", "staged-deletion", "sparse-index"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			switch kind {
			case "invalid":
				if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
					t.Fatal(err)
				}
			case "cloned":
				origin := t.TempDir()
				runTestGit(t, origin, "init", "--bare")
				runTestGit(t, root, "clone", origin, ".")
			default:
				runTestGit(t, root, "init")
				if kind != "initialized" {
					name := filepath.Join(root, "README.md")
					if err := os.WriteFile(name, []byte("keep"), 0644); err != nil {
						t.Fatal(err)
					}
					if kind != "file" {
						runTestGit(t, root, "add", "README.md")
						runTestGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "initial")
						if kind == "sparse-index" {
							runTestGit(t, root, "update-index", "--skip-worktree", "README.md")
						}
						if err := os.Remove(name); err != nil {
							t.Fatal(err)
						}
						if kind == "staged-deletion" {
							runTestGit(t, root, "add", "-u")
						}
					}
				}
			}
			err := newCreationService(t).ValidateWorkspaceTarget(root)
			allowed := kind == "initialized" || kind == "cloned"
			if (err == nil) != allowed {
				t.Fatalf("allowed=%v error=%v", allowed, err)
			}
			if _, err := os.Stat(filepath.Join(root, "one.manifest.json")); !os.IsNotExist(err) {
				t.Fatal("validation wrote files")
			}
		})
	}
}

func TestCreateLinkedEmptyWorktreeDoesNotInstallSharedHooks(t *testing.T) {
	isolateGit(t)
	origin := t.TempDir()
	runTestGit(t, origin, "init")
	runTestGit(t, origin, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "empty")
	root := filepath.Join(t.TempDir(), "linked")
	runTestGit(t, origin, "worktree", "add", "--detach", root)
	result, err := newCreationService(t).CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "linked", CreatedInPlace: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.HooksWarn == nil {
		t.Fatal("shared hook skip was not reported")
	}
	if _, err := os.Stat(filepath.Join(origin, ".git", "hooks", "pre-commit")); !os.IsNotExist(err) {
		t.Fatal("shared hooks changed")
	}
}
