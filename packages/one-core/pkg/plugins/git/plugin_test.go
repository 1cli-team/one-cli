package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	local "github.com/torchstellar-team/one-cli/packages/one-core/internal/adapters/workspaceregistry/local"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	gitplugin "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/git"
)

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func fixture(t *testing.T) (string, *gitplugin.Plugin, *gitmodule.Service, *workspaceapp.RegistryService) {
	t.Helper()
	t.Setenv("ONE_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	git(t, root, "init", "-b", "main")
	manifest := "version = 2\n[workspace]\nid = \"test-workspace\"\nname = \"Test\"\n"
	if err := os.WriteFile(filepath.Join(root, "one.manifest.toml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "initial")
	plugin := gitplugin.New()
	registry, err := workspaceapp.NewRegistryService(local.NewAt(filepath.Join(t.TempDir(), "workspaces.json")), workspaceapp.RegistryOptions{WorktreeIdentity: plugin.WorkspaceIdentity, Git: plugin.InspectWorkspaceRepository})
	if err != nil {
		t.Fatal(err)
	}
	return root, plugin, gitmodule.NewService(plugin, registry), registry
}

func TestWorktreeLifecycleAndWorkspaceIdentity(t *testing.T) {
	root, plugin, service, registry := fixture(t)
	ctx := context.Background()
	main, err := registry.Observe(ctx, root, "test")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "中文 空格\nworktree")
	input := gitmodule.AddInput{Path: path, Branch: "feature/topic", NewBranch: true}
	result, err := service.Add(ctx, root, input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Warning != "" || result.Workspace == nil || result.Workspace.Status != "ready" {
		t.Fatalf("registration: %+v", result)
	}
	if result.Workspace.EntryID != main.EntryID {
		t.Fatal("worktrees must share one logical workspace")
	}
	if result.Workspace.WorktreeID == main.WorktreeID {
		t.Fatal("worktrees must retain separate contexts")
	}
	if _, err := registry.Resolve(ctx, main.EntryID); err != nil {
		t.Fatalf("main conflicts: %v", err)
	}
	if _, err := registry.Resolve(ctx, result.Workspace.EntryID+"~"+result.Workspace.WorktreeID); err != nil {
		t.Fatalf("linked worktree conflicts: %v", err)
	}
	listed, err := service.List(ctx, path)
	if err != nil || len(listed.Worktrees) != 2 {
		t.Fatalf("list from linked: %+v %v", listed, err)
	}
	retry, err := service.Add(ctx, root, input)
	if err != nil || retry.Worktree.ID != result.Worktree.ID {
		t.Fatalf("create retry: %+v %v", retry, err)
	}
	if err := os.WriteFile(filepath.Join(path, "untracked"), []byte("keep me"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := service.Remove(ctx, root, result.Worktree.ID); err == nil {
		t.Fatal("removed dirty worktree")
	}
	if _, err := os.Stat(filepath.Join(path, "untracked")); err != nil {
		t.Fatal("lost untracked file")
	}
	if err := os.Remove(filepath.Join(path, "untracked")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "worktree", "lock", path)
	if err := service.Remove(ctx, root, result.Worktree.ID); err == nil {
		t.Fatal("removed locked worktree")
	}
	git(t, root, "worktree", "unlock", path)
	if err := service.Remove(ctx, root, result.Worktree.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Remove(ctx, root, result.Worktree.ID); err != nil {
		t.Fatalf("remove retry: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("directory still exists: %v", err)
	}
	listed, err = plugin.Inspect(ctx, root)
	if err != nil || len(listed.Worktrees) != 1 {
		t.Fatalf("remove readback: %+v %v", listed, err)
	}
	if err := service.Remove(ctx, root, listed.Worktrees[0].ID); err == nil {
		t.Fatal("removed main worktree")
	}
	git(t, root, "show-ref", "--verify", "refs/heads/feature/topic")
}

func TestConcurrentCreateAndExternalChanges(t *testing.T) {
	root, plugin, _, _ := fixture(t)
	ctx := context.Background()
	input := gitmodule.AddInput{Path: filepath.Join(t.TempDir(), "concurrent"), Branch: "concurrent", NewBranch: true}
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		wg.Go(func() { _, err := plugin.Add(ctx, root, input); errors <- err })
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	external := filepath.Join(t.TempDir(), "external")
	git(t, root, "worktree", "add", "--detach", external)
	listed, err := plugin.Inspect(ctx, root)
	if err != nil || len(listed.Worktrees) != 3 {
		t.Fatalf("external creation: %+v %v", listed, err)
	}
	var detached gitmodule.Worktree
	for _, tree := range listed.Worktrees {
		if tree.Detached {
			detached = tree
		}
	}
	if detached.Path == "" {
		t.Fatal("missing detached checkout")
	}
	if err := plugin.Remove(ctx, root, detached.ID); err != nil {
		t.Fatal(err)
	}
}

func TestIndependentCloneStillConflicts(t *testing.T) {
	root, _, _, registry := fixture(t)
	ctx := context.Background()
	main, err := registry.Observe(ctx, root, "test")
	if err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	git(t, root, "clone", root, clone)
	copy, err := registry.Observe(ctx, clone, "test")
	if err != nil {
		t.Fatal(err)
	}
	if copy.Status != "identity-conflict" {
		t.Fatalf("clone status: %s", copy.Status)
	}
	if _, err := registry.Resolve(ctx, main.EntryID); err == nil {
		t.Fatal("independent clone escaped conflict detection")
	}
}

func TestExistingBranchDetachedAndRejectedInputs(t *testing.T) {
	root, plugin, _, _ := fixture(t)
	ctx := context.Background()
	git(t, root, "branch", "existing")
	path := filepath.Join(t.TempDir(), "existing")
	created, err := plugin.Add(ctx, root, gitmodule.AddInput{Path: path, Branch: "existing"})
	if err != nil || created.Branch != "existing" || created.Detached {
		t.Fatalf("existing branch: %+v %v", created, err)
	}
	for _, input := range []gitmodule.AddInput{
		{Path: filepath.Join(t.TempDir(), "occupied"), Branch: "existing"},
		{Path: filepath.Join(t.TempDir(), "bad"), Branch: "--orphan", NewBranch: true},
		{Path: filepath.Join(t.TempDir(), "invalid"), Branch: "invalid..branch", NewBranch: true},
		{Path: filepath.Join(t.TempDir(), "conflicting"), Branch: "x", NewBranch: true, Detached: true},
	} {
		if _, err := plugin.Add(ctx, root, input); err == nil {
			t.Fatalf("accepted invalid input %+v", input)
		}
	}
	detached, err := plugin.Add(ctx, root, gitmodule.AddInput{Path: filepath.Join(t.TempDir(), "detached"), Detached: true, StartPoint: "main"})
	if err != nil || !detached.Detached || detached.Branch != "" {
		t.Fatalf("detached: %+v %v", detached, err)
	}
	// A live Git environment set by a caller must not redirect repository reads.
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "nonexistent"))
	if _, err := plugin.Inspect(ctx, root); err != nil {
		t.Fatal(err)
	}
}

func TestNestedWorkspaceRegistration(t *testing.T) {
	root, _, service, registry := fixture(t)
	ctx := context.Background()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := []byte("version = 2\n[workspace]\nid = \"nested\"\nname = \"Nested\"\n")
	if err := os.WriteFile(filepath.Join(nested, "one.manifest.toml"), manifest, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "nested")
	if _, err := registry.Observe(ctx, nested, "test"); err != nil {
		t.Fatal(err)
	}
	result, err := service.Add(ctx, nested, gitmodule.AddInput{Path: filepath.Join(t.TempDir(), "nested-copy"), Branch: "nested", NewBranch: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Workspace == nil || result.Workspace.Status != "ready" || result.Workspace.Root != result.Worktree.Path {
		t.Fatalf("nested workspace mapping: %+v", result)
	}
	opened, err := service.Open(ctx, nested, result.Worktree.ID)
	if err != nil || opened.EntryID != result.Workspace.EntryID {
		t.Fatalf("open: %+v %v", opened, err)
	}
}
