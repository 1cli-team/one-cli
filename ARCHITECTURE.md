# One CLI Architecture

One CLI is a Go multi-module monorepo. `packages/kernel` provides a small,
Cordis-inspired context and effect-ownership runtime; `packages/cli` owns the
product domains and executable. The kernel is a compiled dependency, not a
dynamic plugin runtime, and the migration preserves the current binary,
manifest, profile storage, and Dashboard contracts.

The design has two goals: backend capabilities must be composable, and every
temporary side effect must have an owner and a cleanup path.

## Public vocabulary

The public product vocabulary is intentionally small and hierarchical:

- Runtime: Workspace, Project, Environment, Backend, Profile
- Creation: Template
- Profile data: Credential

The runtime concepts are:

- Workspace
- Project
- Environment
- Backend
- Profile

`Template` exists only while a Project is created. `Credential` is a sensitive,
typed value inside a Profile; it is not a peer runtime concept.

`kind` in `one.manifest.toml`, `--provider` flags, and existing `PLUGIN_*`
error codes are compatibility surfaces. New code and documentation use
**backend** as the canonical term.

The built-in catalog currently contains 16 backends:

| Domain | Backends | Count |
| --- | --- | ---: |
| Environment | infisical | 1 |
| Deploy | aliyun-oss, tencent-cos, aws-s3, minio, rustfs, r2, kustomize, vercel, cloudflare, edgeone | 10 |
| Container | docker, dockerhub, ghcr, acr | 4 |

## Internal model

Cordis concepts are translated into explicit Go constructs:

| Cordis | One CLI |
| --- | --- |
| Context | `kernel.Context` ownership plus an immutable execution `Scope` |
| Component | a compiled-in backend adapter or a cohesive feature module |
| Service | an application service or a narrow contract under `internal/ports` |
| Inject / coeffect | backend requirements declared in the catalog |
| Effect / fiber | `kernel.Lifecycle`, an owned, idempotent cleanup stack |

The implementation deliberately does not provide a string-keyed service
locator, runtime package loading, hot module replacement, proxy properties, or
a generic event bus.

## Dependency direction

```text
packages/cli/cmd/one
  -> bootstrap                       unique process composition root
       |-> transport                 Cobra and Dashboard HTTP boundaries
       |-> modules                   cohesive feature slices
       |-> application               stable transport-neutral use cases
       |-> adapters                  Docker/cloud/environment/CI implementations
       |-> ports                     provider and runtime contracts
       |-> core                      backend/profile/template/workspace model
       |-> platform                  shared technical capabilities
       `-> resources                 immutable embedded assets

packages/cli -> packages/kernel     context and effect ownership only
packages/kernel -> Go standard library

transport   -> modules / application / ports / core / platform / resources
modules     -> application / adapters / ports / core / platform / resources
application -> ports / core / platform
adapters    -> ports / core / platform
ports       -> core / platform
core        -> platform / resources
```

`platform` and `resources` are leaves. `bootstrap` is the only layer allowed
to compose the complete CLI graph. `kernel` is independent of CLI domains,
transports, adapters, and vendor SDKs.

The physical Go workspace is:

```text
go.work                   activates the CLI and kernel modules

packages/kernel/
  go.mod                  independent kernel module
  README.md               package purpose and dependency boundary
  docs/                   architecture decisions and extension guidance
  internal/effect/        private cleanup-stack implementation
  pkg/kernel/             public Context and Lifecycle API

packages/cli/internal/
  bootstrap/
    cli/               constructs adapters and injects all dependencies
  core/
    backend/          backend identity, capabilities, profile type, and form policy
    profile/          machine profile model and persistence primitives
    template/         template registry and selection model
    workspace/        manifest and workspace model
  ports/
    secrets/          command environment loader contract
  application/
    configure/ execution/ workspace/
  adapters/
    env/ shared/ toolchain/ runtime/
  modules/
    creation/          Template-to-Workspace/Project materialisation
    development/       local development process orchestration
    environment/       Infisical variables and workspace bindings
    tasks/             finite task plans and environment snapshots
    miseconfig/        additive task and tool configuration
    hooks/             staged-file checks and Git integration
  platform/
    errors/ helpui/ i18n/ output/ preferences/ process/ prompt/ updatecheck/
  resources/
    bundled/           embedded templates, registry, and Dashboard
  transport/
    cobra/             one directory per command family
    http/              local Dashboard API

packages/cli/
  cmd/one/             thin executable entry point
  pkg/                 intentionally public Go packages
  testdata/            stable fixtures and compatibility snapshots
  tests/e2e/           binary and full-command contract tests
  tools/               repository verification programs

apps/dashboard/src/
  api/                 typed HTTP boundary and SWR keys
  architecture/        executable frontend dependency rules
  features/
    profile-editor/    Catalog-driven profile editing workflow
    project-settings/  Desktop project matrix and project inspector workflow
  pages/               route composition and page-specific presentation
```

Rules:

1. Transport packages own input parsing, prompts, and rendering. Reusable
   backend selection and execution policy belongs in application services or
   cohesive feature modules. Transports never import concrete adapters.
2. Application packages own use cases; `internal/ports` owns the narrow
   contracts implemented by adapters.
3. Adapters contain vendor and operating-system details. They do not import
   application, modules, transport, or bootstrap. Compatibility result and
   error types may use platform helpers.
4. Modules are vertical, compiled-in feature slices. They may orchestrate
   ports and adapters, but they do not depend on transport or bootstrap.
5. Platform packages contain process-wide technical concerns, not product
   feature policy. Resources contain immutable embedded data and import no
   other internal layer.
6. Execution scopes carry request state, not arbitrary services or
   credentials. `application/execution.Workspace` resolves the workspace
   boundary and manifest once per command; project selection reuses that
   snapshot instead of reading the manifest in each transport helper.
7. Credentials remain typed and are resolved immediately before the adapter
   call that needs them.
8. The backend catalog is the only source for backend identity, capabilities,
   requirements, profile type, credential-form metadata, safe project-setting
   fields, and secret-field disclosure policy. `core/profile` dispatches typed
   schema access by profile type; application workflows never maintain a
   second codec or project-form registry.
9. Cleanup is LIFO, idempotent, and best-effort. It covers local temporary
   resources; it does not pretend that an external cloud deployment is a
   reversible transaction.
10. Command, runtime, toolchain, and secrets provider sets are constructed explicitly.
    Provider packages do not register themselves through `init()`.
11. `internal/architecture/dependencies_test.go` enforces these boundaries for
    production Go files, including leaf-layer and transport/adapter rules.
12. `packages/kernel` imports only the Go standard library. It never owns
    Workspace, Project, Environment, Backend, Profile, or Template policy.
13. Workspace modules are listed explicitly in `go.work`; embedded Go template
    modules under `packages/templates` remain standalone template fixtures.
14. `packages/kernel` follows the applicable parts of the Go project-layout
    convention: public APIs live under `pkg/kernel`, private implementation
    lives under `internal`, and design notes live under `docs`. Executable and
    deployment directories are intentionally absent because Kernel is a
    library, not a standalone service.

## Transport flow

```text
Cobra / HTTP / Dashboard
          |
          v
application service / feature module
          |
          +---- validates Backend Catalog capability
          |
          +---- real extension seam --> typed port / provider registry
          |
          `---- compiled feature ----> built-in adapter
```

## Execution scoping

The root harness creates an immutable `execution.Scope` containing the process
context, working directory, and Kernel lifecycle. A command that requires a
workspace resolves that scope through `execution.ResolveWorkspace`:

```text
command context
      |
      v
execution.Scope                 working directory + lifecycle
      |
      v
execution.ResolveWorkspace     walk up to one.manifest.toml once
      |
      v
execution.Workspace            root + manifest snapshot + project lookup
      |
      +--> select Project by name or relative path
      `--> infer Project from the command working directory
```

Dev, configure, run, exec, add, and environment commands use
this same boundary. Helpers receive the snapshot rather than a root string that
would let them rediscover the workspace. A workflow that intentionally writes
`one.manifest.toml` must call `Workspace.Reload` before relying on the new
state. The bare root command keeps optional discovery because running `one`
outside a workspace renders help instead of producing a workspace error.

Creation is one Template-driven compiled workflow:

- `modules/creation.Service` is the single mutation boundary shared by ordinary
  `one create` and `one add`;
- workspace target revalidation, skeleton generation, Backend selection,
  environment preparation, Template rendering, manifest publication, project
  artifact generation and best-effort Git initialization
  stay behind that boundary;
- project creation synchronizes native mise tasks and derived environment paths;
- there is no top-level `modules/scaffold`: workspace-file generation is an
  implementation detail of creation, not another product concept;
- there is no cross-adapter `projectsync` package: orchestration stays beside
  the workflow that supplies its complete input;
- workload-name and kebab-case policy come from `core/workspace`, rather than
  being duplicated by adapter helper packages.

Dashboard Workspace reads and machine-local Profile selections enter through
`application/workspace.Service`. The service owns Overview construction,
Backend validation, Project lookup, Template compatibility, and
Profile-binding policy, but has no manifest-publication capability.
`one.manifest.toml` is a read-only fact source for that projection service:
the Project projection exposes its values, a SHA-256 revision, and resolved
Profile names/sources, never Profile values or credentials. Confirmed
Dashboard publication enters through the separate `application/manifest.Service`.
It accepts only typed workspace Infisical binding changes, compares the submitted
revision with the current file, and publishes through the atomic TOML writer.
Preview returns the actual before/after TOML, preserving user comments and
untouched text. Project settings are read-only: remote folders derive from
project paths, with shared and parent-folder inheritance always enabled.
Remote storage is initialized only on an explicit first variable save.
Stale drafts fail with `SERVE_MANIFEST_CONFLICT`; browser clients never submit
a replacement Manifest document. Environment-aware Workspace and Project
selections are stored in XDG-aware
`~/.config/one/profile-bindings.json` v1. The store is keyed by canonical
Workspace root and safe environment id, and its Workspace/Project maps contain
only `domain/backend -> Profile name` selections. Keeping the canonical root
in the key isolates two repository copies even when their manifests share one
Workspace id. The Dashboard UI exposes the binding's environment slugs (default `dev`, `staging`, and `prod`) as URL
state (`?env=`), not as a manifest migration; the core/API contract also
accepts safe custom environment ids for non-UI workflows. HTTP handlers only
decode requests, resolve the trusted Workspace root, map application errors,
and render application envelopes.
Historical manifest-mutation route paths fail closed with HTTP 409 and
`SERVE_REPOSITORY_READ_ONLY`; they never silently ignore a requested write.

Workspace discovery across invocations is a separate machine-local registry,
not Profile state and not Kernel state. `one create` observes a Workspace only
after successful creation; `one serve` observes the nearest manifest found by
walking up from its launch directory. Both update the XDG-aware
`workspaces.json` through `application/workspace.RegistryService` and the local
registry adapter. The registry stores only an opaque local entry id, manifest
identity, canonical root, display name, and observation timestamps. Projects,
Backend configuration, Profile values, and credentials are always read from
their authoritative stores.

The opaque local entry id is the Dashboard routing identity. Manifest
`workspace.id` cannot fill that role because copying a repository also copies
its manifest identity. Repeated observations of one canonical root are
idempotent; two live roots with one manifest identity remain separate and are
reported as a conflict instead of being silently re-keyed. Missing paths stay
visible until an explicit Forget operation. Plural `/api/workspaces/*` routes
resolve that opaque id server-side and revalidate the manifest before every
read, local Profile-binding mutation, Manifest publication, or secret mutation;
clients never submit an arbitrary filesystem root. Existing singular
`/api/workspace/*` routes remain pinned to
the launch Workspace for wire compatibility.

Finite workspace tasks are owned by `modules/tasks` and `modules/miseconfig`:

- `core/workspace.DiscoverTasks` reads pnpm scripts and Go Taskfiles. Commands stay in their native project files.
- `modules/miseconfig` generates additive mise fragments, tool pins, local dependency edges, and explicit cache declarations. User overrides stay in `mise.toml`.
- `modules/tasks` validates selection and dependencies, freezes each project's environment before cache lookup, then invokes mise once for the finite graph.
- Hidden task adapters consume the same temporary context for cache fingerprints and execution. Context values never enter generated configuration or result envelopes.
- mise owns task concurrency and artifact storage. One reports overall success/failure and leaves unsupported per-task event state unknown.
- `one build` and `one run build` share this implementation. `one exec` handles arbitrary commands, while development keeps the existing terminal supervisor after finite prerequisite builds.
- GitHub Actions files stay repository-owned and call the ordinary `ci` aggregate. Hooks remain supported.

Environment is a vertical deep module because its two built-in backends are
compiled implementation components rather than independently distributed
plugins:

- `modules/environment.Service` owns environment/backend resolution, profile
  resolution, project/path targeting, set planning, list/set/delete, explicit Dashboard reads,
  backend switching, manifest bookkeeping, and create-time environment setup;
- the module composes the Infisical adapter directly;
- create enters through `PrepareWorkspace`; Cobra does not sequence backend
  sync/bind functions, and the Infisical adapter exposes no no-op `Sync` API;
- `env set` enters through `PlanSet` and `Set`; the workspace resolution carried
  between those operations is private to the module;
- Cobra owns value/scope confirmations, flags, spinners, and rendering;
- there is deliberately no broad `environment.Runtime` or pass-through
  `WorkspaceSetup` port. A real extension point should be introduced only when
  a separately replaceable backend implementation exists.

Profile configuration uses the Backend Catalog as its identity-to-schema
boundary:

- every backend declares one internal `ProfileType` plus its JSON fields;
- each Catalog field owns its stable input name, requiredness, default,
  placeholder, control type, and disclosure type; Cobra derives flags and
  interactive inputs from those fields instead of repeating Backend switches;
- `core/profile` builds one typed schema-v1 section-policy table in persisted
  `Config` field order, validates it against every profile-bearing Catalog
  backend, and reuses it for CRUD, resolution, deterministic JSON emission,
  credential split/merge/stripping, section inspection, payload decoding, and
  credential-source access;
- Profile definitions/defaults remain in `config.json`, secrets remain in
  `credentials.json`, and legacy Workspace/Project selections in
  `config.json#workspaces` remain readable. The additive
  `profile-bindings.json` store does not change either schema-v1 file or
  `one.manifest.toml`;
- resolution is deterministic: one-shot flag, environment-aware Project
  selection, environment-aware Workspace selection, legacy Project selection,
  legacy Workspace selection, then machine default. Environment-aware keys use
  canonical Workspace root rather than manifest Workspace id;
- `application/configure` owns profile use cases, disclosure masking, and
  masked-secret preservation, but consumes the typed `core/profile` schema API
  instead of registering another codec table;
- HTTP masking follows fields marked `secret` in the Catalog, while the stricter
  CLI view masks every Catalog field below `credentials/`;
- application startup validates every configurable Catalog entry against the
  schema-v1 Go type and JSON field paths, so catalog/profile drift fails during
  composition instead of leaking into a request;
- kubeconfig context discovery is the only profile-form specialization in
  Cobra because its choices depend on a local file; it dispatches by
  `ProfileType`, then returns to the Catalog-driven typed decode path;
- adding a backend that reuses an existing profile type does not add Configure
  or profile-workflow switches. Extending the static profile-storage schema
  still requires its typed `Config` / `CredentialsFile` fields; introducing a
  genuinely new profile shape requires one schema policy factory.

The Dashboard loads `GET /api/catalog` once through an immutable SWR cache and
derives credential and project configuration fields from that response. Adding
a backend no longer requires duplicating backend lists and form switches across
the UI. `features/project-settings` owns the desktop project matrix, lazy
project-detail read, right-side inspector, read-only project configuration, and Project
Profile binding controls. `features/manifest-draft` keeps per-Workspace typed
workspace Infisical binding patches and human-readable differences in memory; the top bar is the only
publish affordance and requires a confirmation review. Profile binding saves
remain independent machine-local writes. Backend choices and backend-specific
Project fields come from the Catalog, while the server repeats all allowlist
and compatibility validation before publication.
`features/secrets` manages only `env/infisical` values at the server-derived
Workspace or Project folder. Lists contain key names without values; a value is
retrieved individually into component-local state and every response uses
`Cache-Control: no-store`. The error-state retry action may call the explicit
Backend initialization endpoint to repair a missing Infisical project binding
created by an older Dashboard or a hand-edited Manifest; secret reads never create a binding. The first explicit variable save may
initialize storage; values and variable-name registries never enter the Manifest
or its draft transaction.
The router preserves the selected environment slug in `?env=` across Workspace and Project links.
The global Settings page hides the selector because Profile definitions/CRUD
are machine-global rather than environment-scoped; preserved query state still
returns users to the same Workspace/Project binding namespace. The query does
not require or create a manifest environment.
`features/profile-editor` owns nested profile values, the Catalog-driven form,
dialog lifetime, upsert/toast behavior, and save notification. Routed pages
provide only an editor target and their own post-save cache refresh, so
`Overview` never imports another routed page.

Dashboard dependencies point inward: `router` composes `pages`, pages compose
features, and features may use API wrappers and shared UI primitives. Only the
router imports routed pages; features never import pages or the router. API
wrappers do not import presentation code, and `components/ui` remains a leaf
view layer. `src/architecture/dependencies.test.ts` enforces these rules and
rejects local TypeScript barrel entrypoints so imports stay directly
analyzable.

Repository verification has one public contract: `mise run check` (also exposed as
root `pnpm check`). It composes `check:static` and `check:test`; CI runs those
same two subtasks in parallel. `mise run pre-push` adds Go race detection without
creating a separate definition of the PR gate.

Node dependency resolution is likewise repository-owned: root
`pnpm-workspace.yaml`, `package.json`, and `pnpm-lock.yaml` are the only
workspace definitions for `apps/*`. Application packages declare their own
dependencies but do not carry a second lockfile or `packageManager` version.
Template packages are excluded from this rule because generated projects must
remain independently installable after leaving this repository.

## Compatibility boundary

Internal migration must preserve:

- command exit behavior, except for explicitly approved command removals and renames;
- `one.manifest.toml` v2 (the former JSON format is not supported);
- `~/.config/one/config.json` and `credentials.json` v1;
- the legacy profile resolution order, extended ahead of it by optional
  Project+Environment and Workspace+Environment bindings from the additive
  machine-local `profile-bindings.json` v1 store;
- structured success envelopes and `one-cli/error/v1` error codes;
- remaining public toolchain packages under `packages/cli/pkg`; the CI rendering contract has been removed.

New HTTP endpoints may be added. Existing read and Profile-management payloads
remain compatible; historical Dashboard routes that wrote a repository are a
deliberate safety exception and still return an explicit read-only error. The
supported repository writes are the typed, revision-checked workspace Infisical binding
draft endpoint and the revision-checked environment Backend switch endpoint.
