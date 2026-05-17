# nextjs-app — Agent Guide

SSR / RSC Next.js app with App Router. Stack: **Next.js 16 + React 19 + TypeScript + shadcn/ui + Tailwind CSS v4 + next-themes + SWR + Zustand + sonner + axios**.

The homepage at `src/app/page.tsx` is intentionally minimal (Vue/React-scaffold style). Don't bring back demo galleries — extend by composing the pre-wired infrastructure below.

## Project layout

```
src/
├── api/                 # Pure functions + request keys (e.g. api/hello.ts: helloKey + getHello)
├── app/                 # App Router segments
│   ├── layout.tsx       # Root layout — ThemeProvider + <Toaster />
│   ├── page.tsx         # Home (this is the welcome screen)
│   ├── api/<feature>/route.ts # Route handlers (server-side endpoints)
│   ├── global-error.tsx # Error boundary at the app root
│   └── globals.css      # Tailwind entry + @theme inline mapping
├── components/
│   ├── ui/              # shadcn primitives — atoms
│   ├── theme-provider.tsx, theme-toggle.tsx  # next-themes integration
│   └── (your own)       # molecules / sections
├── lib/
│   ├── http.ts          # Axios instance (client-side SWR fetcher)
│   ├── server-data.ts   # Server-side data helper (use in async server components)
│   └── utils.ts         # cn() + createStore() (zustand + devtools)
├── stores/              # Zustand slices (create per-feature)
└── styles/tokens.css    # Design tokens — CSS variables, light + dark
```

## Pre-wired infrastructure — DO import, DON'T recreate

| Need | Where |
|------|-------|
| Theme toggle | `next-themes` via `<ThemeProvider>` in `app/layout.tsx`; UI in `components/theme-toggle.tsx` |
| Toast notifications | `import { toast } from "sonner"`; `<Toaster />` is mounted in layout |
| Client HTTP | `http` from `@/lib/http` (axios) |
| Server HTTP | `getHello`, etc. from `@/api/<feature>.ts` (use in server components or route handlers) |
| SSR data | `getHelloData` from `@/lib/server-data` — call inside an `async` server component |
| Store helper | `createStore(creator, name)` from `@/lib/utils` — wraps zustand + devtools in dev |
| Global error | `app/global-error.tsx` (Next's RSC error boundary) |
| Class merging | `cn()` from `@/lib/utils` |

## Server vs client components

- Default to **server components**. Add `"use client"` only when you need state, effects, or browser APIs.
- SWR, Zustand, sonner all require `"use client"` — split files at the boundary.
- Don't import server-only utilities (`fs`, `next/headers`) into client components.

## Atomic design (advisory — physical folders NOT enforced)

| Layer | Where | Examples |
|-------|-------|----------|
| atoms | `src/components/ui/` | Button, Card, Badge, Input |
| molecules | `src/components/` | Compose atoms |
| organisms | `src/components/sections/` (create when needed) | Page-level blocks |
| pages | `src/app/<route>/page.tsx` | App Router pages |

## Design tokens — use Tailwind utilities, never hex/rgb

Tokens are defined in `src/styles/tokens.css` (oklch color space) and imported via `@theme inline` in `globals.css`.

| Concern | Use these classes |
|---------|-------------------|
| Surface | `bg-background`, `bg-card`, `bg-popover`, `bg-muted` |
| Text | `text-foreground`, `text-muted-foreground`, `text-primary` |
| Border | `border-border`, `border-input` |
| Accent | `bg-primary`, `bg-secondary`, `bg-destructive` |
| Sidebar | `bg-sidebar`, `text-sidebar-foreground` etc. |

❌ DON'T write hex/rgb. If you need a new color, add a CSS variable to `tokens.css` first.

## Common patterns

**Server component with SSR data**

```tsx
// app/page.tsx
export default async function HomePage() {
  const data = await getHelloData();
  return <pre>{JSON.stringify(data, null, 2)}</pre>;
}
```

**Client component with SWR**

```tsx
"use client";
import useSWR from "swr";
import { helloKey, getHello } from "@/api/hello";
export default function ClientHello() {
  const { data, error } = useSWR(helloKey, getHello);
  // ...
}
```

**Counter store**

```ts
// src/stores/counter.ts
import { createStore } from "@/lib/utils";
export const useCounterStore = createStore<{ count: number; inc: () => void }>(
  (set) => ({ count: 0, inc: () => set((s) => ({ count: s.count + 1 })) }),
  "counter",
);
```

**Toast**

```tsx
"use client";
import { toast } from "sonner";
toast.success("Saved", { description: "Draft updated" });
```

**Route handler**

```ts
// app/api/hello/route.ts
import { NextResponse } from "next/server";
export async function GET() {
  return NextResponse.json({ message: "hello" });
}
```

## Quality gates

```bash
pnpm lint          # oxlint
pnpm check         # lint + format
pnpm build         # next build (runs typecheck)
```

All must pass before declaring a change complete.
