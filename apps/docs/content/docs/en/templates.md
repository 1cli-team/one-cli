---
title: How to Choose Templates
description: Decision tree for the 10 base templates. Pick the right one in 30 seconds.
---

If you are adding a new project and do not know which base template to pick, this page gives you a decision tree and a comparison table.

If you would rather start from a complete workspace composition instead of choosing each `one add` template yourself, use [Template Examples](/en/templates/). That page has full starters for mobile, desktop, web, consumer, admin, and docs projects, each with a copyable `one create --preset ...` command.

**For**: people who ran `one templates` and saw too many IDs, tech leads evaluating stack choices, and anyone writing template-selection rules for agents.

**You will learn**: how to pick the right base template in 30 seconds, and when to skip the decision and use a complete example instead.

## 30-second Rule

```text
Need a backend API -----------------------> nestjs-api / go-api
Need a browser-facing web project --------> nextjs-app / react-spa / astro-site
Need a reusable package ------------------> ts-library / go-lib
Need a documentation site ----------------> starlight-docs
Need a mobile app ------------------------> expo-mobile
Need a desktop app -----------------------> electron-app
```

If unsure, ask one question: **how does the user consume this thing?** Browser -> Web. Command-line / HTTP calls -> API. `npm install` / `go get` -> Library. `.app` / `.dmg` / `.exe` -> Desktop. App Store -> Mobile. Reading content -> Docs.

## Full Comparison

| ID | Category | Keywords | One-line fit | Details |
|---|---|---|---|---|
| `nestjs-api` | API | TypeScript, NestJS, REST | Default API template for TypeScript teams | [Mobile / marketing / admin examples](/en/templates/) |
| `go-api` | API | Go, Gin, GORM | High-throughput / low-memory / mixed-language teams | - |
| `nextjs-app` | Web | Next.js, SSR, React | Default consumer web or full-stack app | [Consumer example](/en/templates/consumer-starter/) |
| `react-spa` | Web | Vite, React, SPA | Console / internal app / no SEO | [Admin example](/en/templates/admin-starter/) |
| `astro-site` | Web | Astro, static-first | Marketing or content site | [Marketing example](/en/templates/landing-starter/) |
| `starlight-docs` | Docs | Starlight, Astro | Documentation site or knowledge base | [Docs example](/en/templates/docs-starter/) |
| `expo-mobile` | Mobile | Expo, React Native | Cross-platform iOS + Android | [Mobile example](/en/templates/mobile-starter/) |
| `electron-app` | Desktop | Electron, React, Vite | Desktop app for macOS / Windows / Linux | [Desktop example](/en/templates/desktop-starter/) |
| `ts-library` | Library | TS, strict semver | Reusable TypeScript package | - |
| `go-lib` | Library | Go, module, package layout | Reusable Go module | - |

## Add The Template You Chose

The `ID` column is the first argument after `one add`.

If you are still unsure, use the interactive flow:

```bash
one add
```

If you already picked a template:

```bash
one templates
one add nestjs-api --name api
```

`nestjs-api` comes from the template ID. `api` is the project name you choose.

## Recommended Combos

### Full-stack SaaS (default)

```bash
one create my-saas
cd my-saas
one add nestjs-api --name api
one add nextjs-app --name web
one add ts-library --name shared
```

Why: TypeScript across the stack lets `shared` be used by both API and web. Next.js can cover SEO and authenticated app surfaces.

### High-performance Backend + Static Marketing

```bash
one add go-api --name api
one add astro-site --name marketing
one add react-spa --name console
```

Why: Go handles traffic; Astro keeps the public site fast and SEO-friendly; React SPA works for an authenticated console.

### Mobile + API

```bash
one add nestjs-api --name api
one add expo-mobile --name app
one add ts-library --name shared
```

`shared` can hold DTOs and business types reused by React Native and the API.

## Still Unsure?

Run `one templates -o json` for full template metadata, or run `one add` interactively. The picker includes category and one-line descriptions.

You can also pick one of the recommended combos above, get it running, and change course once you know more.
