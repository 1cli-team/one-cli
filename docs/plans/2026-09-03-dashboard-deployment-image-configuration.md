# Dashboard Deployment Image Configuration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Present image/container settings as a deployment dependency in the Dashboard, only when the selected deployment backend requires a container artifact.

**Architecture:** Keep `container` and `deploy` as independent Manifest and CLI domains. Compose them in the project settings UI by reading the selected deploy backend's catalog requirements. A deploy backend that declares `container/build` or `container/push` capabilities reveals the image configuration directly below deployment settings; other deploy backends do not render it. Existing hidden container values remain untouched.

**Tech Stack:** React 19, TypeScript, Zustand, SWR, Vitest, Testing Library, One CLI backend catalog.

---

### Task 1: Lock the progressive-disclosure behavior with tests

**Files:**

- Modify: `apps/dashboard/src/pages/Overview.test.tsx`

**Steps:**

1. Add container capability requirements to the Kustomize catalog fixture.
2. Assert the project settings tabs no longer include a top-level Container tab.
3. Assert Vercel deployment does not render image configuration.
4. Assert Kustomize deployment renders image configuration and saves its registry Profile through the container binding endpoint.
5. Assert changing the staged deploy backend from Vercel to Kustomize immediately reveals image configuration.
6. Run the focused Overview test and confirm the new assertions fail before implementation.

### Task 2: Model image requirements from the backend catalog

**Files:**

- Modify: `apps/dashboard/src/api/catalog.ts`

**Steps:**

1. Add a pure helper that detects whether a backend requires a container artifact from its non-optional capability requirements.
2. Cover both `container/build` and `container/push` without checking backend names.

### Task 3: Compose image settings inside deployment

**Files:**

- Modify: `apps/dashboard/src/features/project-settings/ProjectInspector.tsx`
- Modify: `apps/dashboard/src/features/project-settings/forms/FormLayout.tsx`
- Modify: `apps/dashboard/src/features/project-settings/forms/ContainerForm.tsx`

**Steps:**

1. Remove Container from the project-level tab list.
2. Read the effective deploy backend from the staged Manifest draft, falling back to the persisted project setting.
3. Render `DeployForm` and `ContainerForm` as sibling forms inside the Deploy tab only when the selected backend requires an image.
4. Aggregate deploy and registry Profile dirty state so saving one form does not clear the other's unsaved-change guard.
5. Keep an already-dirty image form visible if the staged deploy target changes, preventing an unsaved local Profile selection from disappearing.
6. Give each form an accessible name so the two independent save actions remain unambiguous.

### Task 4: Align secondary navigation and copy

**Files:**

- Modify: `apps/dashboard/src/features/project-settings/ProjectMatrix.tsx`
- Modify: `apps/dashboard/src/features/workspace-overview/WorkspaceActionCenter.tsx`
- Modify: `apps/dashboard/src/locales/en-US.json`
- Modify: `apps/dashboard/src/locales/zh-CN.json`

**Steps:**

1. Remove the standalone container column from the project matrix.
2. Route container issues to the Deploy tab.
3. Rename the embedded section to Image configuration / 镜像配置 and explain that it is required by the selected deployment.
4. Remove unused top-level Container-tab and matrix-column translations.

### Task 5: Verify the completed change

**Steps:**

1. Run the focused Overview tests.
2. Run the full Dashboard test suite.
3. Run Dashboard lint, formatting checks, and production build.
4. Run `git diff --check` and inspect the final diff for unrelated changes.
5. Perform visual QA of Vercel and Kustomize project deployment views if the local Dashboard can be started safely.

Do not stage, commit, or push changes unless the user explicitly requests it.

## Follow-up: paired deployment and Profile layout

The deployment Manifest controls and its Backend-specific Profile are one master-detail relationship. Present them in a responsive 3:2 grid when the available content width is sufficient, with Manifest configuration on the left and the matching local Profile on the right. Stack them on narrower screens.

The Profile field follows the currently selected deploy Backend immediately. If that Backend differs from the persisted Manifest, allow the user to inspect and preselect one of its Profiles, but keep the local-binding save action disabled with a direct instruction to save the pending Manifest change first. This preserves the existing API invariant that Profile bindings are validated against the repository's current Backend and prevents credentials from being written under the wrong Backend.

## Follow-up: embed Profile with its Backend

Treat the Profile selector as a dependent Backend field instead of a separate configuration card. Embed it inside the environment, deployment, and image configuration cards while retaining the copy and save action that identify Profile bindings as machine-local.

Backend and project fields continue to stage Manifest changes; the footer action continues to save only the local Profile binding. Wide layouts align related controls in one row, while narrow layouts stack them without horizontal overflow.

The editable configuration already communicates the active Backend, so omit the duplicated inherited-Backend banner from every project settings form. This keeps attention on the actionable Backend/Profile pair without changing either persistence path.

Apply the same dependency layout to Workspace environment settings: merge the machine-local Profile selector and save state into the Backend card. The single responsive card keeps Manifest and local-binding persistence visually distinct as two aligned columns, stacking them on narrow screens.

Profile binding changes now persist immediately when a Dashboard selector changes. Remove the explicit confirmation footers from Workspace and Project forms, disable selectors while requests are in flight, and revert the visible selection when an automatic save fails. A Profile remains unavailable while its Backend is only staged in the Manifest draft so the binding cannot be written against the previously active Backend.

Environment Backend and Profile selection remain Workspace-level concerns. The project Environment form therefore exposes only project-specific Manifest overrides (path, inheritance, disabled state, and declared variable names) and does not render or mutate a separate project environment Profile binding.

When the Workspace environment Backend is Infisical, replace the project's declared-key summary with the existing secret-management surface fixed to that project and environment. Users can list, reveal, copy, create, update, and delete values in the active project folder without leaving the project Environment tab; the workspace-level Secrets tab remains available for switching across scopes.

Render persistent boolean project settings as switches rather than checkbox selections. This applies consistently to environment inheritance, project-environment disabling, and deployment image enablement while preserving the existing Manifest draft behavior.

Keep the deployment and image sections visually terse: render only the section title above each configuration card. Remove the repeated explanatory sentence and the detached machine-Profile badge because the embedded Profile field already communicates both context and persistence scope.

The Manifest publication dialog renders the complete `one.manifest.json` before and after applying the draft. A preview endpoint uses the same canonical serialization and project-patch logic as publication, while the Dashboard presents the result as one full-file unified diff with file headers, old/new line numbers, red `-` lines, green `+` lines, and every unchanged configuration line retained as context. The save-button count continues to count changed fields rather than contextual lines.

Deployment remains the visible parent of image/container configuration. When a deployment backend such as Kustomize requires a container artifact, render the image build switch, container backend, Profile, namespace, and image address inside the same deployment card below a dependency divider. Keep the underlying Manifest domains independent while avoiding a sibling card that implies deployment and image configuration have equal scope.

Use the top-level Dashboard environment selector as the only visible environment control. Deployment backend fields typed as `environment` are omitted from project deployment cards so users do not have to choose the same environment twice.
