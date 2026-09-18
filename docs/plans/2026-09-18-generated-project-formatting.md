# Generated Project Formatting Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Freshly generated projects pass formatting checks, and failed commit checks show a repair command that works from the workspace root.

**Architecture:** Format the bundled Node templates with their declared Oxfmt version. Update generated package manifests by replacing only the requested JSON fields, preserving unrelated layout, key order, and literal characters. Append a localized repair hint at the One hk command boundary without changing child output or exit status.

**Tech Stack:** Go, encoding/json, Cobra, Oxfmt, hk, mise.

---

### Task 1: Preserve generated package formatting

**Files:** `packages/cli/internal/modules/creation/languages.go`, `ordered_json.go`, a focused JSON field editor, and creation tests.

1. Add failing tests for package name/manager updates and script replacement that preserve tabs, arrays, field order, and shell characters.
2. Replace whole-object map serialization with JSON-decoder-based field edits. Encode changed strings without HTML escaping; preserve script ordering.
3. Keep root workspace package generation formatted and preserve existing root fields during workspace updates.
4. Run creation tests for pnpm, npm, yarn, and bun.

### Task 2: Normalize templates and verify generated projects

**Files:** `packages/templates/` Node templates and a creation integration test.

1. Run Oxfmt 0.67.0 within each Node template, using its own EditorConfig; inspect formatting-only changes.
2. Synchronize bundled resources with `task sync-bundled`.
3. Generate each Node project through the creation service, then run the real formatter against the generated files using `ONE_TEST_OXFMT_BINARY`.
4. Validate React's generated lint and formatting checks and a real first Git commit in a temporary workspace.

### Task 3: Actionable hook failures

**Files:** `packages/cli/internal/transport/cobra/hooks/cmd.go`, its tests, English/Chinese i18n catalogs, and hook documentation.

1. Add tests for failed pre-commit/check commands, successful checks, commit-msg failures, and cancellation.
2. After failed checks, print a localized hint to run `one hk fix` from the workspace root, review and stage changes, then retry. Do not suggest formatting fixes for commit-message errors or interrupted commands.
3. Document that `one mise` keeps the current directory and show a complete subproject command.
4. Run `task check` and inspect the final diff. Leave changes for review without automatically committing or pushing.

## Validation results

- All eight bundled Node templates were formatted with Oxfmt 0.67.0; only the Expo home screen required a source formatting change.
- The creation-service integration test generated all eight Node projects and verified both their files and workspace package manifests with the real formatter.
- Package edit regressions cover pnpm/npm/yarn/bun, indentation, compact arrays, readable shell operators, field removal/addition, escaped keys, CRLF, and idempotence.
- A temporary React workspace created by the compiled CLI installed dependencies and completed its first Git commit with the real hk lint, formatting, and commit-message checks.
- A deliberately misformatted package manifest blocked the next temporary commit and displayed the repair hint. Running `one hk fix` at the workspace root restored formatting without changing the index.
- `task check` passed, including Go/E2E tests and all 56 Dashboard tests.
