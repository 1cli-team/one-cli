# One Desktop Implementation Plan

Goal: implement the approved Tauri 2 + existing React + Go CLI architecture. React calls authenticated loopback HTTP directly; Rust owns native integration and the backend lifecycle. The 2026-10-13 Tauri design is the architectural reference; the earlier Wails document is superseded.

Architecture: `apps/desktop` contains React and the conventional `src-tauri` host. The application bundles `one-api` from `services/desktop-api` and `one` from `packages/cli`, sharing business code from `packages/one-core`. One long-lived Go API per native application, random port and session credentials delivered only through a private pipe, browser HTTP, and a separately installed terminal CLI. No generic Rust business proxy. Existing workspace data and keyring remain owned by Go. Workspace tasks run only through the terminal CLI; the bundled CLI supplies terminal installation.

Tech stack: Tauri 2, Rust, React/Vite, Axios/SWR, Go and existing One task execution.

## Implementation sequence

1. Go desktop entry, control protocol, authenticated HTTP, session readiness and workspace registration. Remove SPA embedding and turn `one serve` into a bilingual migration error.
2. Tauri host and native configuration: bundled executable identity, startup timeout, bounded restart, graceful shutdown, single instance, folder selection, external links, restricted capabilities and CSP.
3. React connection gate, generation changes/cancellation, HashRouter, workspace opening, bilingual desktop settings and first-run terminal integration.
4. Go installation transactions shared with the script entrypoint; protect bundled executables from self-update. Preserve unknown or newer terminal installations, expose PATH state separately.
5. Keep task execution in the terminal CLI. Desktop task UI, task HTTP routes, SSE logs and private task workers were removed at the user's request.
6. Reproducible sidecar/native packaging, pinned dependencies, updater configuration and transaction health. Production updates stay disabled until trusted signing configuration and platform recovery acceptance exist.
7. Update build tasks and bilingual migration/development documentation. Verify Go build without web assets, existing repository checks, Rust formatting/checks and a real macOS package. Exercise loopback security and lifecycle using isolated configuration; do not install into the developer's real CLI path.

## Verification and delivery boundaries

Use existing quality gates (`mise run check`) and actual runtime/build checks; do not add test-only code or mock acceptance scripts. Do not publish tags or packages, create signing keys, replace the developer's CLI, or claim Windows/Linux runtime validation from a macOS build. Record remaining external signing/platform acceptance prerequisites in the desktop development guide.

## Directory migration validation (2026-10-13)

- React and Tauri use `apps/desktop/src` and `apps/desktop/src-tauri`; project tasks use `desktop`.
- `one-core/pkg` contains shared APIs and services. Private adapters and embedded resources use `one-core/internal`; external consumers do not import them.
- Imports, Go module dependencies, resource generation, release linker symbols, task discovery and development documentation use the new paths.
- `mise run check` passed on macOS arm64: Go vet/format, React lint/format, Rust fmt/check, documentation and existing release rules.
- `mise run build-desktop` produced a local 61.83 MiB macOS `.app`, including both Go programs with matching build identities. This is a development-channel package, not a signed/notarized production release.
- Isolated runtime checks passed for bootstrap, authentication/generation, Host/Origin/CORS, no SPA, quiesce, stdin EOF shutdown, and CLI installation into a temporary directory. The developer's installation and keyring were not used.
- Desktop release publishing/feed collection, script installer convergence, complete platform recovery and Windows/Linux runtime acceptance remain outstanding; the implemented updater remains gated accordingly.

## Desktop task removal (2026-10-13)

- Removed the task navigation/page, xterm dependency, authenticated log streaming, task HTTP routes, task process manager and private CLI worker entry. Workspace task execution remains in the terminal CLI.
- `one check --ui stream` passed. Both language dictionaries have matching keys after removing task-specific translations.
- An isolated backend session returned 404 for all six former task routes and 200 for session readiness, then stopped cleanly on stdin EOF. The rebuilt CLI still resolves `one run build --dry-run`; its private desktop task worker entry is no longer available.
