# Dashboard Fixed-Manifest Development Mode Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Give `task dev` a stable, representative Workspace/Manifest while keeping Profile CRUD and Profile bindings connected to the developer's real machine-local One configuration.

**Architecture:** Use a checked-in `one.manifest.json` fixture as the development Workspace root. The Go server reads it through the same production Workspace services and HTTP handlers used by `one serve`; there is no browser-time MSW interception. Profile definitions, credentials, defaults, and bindings continue through the real Profile service, while repository-owned manifest data remains read-only.

**Tech Stack:** go-task, Go/Cobra, One manifest v1, React/Vite, Vitest, Go tests.

---

## Design decision

Use a fixed manifest fixture rather than frontend response mocks.

- **Recommended: fixed manifest fixture.** Exercises manifest parsing, Workspace registry resolution, Overview/Project projections, missing-profile diagnostics, Profile CRUD, and binding persistence end to end.
- **Rejected: browser MSW hybrid.** Easy to start, but Workspace/Profile binding mutations would need a second fake state store because the Go server cannot resolve a mocked `entryId`.
- **Rejected: Go mock Workspace provider.** Keeps HTTP realistic, but adds production interfaces and dependency injection solely for development data when a manifest fixture already satisfies the boundary.

The fixture must contain no credentials. Profile values remain in the existing machine-local files under `~/.config/one`; API responses keep using the current masking behavior. Running the development Dashboard can therefore change real local Profiles and fixture-scoped bindings, and the UI/docs must make that explicit.

## Data flow

1. `task dev` finishes `sync-bundled` and `sync-web`.
2. The Go child starts with `packages/cli/testdata/dashboard-dev-workspace` as its working directory.
3. Normal `one serve` discovery finds the fixture's `one.manifest.json`, registers it as the current Workspace, and serves real `/api/workspace*` projections.
4. Vite serves the React UI on port 5173 and proxies `/api/*` to the Go process on port 5174.
5. `/api/configure/*` reads and writes real machine Profiles. Workspace/Project Profile binding PUTs write only machine-local binding data keyed by the fixture's canonical root.
6. The fixture manifest is never mutated by the Dashboard.

### Task 1: Add and validate the development Workspace fixture

**Files:**

- Create: `packages/cli/testdata/dashboard-dev-workspace/one.manifest.json`
- Create: `packages/cli/internal/core/workspace/dashboard_dev_fixture_test.go`

**Step 1: Write the failing fixture contract test**

Add a test that resolves `../../../testdata/dashboard-dev-workspace`, calls `ReadManifest`, and asserts:

- manifest version is `1`;
- Workspace ID is `one-dashboard-dev`;
- environments are `dev`, `preview`, and `prod` with `dev` as default;
- projects cover app, service, and package roots;
- profile-capable env, deploy, and container backends are represented.

**Step 2: Verify the test fails before the fixture exists**

Run:

```bash
go test ./packages/cli/internal/core/workspace -run TestDashboardDevelopmentFixture -count=1
```

Expected: FAIL because `one.manifest.json` is missing.

**Step 3: Add the fixed manifest**

Use a stable Workspace identity and representative projects:

- `web`: React SPA, Node/pnpm, Infisical env, Docker container, Vercel deploy;
- `api`: Go API, Infisical env, Docker container, Kustomize deploy;
- `docs`: Starlight, Infisical env, AWS S3 deploy;
- `shared`: TypeScript package with no deploy/container surface.

Include useful non-secret values such as build versions, dev commands, env key names, deploy environment names, image names, and bucket/project IDs. Do not add API keys, tokens, usernames, passwords, kubeconfig paths, or profile names.

**Step 4: Verify the fixture contract passes**

Run the command from Step 2.

Expected: PASS.

### Task 2: Make `task dev` launch against the fixture

**Files:**

- Modify: `Taskfile.yml`
- Modify: `CONTRIBUTING.md`

**Step 1: Change the Go child working directory**

Set `dev:serve.dir` to `packages/cli/testdata/dashboard-dev-workspace` and run the current source package via:

```bash
go run ../../cmd/one serve --port 5174 --open=false -o text
```

This preserves source-level Go debugging while making normal Workspace discovery select the fixture. Do not add a public `one serve --mock` flag or mock behavior to the release binary.

**Step 2: Document the live Profile boundary**

Update the `task dev` summary and contributor guide to state:

- Workspace/Manifest data comes from the checked-in fixture;
- Profile CRUD uses real machine-local One configuration;
- Profile bindings are real but scoped to the fixture root;
- the Vite URL can be opened directly without a session token.

**Step 3: Verify Task expansion**

Run:

```bash
task --dry dev
```

Expected: preparation tasks run first, followed by parallel Go and Vite child tasks; the Go command is rooted at the fixture.

### Task 3: Add a visible development-data safety indicator

**Files:**

- Modify: `Taskfile.yml`
- Modify: `apps/dashboard/src/vite-env.d.ts`
- Modify: `apps/dashboard/src/components/TopBar.tsx`
- Modify: `apps/dashboard/src/components/TopBar.test.tsx`
- Modify: `apps/dashboard/src/locales/en-US.json`
- Modify: `apps/dashboard/src/locales/zh-CN.json`

**Step 1: Write the failing UI test**

Add a TopBar test for a development indicator conveying both halves of the mode: “Fixture Workspace” and “Live local Profiles”. Production-mode rendering must not show it.

**Step 2: Add the development-only build flag**

Set `VITE_DEV_DATA_MODE=fixture-live-profiles` only on `dev:dashboard`. Type the variable in `vite-env.d.ts`; never set it during `build-web`, so the embedded release UI remains unchanged.

**Step 3: Render the localized indicator**

Show a compact warning badge/banner in the TopBar only when the flag is present. It must not display profile values or filesystem paths.

**Step 4: Run the focused test**

```bash
pnpm --dir apps/dashboard test -- TopBar.test.tsx
```

Expected: PASS in both flagged and unflagged cases.

### Task 4: Verify the hybrid behavior end to end

**Files:**

- Modify if coverage needs a reusable fixture helper: `packages/cli/internal/transport/http/handlers_workspaces_test.go`
- Modify if a browser-level regression is found: `apps/dashboard/src/pages/Overview.test.tsx`

**Step 1: Start the stack**

```bash
task dev
```

Open `http://localhost:5173/`.

**Step 2: Verify fixed Workspace projections**

Check that the fixture is current, all four projects render, environment switching works, and missing Profile diagnostics reflect the developer's actual configured Profiles.

**Step 3: Verify real Profile operations carefully**

Create a clearly disposable Profile such as `dashboard-dev-probe`, verify it appears after refresh, bind it to the fixture in one environment, unbind it, and then remove it. Confirm the fixture manifest remains byte-for-byte unchanged.

**Step 4: Run automated verification**

```bash
go test ./packages/cli/internal/core/workspace ./packages/cli/internal/application/workspace ./packages/cli/internal/transport/http
pnpm --dir apps/dashboard test
task check:dashboard
git diff --check
```

Expected: all commands pass. Do not stage or commit unless the user explicitly requests it.

## Acceptance criteria

- `task dev` remains the single startup command.
- Dashboard Workspace and Project content is deterministic across machines.
- Runtime frontend code does not install or enable an MSW worker.
- Profile CRUD and Profile binding requests use the real Go API.
- No credential or machine Profile value is checked into the fixture.
- Production `one serve` behavior and embedded UI remain unchanged.
- The UI clearly distinguishes fixture Workspace data from live local Profile data.
