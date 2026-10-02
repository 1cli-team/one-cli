---
name: one-web
description: Develop React SPA and Next.js App/Site projects in One CLI workspaces, including shared Web UI in Electron renderers and Fumadocs, with Base UI and clear data/state ownership.
---

# One Web

Read the target package.json, components.json, build config, and One manifest.
Use the user's project name under apps/; discover tasks with `one run` and verify
through the project's One tasks. Actual dependencies and user choices override
template defaults. This skill does not prescribe a stack for empty templates or
React Native apps.

Generated Web projects use shadcn Base UI, Tailwind CSS, semantic design tokens,
and Base UI Toast. Inspect components.json before composing UI: Base UI uses
`render`, and Radix `asChild` examples cannot be copied mechanically. Existing
applications may use Radix; preserve their actual primitive library. Add only
components the requested screen needs and preserve keyboard/focus behavior,
accessible labels, and light/dark styling. Reuse the mounted Toaster and its
manager; do not add Sonner, another notification layer, or a toast-history store.

Where Axios, SWR, and Zustand are installed, retain their distinct roles:
Axios handles HTTP and its fetcher returns response data; SWR owns remote data,
cache, and revalidation; Zustand owns shared UI state. Use React for local state.
Do not duplicate SWR results in Zustand or invent business-code/token wrappers.
Do not add these dependencies to static content projects merely for consistency.
Authentication, user models, and business APIs are requested features, not
starter infrastructure; business skills are installed manually.

Read only the reference for the current runtime:

- [React SPA](references/react-spa.md): Vite, routing, client state, and renderer adaptations.
- [Next.js](references/nextjs.md): server/client boundaries, per-Provider stores, and static export.

Electron renderer changes must also respect the installed one-electron guidance
and typed preload contract; privileged capabilities belong in main. Fumadocs
content, navigation, MDX, and search changes use one-fumadocs guidance when
installed. Framework-specific skills complement this skill rather than replacing
the project's One conventions.
