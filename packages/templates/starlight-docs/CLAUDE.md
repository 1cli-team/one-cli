# starlight-docs — Agent Guide

Astro + Starlight documentation site. Stack: **Astro + Starlight + MDX + TypeScript**.

This template is for **content**, not UI engineering. The homepage and component layout are managed by Starlight's defaults — don't restructure them, write docs.

## Project layout

```
src/
├── content/docs/           # All documentation lives here
│   ├── index.mdx           # Landing page (Starlight splash template)
│   ├── guides/             # Step-by-step guides
│   └── reference/          # Reference docs (auto-sidebared)
├── styles/custom.css       # Starlight theme variables — customize here, NOT inline in pages
└── content.config.ts       # Content collection schema (Starlight defaults)

astro.config.mjs            # Site title, sidebar, social links — primary config surface
public/                     # Static assets (favicons, images)
```

## What an agent should NOT do

- ❌ Don't add custom React/Vue components unless absolutely necessary — use Starlight's built-in components (`<Card>`, `<CardGrid>`, `<LinkCard>`, `<Tabs>`, `<Steps>`, `<Aside>`).
- ❌ Don't hand-roll a homepage or navigation. Use the splash template + `astro.config.mjs` sidebar.
- ❌ Don't introduce Tailwind into doc pages. Starlight has its own theming via `src/styles/custom.css`.
- ❌ Don't write code samples without language hints — Markdown code fences need ```ts, ```bash, etc. for proper highlighting.

## What an agent SHOULD do

- ✅ Write `.md` / `.mdx` under `src/content/docs/`. Frontmatter required: `title`, `description` (recommended).
- ✅ Add new sidebar entries in `astro.config.mjs` under `starlight.sidebar`.
- ✅ Use Starlight's built-in components for callouts, tabs, steps, link cards.
- ✅ Customize colors / typography via CSS variables in `src/styles/custom.css`.
- ✅ Cross-link with relative paths: `[Setup guide](/guides/setup)`.

## Frontmatter pattern

```mdx
---
title: Setup
description: How to install and configure the toolkit.
---

import { Aside } from '@astrojs/starlight/components';

# Setup

<Aside type="tip">
  Use `pnpm dlx` to try without installing.
</Aside>
```

## Sidebar configuration

Edit `astro.config.mjs`:

```js
sidebar: [
  { label: 'Guides', autogenerate: { directory: 'guides' } },
  { label: 'Reference', autogenerate: { directory: 'reference' } },
],
```

## Quality gates

```bash
pnpm check         # lint + format
pnpm build         # astro build (validates frontmatter + links)
```

Both must pass before declaring a change complete.
