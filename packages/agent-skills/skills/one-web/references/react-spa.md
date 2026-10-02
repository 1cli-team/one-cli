# React SPA and renderer runtime

The react-spa template uses Vite, React Router, an Axios fetcher, SWRConfig,
a Zustand UI theme store, semantic CSS tokens, and Base UI Toast. Keep routing
in the existing composition and add features when real domains warrant them;
a small screen does not need an empty architectural tree.

Read Vite's public environment prefix and base path from the target config.
Use only public values on the client; React SPA's optional API URL is VITE_API_URL.
Keep the workspace package manager and existing Oxlint/Oxfmt configuration.
After changing scripts, synchronize One task definitions when required.

Use the existing shared UI theme store and provider, keep persisted preferences
small, and avoid persisting server responses or introducing an auth store by
copying a starter from another project. Keep error and not-found states usable
with accessible recovery actions.

Electron renderer uses its own Vite APP environment prefix, relative assets,
and HashRouter for file-based production loading. Do not switch to BrowserRouter,
absolute asset URLs, or a custom protocol just because the browser template uses
them. Read native application information through window.electron.getAppInfo();
keep Node/filesystem access outside the renderer and update preload types with
main handlers when a requested feature needs another narrow method.

Run the target project's typecheck, lint/format, and production build tasks.
For Electron renderer changes, also verify the preload/main build dependencies
and assembled production resources; Vite development success alone does not
verify the packaged application.
