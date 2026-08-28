---
title: Continuous integration
description: Inspect, enable, refresh, or remove optional CI workflows.
---

Continuous integration is opt-in. `one create` and `one add` do not generate
workflow files. The current build supports GitHub Actions.

## Usage

```bash
one ci
one ci enable [project]
one ci sync [project]
one ci disable [project]
```

`one ci` is read-only and shows each project's current state and the most useful
next command.

## Commands

| Command | Behavior |
|---|---|
| `one ci` | Show CI status for all projects |
| `one ci enable web` | Generate the canonical workflow for `web` |
| `one ci enable` | Enable CI for every project |
| `one ci sync web` | Regenerate `web` when CI is already enabled |
| `one ci sync` | Regenerate every workflow that already exists |
| `one ci disable web` | Remove `web`'s generated workflow after confirmation |
| `one ci disable --yes` | Remove all generated workflows without prompting |

Refusing the disable confirmation or pressing Ctrl-C is a normal cancellation:
the command exits successfully and does not modify files.

## Files and workspace state

For GitHub Actions, One CLI writes one canonical file per project under
`.github/workflows/`. CI selection is not written to `one.manifest.json`; the
generated file is the state. `enable`, `sync`, and `disable` therefore leave the
manifest unchanged.

## Automation

```bash
one ci -o json
one ci enable web --provider ci/github-actions -o json
one ci sync --project web -o json
one ci disable web --yes -o json
```

Stable schemas are `one-cli/ci-status/v1`, `one-cli/ci-enable/v1`,
`one-cli/ci-sync/v1`, and `one-cli/ci-disable/v1`. The provider ID and JSON field
names remain stable and are not translated. `--project` is retained for older
scripts; everyday help prefers the positional project.

## Errors

| Code | Recovery |
|---|---|
| `CI_NOT_ENABLED` | Run `one ci enable <project>`, then retry sync |
| `CI_PROVIDER_UNKNOWN` | Use an ID from `error.context.available_providers` |
| `CI_RENDER_FAILED` | Inspect the project and workflow path in the error context |

See [Error codes](/en/docs/error-codes/) for the stable error envelope.
