# Mode: Bootstrap Missing Dependencies

Use when an agent is about to build, test, run, or continue work in a One
workspace and dependencies are not installed yet. Also use immediately after
`one create` / `one add` when the next step needs runnable code.

The goal is to make the workspace runnable without asking the user to execute
obvious setup commands. Do not use one Node command for every project: One
workspaces can contain JS / TS and Go projects at the same time.

## Step 1 — Find the workspace and projects

Walk upward from cwd to find `one.manifest.json`, then read it.

Important fields:

- `packageManager`: workspace default for Node projects.
- `projects[].relativeDir`: where the project lives.
- `projects[].toolchain`: usually `node` or `go`.
- `projects[].packageManager`: optional Node override.

If there is no manifest, fall back to local file probes:

- JS / TS / Node: `package.json`
- Go: `go.mod`

## Step 2 — JS / TS / Node dependency install

Run one install from the workspace root when the workspace has Node
projects or a root `package.json`.

Pick the package manager in this order:

1. `one.manifest.json#packageManager` or `projects[].packageManager`
2. Lockfiles: `pnpm-lock.yaml`, `package-lock.json`, `yarn.lock`
3. `packageManager` in `package.json`
4. Default to `pnpm` for One-generated workspaces

Commands:

```bash
pnpm install
npm install
yarn install
```

Do not run `pnpm install` inside every Node project unless the workspace is
not a monorepo and there is no root package manager file.

## Step 3 — Go dependency install

For each Go project, run module commands from that project directory.

Use a non-mutating dependency fetch first:

```bash
go mod download
```

Use tidy when the agent changed imports, created files that import new
packages, or the error mentions missing / stale `go.mod` or `go.sum`:

```bash
go mod tidy
```

Go has no `node_modules` equivalent. Do not try to detect Go dependency state
by looking for a vendor directory; One-generated Go projects do not require
one.

## Step 4 — Mixed workspace rule

If the manifest contains both Node and Go projects:

1. Run the Node workspace install once from the root.
2. Run `go mod download` or `go mod tidy` in each Go project that needs it.

Then continue with the user's requested build / test / run command.

## Step 5 — Report succinctly

In the final response, mention only the dependency commands that actually ran.
If you skipped dependency install because the user asked not to mutate the
workspace, say so.
