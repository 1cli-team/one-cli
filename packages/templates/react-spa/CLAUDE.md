# react-spa — Agent Guide

CSR (client-side rendered) React app. Stack: **React 19 + Vite + TypeScript + shadcn/ui + Tailwind CSS v4 + SWR + Zustand + sonner**.

The homepage at `src/pages/Home.tsx` is intentionally minimal (Vue/React-scaffold style). Don't bring back demo galleries — extend by composing the pre-wired infrastructure below.

## Project layout

```
src/
├── App.tsx              # Shell (header + routes + footer). Theme toggle lives here.
├── main.tsx             # Mount point + providers wiring
├── api/                 # Pure functions, "key + fetcher" shape (api/demo.ts is the canonical example)
├── components/
│   ├── ui/              # shadcn primitives — atoms (Button, Card, Badge, Input, Alert, Label)
│   └── ErrorBoundary.tsx # Wraps the router, catches render errors
├── hooks/               # Custom hooks (useToast)
├── lib/
│   ├── stores/          # Zustand slices (theme, toast)
│   ├── http.ts          # Axios instance — use as SWR fetcher
│   ├── toast.ts         # Sonner-backed toast singleton
│   ├── app-info.ts      # Read VITE_* env vars
│   └── utils.ts         # cn() classname merger
├── pages/               # Route-level components (Home.tsx is "/")
├── providers/           # SWRProvider, ThemeProvider
├── router/routes.tsx    # react-router-dom route table
└── styles/
    ├── tokens.css       # Design tokens — CSS variables, light + dark
    ├── tailwind.css     # Tailwind v4 entry
    ├── index.css        # Global styles entry
    └── reset.css
```

## Pre-wired infrastructure — DO import, DON'T recreate

| Need | Import |
|------|--------|
| Theme toggle | `useThemeStore` from `@/lib/stores/theme` (returns `{ mode, toggle, setMode }`) |
| Toast notifications | `useToast` from `@/hooks/useToast` — `toast.success / .info / .warning / .error` |
| HTTP client | default export from `@/lib/http` (axios) |
| Data fetching | `useSWR(key, fetcher)` — pair with `@/lib/http` |
| Error boundary | already wraps the router (`@/components/ErrorBoundary`) |
| Class merging | `cn()` from `@/lib/utils` |
| App metadata | `appInfo`, `getEnvironmentLabel` from `@/lib/app-info` |

## Atomic design (advisory — physical folders NOT enforced)

| Layer | Where | Examples |
|-------|-------|----------|
| atoms | `src/components/ui/` | Button, Card, Badge, Input |
| molecules | `src/components/` | Compose atoms — e.g. a SearchInput = Input + Button |
| organisms | `src/components/sections/` (create when needed) | Page-level blocks like NavBar |
| pages | `src/pages/` | Route-level components |

Don't dump unrelated logic in atoms. Don't import organisms from atoms (one-way dependency: atoms ← molecules ← organisms ← pages).

## Design tokens — use Tailwind utilities, never hex/rgb

Tokens live in `src/styles/tokens.css` as CSS variables (light + dark themes). The Tailwind classes below map to these variables — use them so theme switching just works.

| Concern | Use these classes |
|---------|-------------------|
| Surface | `bg-background`, `bg-card`, `bg-popover`, `bg-muted` |
| Text | `text-foreground`, `text-muted-foreground`, `text-primary` |
| Border | `border-border`, `border-input` |
| Accent | `bg-primary`, `bg-secondary`, `bg-destructive` |
| Semantic | `bg-success-surface` / `bg-info-surface` / `bg-warning-surface` / `bg-error-surface` (and matching `border-*` / `text-*-foreground`) |

❌ DON'T write `bg-[#ff0000]`, `text-blue-600`, or any hex/rgb. If you need a new color, add a CSS variable to `tokens.css` first.

## Common patterns

**Counter / stateful slice**

```ts
// src/lib/stores/counter.ts
import { createStore } from "@/lib/utils";
export const useCounterStore = createStore<{ count: number; inc: () => void }>(
  (set) => ({ count: 0, inc: () => set((s) => ({ count: s.count + 1 })) }),
  "counter",
);
```

**SWR data fetch**

```tsx
import useSWR from "swr";
import { demoKey, getDemo } from "@/api/demo";
const { data, error, isLoading, mutate } = useSWR(demoKey, getDemo);
```

**Toast notifications**

```tsx
const toast = useToast();
toast.success("Saved", { description: "Draft updated" });
toast.error("Failed", { description: err.message });
```

**Throw to ErrorBoundary**

```tsx
if (somethingWrong) throw new Error("...");  // Caught by ErrorBoundary, shows fallback UI
```

## Quality gates

```bash
pnpm lint          # oxlint
pnpm format        # oxfmt --check
pnpm build         # tsc -b && vite build  (runs typecheck)
```

All three must pass before declaring a change complete.
