# astro-site — Agent Guide

SSG (static site generation) Astro app. Stack: **Astro + TypeScript + Tailwind CSS v4 + custom toast / API utilities**.

The homepage at `src/components/Welcome.astro` is intentionally minimal (Vue/React-scaffold style). Don't bring back demo galleries — extend by composing the pre-wired infrastructure below.

## Project layout

```
src/
├── api/demo.ts              # Pure-function API examples (getDemoPost, createDemoPost)
├── components/
│   ├── ErrorBoundary.astro  # Mounted in Layout.astro; shows error UI in dev
│   └── Welcome.astro        # The homepage (rendered by pages/index.astro)
├── layouts/Layout.astro     # Page wrapper — toast container + ErrorBoundary + global script
├── pages/                   # File-based routes (.astro / .md / .mdx)
│   ├── index.astro          # Home
│   └── 404.astro
├── styles/
│   ├── tokens.css           # @theme — primary / success / warning / error / secondary palettes
│   └── global.css           # Tailwind entry + global resets
└── utils/
    ├── api.ts               # fetch wrapper (used by api/demo.ts)
    └── toast.ts             # showToast.success/info/warning/error, handleError
```

## Pre-wired infrastructure — DO import, DON'T recreate

| Need | Where |
|------|-------|
| Toast notifications | `import { showToast, handleError } from '../utils/toast';` (inside `<script>`) |
| HTTP / API calls | `import { getDemoPost } from '../api/demo';` |
| Global error boundary | `<ErrorBoundary />` in `Layout.astro` (already mounted) |
| Page wrapper | `<Layout title="..." description="...">` from `src/layouts/Layout.astro` |
| 404 page | `src/pages/404.astro` |

## Atomic design (advisory — physical folders NOT enforced)

| Layer | Where | Notes |
|-------|-------|-------|
| atoms | inline Tailwind in `.astro` markup | No UI lib by default; use Tailwind utilities |
| molecules | `src/components/` | Reusable .astro components (e.g. ResourceLink) |
| organisms | `src/components/sections/` (create when needed) | Page-level blocks |
| pages | `src/pages/` | File-based routes |

## Design tokens — use Tailwind utilities, never hex/rgb

Tokens defined via Tailwind v4 `@theme` directive in `src/styles/tokens.css`:

| Palette | Classes |
|---------|---------|
| Primary | `bg-primary-{50,100,200,300,400,500,600,700,800,900}` |
| Success | `bg-success-{50..900}` |
| Warning | `bg-warning-{50..900}` |
| Error | `bg-error-{50..900}` |
| Secondary (neutral) | `bg-secondary-{50..900}` — use for surfaces and text |
| Animations | `animate-fade-in`, `animate-slide-up`, `animate-bounce-slow` |

Same shape applies to `text-*-{50..900}` and `border-*-{50..900}`.

❌ DON'T write hex/rgb. Add a new CSS variable to `tokens.css` first if you need a new color.

## Common patterns

**Page with SSR-time data (build-time fetch)**

```astro
---
// src/pages/blog/[slug].astro
import { getDemoPost } from '../../api/demo';
const { slug } = Astro.params;
const post = await getDemoPost(slug);
---
<h1>{post.title}</h1>
```

**Client-side toast**

```astro
<button id="save-btn">Save</button>
<script>
  import { showToast, handleError } from '../utils/toast';
  document.getElementById('save-btn')?.addEventListener('click', async () => {
    try {
      await fetch('/api/save', { method: 'POST' });
      showToast.success('Saved', 'Your changes are stored.');
    } catch (err) {
      handleError(err, 'save');
    }
  });
</script>
```

**Layout composition**

```astro
---
import Layout from '../layouts/Layout.astro';
---
<Layout title="About" description="About us">
  <main>...</main>
</Layout>
```

## Astro idioms

- Frontmatter (`---`) runs at build time on the server. Keep heavy data fetching there.
- `<script>` blocks run in the browser; use them for interactivity.
- Mark interactive React/Vue/Svelte islands with `client:load` / `client:idle` / `client:visible` directives — but prefer plain HTML + small `<script>` for simple cases.

## Quality gates

```bash
pnpm check         # oxlint + oxfmt
pnpm build         # astro build (validates content + types)
```

Both must pass before declaring a change complete.
