package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
)

func gitValue(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	data, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, data)
	}
	return strings.TrimSpace(string(data))
}

func TestCreateBranchPreservesCheckoutAndRejectsInvalidInput(t *testing.T) {
	root, _, service, _ := fixture(t)
	ctx := context.Background()
	file := filepath.Join(root, "untracked.txt")
	if err := os.WriteFile(file, []byte("keep me"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := service.CreateBranch(ctx, root, gitmodule.CreateBranchInput{Branch: "mg-1-测试", StartPoint: "main"})
	if err != nil || result.CheckedOut || result.Warning != "" {
		t.Fatalf("create: %+v %v", result, err)
	}
	if gitValue(t, root, "branch", "--show-current") != "main" {
		t.Fatal("changed the current checkout")
	}
	if result.Head != gitValue(t, root, "rev-parse", "refs/heads/mg-1-测试") {
		t.Fatal("incorrect created head")
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "keep me" {
		t.Fatal("changed local work")
	}
	for _, input := range []gitmodule.CreateBranchInput{
		{Branch: "mg-1-测试"}, {Branch: ""}, {Branch: "--orphan"}, {Branch: "bad..name"}, {Branch: "bad\nname"}, {Branch: "invalid-start", StartPoint: "--all"},
	} {
		if _, err := service.CreateBranch(ctx, root, input); err == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
	if gitValue(t, root, "branch", "--show-current") != "main" {
		t.Fatal("invalid request changed checkout")
	}
	repository, err := service.List(ctx, root)
	if err != nil || len(repository.Branches) != 2 || len(repository.Worktrees) != 1 {
		t.Fatalf("unexpected repository: %+v %v", repository, err)
	}
}

func TestCreateBranchSwitchFailurePreservesLocalChanges(t *testing.T) {
	root, _, service, _ := fixture(t)
	file := filepath.Join(root, "tracked.txt")
	if err := os.WriteFile(file, []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "tracked.txt")
	git(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "base")
	git(t, root, "switch", "-c", "other")
	if err := os.WriteFile(file, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-am", "other")
	git(t, root, "switch", "main")
	if err := os.WriteFile(file, []byte("local work"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := service.CreateBranch(context.Background(), root, gitmodule.CreateBranchInput{Branch: "conflict", StartPoint: "other", Checkout: true})
	if err == nil {
		t.Fatal("overwrote local changes")
	}
	if gitValue(t, root, "branch", "--list", "conflict") != "" {
		t.Fatal("failed checkout left a branch")
	}
	if gitValue(t, root, "branch", "--show-current") != "main" {
		t.Fatal("failed checkout changed branch")
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "local work" {
		t.Fatal("lost local changes")
	}
	result, err := service.CreateBranch(context.Background(), root, gitmodule.CreateBranchInput{Branch: "mg-2", Checkout: true})
	if err != nil || !result.CheckedOut || gitValue(t, root, "branch", "--show-current") != "mg-2" {
		t.Fatalf("switch: %+v %v", result, err)
	}
}

func TestConcurrentBranchCreation(t *testing.T) {
	root, _, service, _ := fixture(t)
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wait.Go(func() {
			_, err := service.CreateBranch(context.Background(), root, gitmodule.CreateBranchInput{Branch: "mg-3"})
			results <- err
		})
	}
	wait.Wait()
	close(results)
	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("created branch %d times", succeeded)
	}
	if gitValue(t, root, "branch", "--show-current") != "main" {
		t.Fatal("changed checkout")
	}
}

func TestCreateBranchReportsCommittedHookFailure(t *testing.T) {
	root, _, service, _ := fixture(t)
	hook := filepath.Join(root, ".git", "hooks", "post-checkout")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nprintf 'post-checkout failed' >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	result, err := service.CreateBranch(context.Background(), root, gitmodule.CreateBranchInput{Branch: "mg-4", Checkout: true})
	if err != nil || result == nil || !result.CheckedOut || result.Warning == "" {
		t.Fatalf("committed operation was lost: %+v %v", result, err)
	}
	if gitValue(t, root, "branch", "--show-current") != "mg-4" {
		t.Fatal("incorrect branch")
	}
}

func TestCreateBranchUsesSelectedWorktree(t *testing.T) {
	root, plugin, service, _ := fixture(t)
	linked := filepath.Join(t.TempDir(), "linked")
	if _, err := plugin.Add(context.Background(), root, gitmodule.AddInput{Path: linked, Branch: "existing", NewBranch: true}); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(linked, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateBranch(context.Background(), nested, gitmodule.CreateBranchInput{Branch: "mg-5", Checkout: true}); err != nil {
		t.Fatal(err)
	}
	if gitValue(t, root, "branch", "--show-current") != "main" {
		t.Fatal("switched the wrong worktree")
	}
	if gitValue(t, linked, "branch", "--show-current") != "mg-5" {
		t.Fatal("did not switch selected worktree")
	}
}
