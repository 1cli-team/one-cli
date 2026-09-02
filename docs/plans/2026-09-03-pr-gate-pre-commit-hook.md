# PR Gate Pre-Commit Hook Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Run the same `task check` contract used by pull-request CI before every local commit.

**Architecture:** Keep `Taskfile.yml` as the single source of truth. A versioned native Git hook first requires the working tree to match the staged snapshot, then delegates to `task check`; a Task target configures `core.hooksPath` for the current checkout, and the existing developer install enables it automatically.

**Tech Stack:** Git hooks, POSIX shell, go-task, GitHub Actions.

---

### Task 1: Repair the current CI failure

**Files:**

- Modify: `apps/dashboard/src/pages/Overview.test.tsx`

1. Remove the unused `userEvent.setup()` value reported by TypeScript.
2. Confirm that the test still opens the project settings without user interaction.

### Task 2: Add the repository-managed hook

**Files:**

- Create: `.githooks/pre-commit`
- Modify: `Taskfile.yml`

1. Add a POSIX pre-commit hook that resolves the repository root and executes `task check`.
2. Reject unstaged or untracked files so the checked working tree matches the staged commit snapshot.
3. Fail with an actionable message when go-task is unavailable.
4. Add `task hooks:install` to set the checkout-local `core.hooksPath`.
5. Invoke the hook installer from `task install` so the documented initial setup enables it.

### Task 3: Document the workflow

**Files:**

- Modify: `CONTRIBUTING.md`

1. Document `task hooks:install` in the daily command list.
2. Explain that commits run the PR gate automatically.
3. Keep `task pre-push` as the additional race-detector gate.

### Verification

1. Run `task hooks:install`; expect `git config --local --get core.hooksPath` to return `.githooks`.
2. Run `task check`; expect the same static and test subtasks used by PR CI to pass.
3. Create a commit normally; expect the hook to print its PR-gate message before Git writes the commit.
