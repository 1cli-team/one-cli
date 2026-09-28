---
title: one hk
description: Workspace checks, explicit fixes, and Git commit checks with hk.
---

`one hk` runs the pinned hk version through mise. One starts hk at the workspace root and forwards its arguments, output, and exit code.

```bash
one hk check                 # Check changed files
one hk check --all           # Check the whole workspace, including in CI
one hk check --plan --json   # Preview checks
one hk fix                   # Fix files without staging them
one hk validate              # Validate the configuration
```

New workspaces install local `pre-commit` and `commit-msg` launchers. After cloning a workspace or moving your One installation, run `one init hooks` to install or refresh the launchers.

## Configuration

- `.config/hk.pkl` is the workspace's hk configuration. hk discovers it from the workspace root. Project paths in checks remain relative to that root.
- Root `mise.toml` pins the hk version.
- Git launchers are installed in the repository's Git hooks directory.

Edit `.config/hk.pkl` directly. `one add` and `one init hooks` update unchanged defaults while preserving your edits and comments. If you and One change the same check differently, the operation reports `HOOKS_CONFIG_CONFLICT` and preserves the existing file.

Keep the `// one:begin`, `// one:end`, `// one:insert`, and `// one:managed-v1` tracking comments. Add custom checks inside `steps`, outside generated entry markers. To delete a default check, remove its matching begin/end markers along with its body; later refreshes preserve that deletion. An existing `.config/hk.pkl` without One markers stays fully user-owned.

An existing root `hk.pkl` takes precedence over `.config/hk.pkl`. Merge its settings into `.config/hk.pkl` and remove the root file before configuring One hooks. Extra hook events require their own Git launchers.

## Defaults

Pre-commit checks staged content, temporarily stashing and restoring unstaged changes. It does not fix or stage files automatically. Commit messages use hk's built-in Conventional Commits check. Go projects use `gofmt`; Node projects use declared oxlint / oxfmt tools or explicit `lint` and `format:check` scripts.

Run `one hk fix`, review the changes, and stage them again before retrying a failed commit. Node checks require project dependencies; install them with `one mise exec -- pnpm install` at the workspace root.

## Preview and refresh

```bash
one init hooks --dry-run -o json
one init hooks
```

Only recognized default Husky / commitlint files are migrated automatically. Custom rules and hook directories produce a conflict for manual integration. Reinstall Node dependencies afterward to update the lockfile.

For CI, prepare One and project dependencies, then run `one hk check --all`.
