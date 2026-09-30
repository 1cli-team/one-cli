# Task startup environment loading

## Problem and design

Composer's server, home and admin tasks read nine inherited folders serially,
even though only six paths are unique. Each project also reconstructs the
Infisical client and reads the same login session. This delays the first task
output. Existing binding verification and dependency checks remain required.

- Add an optional batch loader port; keep serial fallback for other providers.
- Resolve configuration and login once per batch. Read each unique folder once
  with at most six concurrent requests and a 30-second request timeout.
- Reuse the SDK list API with a context-aware HTTP client, since the SDK's
  high-level client does not bind request lifetimes to its constructor context.
- Merge root → ancestors → project after all reads succeed. Missing folders are
  empty; permission, network and other API errors abort before any task starts.
- Keep snapshots in invocation memory only. Each later invocation fetches fresh
  values; project maps are isolated and no secret values are written to disk.
- Report task preparation, dependency checks, environment loading and binding
  verification on stderr in both supported languages. Keep dry-run, discovery
  and structured stdout unchanged.

## Validation

Cover deduplication, precedence despite out-of-order completion, bounded parallel
reads, empty folders, failures, cancellation, freshness, project/environment
isolation, the fallback loader and task integration. Run the affected race tests
and the repository `one run check` gate. Compare the preserved original binary
with the rebuilt CLI using no-op tasks and development-only environment reads.
No production changes, deployment or environment writes are part of this work.

## Measured development startup

A temporary workspace mirrors `services/server`, `apps/home`, and `apps/admin`,
using Composer's existing dev binding. All three tasks run `/usr/bin/true`.
No business service is started and no remote configuration is written. The
original `a11b08d` binary and optimized binary alternate for three rounds:

| Round | Original | Optimized |
| --- | --- | --- |
| 1 | 7.267 s | 1.514 s |
| 2 | 6.603 s | 2.109 s |
| 3 | 6.948 s | 1.738 s |

Median: 6.948 s → 1.738 s, about 75% less time. The new progress output appeared
within 9–202 ms. These measurements exclude application dependency preparation,
building and server startup; network latency and local load still affect them.

The integration tests exercise both batched and legacy loaders with native mise,
including child/grandchild environments, aliases, file tasks, profile overrides,
concurrent invocations, rotation/removal of variables, and session cleanup.

## Verification results

- Passed targeted `one run cli:test:race` checks for batch fetching, scope
  isolation, cancellation, legacy-loader map isolation, native mise project
  environments, freshness and authenticated session cleanup.
- Passed bilingual startup progress and static version/help/dry-run terminal
  tests, including the prohibition on startup terminal queries.
- `one run check --concurrency 1` passed Go vet/format, architecture/unit tests,
  CLI references/help/frontmatter checks, and all 101 Dashboard tests.
- Initial complete checks failed because One prepends the selected mise binary's
  directory to PATH, allowing host package tools to override the e2e fixture's
  fake pnpm. The Go fixture also inherited a Go 1.27.1 GOROOT while invoking
  Go 1.25.0. The pnpm misrouting was reproduced with both the original `a11b08d`
  binary and the optimized binary in an unbound temporary workspace.
- The fixture now symlinks the real mise binary into its fake-tool directory and
  clears inherited GOROOT so the selected Go binary discovers its own standard
  library. The representative Node build-order and Go build/argument-forwarding
  regressions passed through `one run cli:test`; application runtime and
  toolchain selection behavior are unchanged.
- `one run test:go --concurrency 1` passed the complete Kernel and CLI race
  suites, including all e2e tests. The repository pre-commit hook enforces the
  complete check against the staged snapshot before creating the commit.
- The initial parallel gate also hit six Dashboard 5-second timeouts. All 101
  passed when rerun without competing Go compilation.
- `git diff --check` passed.
