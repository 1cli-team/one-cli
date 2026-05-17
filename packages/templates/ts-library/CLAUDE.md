# ts-library — Agent Guide

Publishable TypeScript library. Stack: **TypeScript (strict) + tsdown (build) + vitest (test) + oxlint + oxfmt**.

## Project layout

```
src/
├── index.ts            # public entry point — only export what's stable
└── index.test.ts       # tests next to source

package.json            # name + version + exports map (this is the contract)
tsdown.config.ts        # build config (ESM + CJS + .d.ts)
tsconfig.json           # strict mode on
vitest.config.ts        # test runner config
```

## Library contract — these are the rules

A library is a public API. Every exported symbol is a promise.

- **Only export what's intentional.** Re-exports from `src/index.ts` are the public API. Internals stay un-exported.
- **Semantic versioning is non-negotiable.** Breaking changes bump major. New features bump minor. Bugfixes bump patch. Use `pnpm changeset` to record changes.
- **Type definitions ship.** `tsdown` produces `.d.ts`. If a type changes, that's an API change.
- **Don't depend on runtime conveniences that consumers don't have.** No Node-only APIs unless the library is Node-only (declared in `package.json#engines`).
- **Don't ship dev-only code.** Test files, fixtures, source maps to `dist/` only when intentional.

## Engineering discipline — mandatory

1. `pnpm typecheck` exits 0
2. `pnpm lint` exits 0
3. `pnpm test` passes — and any new public API must come with tests
4. `pnpm build` produces `dist/` cleanly
5. Run `pnpm changeset` for any user-visible change — semver bump + changelog entry.
6. Stage explicitly: `git add <file>`. Never `git add -A`.
7. Conventional commit messages.

## Testing conventions

- Tests live next to source as `<name>.test.ts`. Vitest auto-discovers them.
- Test the **public API** (what `src/index.ts` exports), not internal helpers.
- Avoid mocking — pure functions don't need it. Inject collaborators when impurity is unavoidable.
- `pnpm test:coverage` to check coverage gaps before publishing.

## Code style

- TypeScript `strict` mode is on. Keep it.
- ❌ NEVER `any`. Use `unknown` and narrow, or generics.
- ❌ NEVER ship side effects from import. The library should be tree-shakable.
- ❌ NEVER add a runtime dependency without considering bundle-size impact for consumers.
- ✅ Every exported function / class / type has a JSDoc with at least a one-line summary + `@example` for non-trivial APIs.
- ✅ Prefer narrow types: `string` is rarely the right type — use a branded type or literal union.

## Adding a new public API

1. Implement in `src/<feature>.ts`.
2. Add tests in `src/<feature>.test.ts` (happy path + edge cases + error cases).
3. Re-export from `src/index.ts`.
4. Run `pnpm typecheck && pnpm test && pnpm build`.
5. `pnpm changeset` → describe the change in the prompt.
6. Commit including the `.changeset/*.md` file.

## Publishing

```bash
pnpm changeset version    # bumps package.json + writes CHANGELOG.md
pnpm changeset publish    # publishes to npm (after CI gate passes)
```

## Quality gates

```bash
pnpm typecheck
pnpm lint
pnpm test
pnpm build
```

All must pass before declaring a change complete.
