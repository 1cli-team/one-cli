# electron-app — Agent Guide

Cross-platform desktop app. Stack: **Electron + React 19 + Vite + TypeScript + shadcn/ui + Tailwind CSS v4 + Zustand (with immer) + SWR + sonner**.

The home screen at `apps/ui/src/nodes/HomePage/index.tsx` is intentionally minimal. Don't bring back demo galleries — extend by composing the pre-wired infrastructure below.

## Project layout

```
apps/
├── main/              # Electron main process (Node-side)
├── preload/           # Bridge between main and renderer
│   └── channels.ts    # IPC channel + event constants — IMPORT FROM HERE, never redefine
└── ui/                # Renderer process (React app)
    └── src/
        ├── App.tsx                        # Shell (header + routes)
        ├── main.tsx                       # Mount point
        ├── nodes/                         # Page-level (HomePage, AboutPage)
        │   └── HomePage/index.tsx         # The home screen
        ├── components/ui/                 # shadcn primitives — atoms
        ├── hooks/electron.ts              # useElectron() — IPC wrapper
        ├── lib/http.ts                    # Axios fetcher
        ├── lib/utils.ts                   # cn()
        ├── store/                         # Zustand stores (with immer)
        │   ├── app-store.ts               # Sample counter store
        │   └── index.ts                   # Re-exports
        └── styles/globals.css             # Tailwind entry + design tokens
```

## Pre-wired infrastructure — DO import, DON'T recreate

| Need | Where |
|------|-------|
| IPC client | `useElectron()` from `@/hooks/electron` — `electron.invoke(channel, ...args)`, `electron.send(...)`, `electron.on(event, cb)` |
| IPC channels | `IPC` from `@app/preload/channels` (e.g. `IPC.dialog.open`, `IPC.shell.open`, `IPC.contextMenu.show`) — DO NOT redefine |
| IPC events | `EVENT` from `@app/preload/channels` (e.g. `EVENT.update.downloadProgress`) |
| Store | `useAppStore` from `@/store` — zustand + immer; create new slices in `store/<feature>.ts` |
| HTTP client | `fetcher` / default export from `@/lib/http` |
| Toast | `import { toast } from "sonner"` — `<Toaster />` mounted by parent shell |
| Class merging | `cn()` from `@/lib/utils` |

## Atomic design (advisory — physical folders NOT enforced)

| Layer | Where | Examples |
|-------|-------|----------|
| atoms | `apps/ui/src/components/ui/` | Button, sonner toaster |
| molecules | `apps/ui/src/components/` | Compose atoms |
| organisms | `apps/ui/src/nodes/<Page>/` (alongside the page file) | Page-level sections |
| pages | `apps/ui/src/nodes/<Page>/index.tsx` | Route components |

## Design tokens — use Tailwind utilities, never hex/rgb

Tokens defined in `apps/ui/src/styles/globals.css` (oklch + shadcn convention).

| Concern | Use these classes |
|---------|-------------------|
| Surface | `bg-background`, `bg-card`, `bg-popover`, `bg-muted` |
| Text | `text-foreground`, `text-muted-foreground`, `text-primary` |
| Border | `border-border`, `border-input` |
| Accent | `bg-primary`, `bg-secondary`, `bg-destructive` |

❌ DON'T write hex/rgb. Add a CSS variable in `globals.css` first if you need a new color.

## Common patterns

**File dialog (IPC)**

```tsx
const electron = useElectron();
const files = await electron.invoke(IPC.dialog.open, { type: "file" });
```

**Open external URL**

```tsx
electron.invoke(IPC.shell.open, "https://github.com");
```

**Native context menu**

```tsx
const picked = await electron.invoke(IPC.contextMenu.show, [
  { key: "edit", label: "Edit" },
  { type: "separator" },
  { key: "delete", label: "Delete" },
]);
```

**Subscribe to a main-process event**

```tsx
useEffect(() => {
  const off = electron.on(EVENT.update.downloadProgress, (p) => setProgress(p));
  return off; // cleanup
}, [electron]);
```

**Counter store (with immer)**

```ts
// store/counter.ts
import { create } from "zustand";
import { immer } from "zustand/middleware/immer";

export const useCounterStore = create(immer<{ count: number; inc: () => void }>(
  (set) => ({ count: 0, inc: () => set((s) => { s.count += 1; }) }),
));
```

**SWR with public API**

```tsx
import useSWR from "swr";
import { fetcher } from "@/lib/http";
const { data, isLoading } = useSWR("https://api.github.com/zen", fetcher);
```

## Process boundary rules

- **Renderer (`apps/ui`)**: no Node APIs (`fs`, `path`, `child_process`). Always go through IPC.
- **Main (`apps/main`)**: no DOM, no React. Owns Node + Electron APIs.
- **Preload**: defines the safe IPC surface. Add a new channel here, then expose typed handlers in main, then call from renderer via `useElectron()`.

## Quality gates

```bash
pnpm lint
pnpm typecheck
pnpm build         # vite build for ui + electron-builder for main
```

All must pass before declaring a change complete.
