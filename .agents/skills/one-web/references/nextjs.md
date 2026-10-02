# Next.js runtime and static export

Resolve Next from the target subproject and read its installed documentation
under node_modules/next/dist/docs when available. Read next.config and the actual
route/layout structure before applying upstream patterns. In the templates,
route entries are under src/app/; keep route-specific code colocated and shared
UI outside the route tree.

## Application runtime

nextjs-app supports a server runtime. Keep server data access and private values
in server code; NEXT_PUBLIC_API_URL is public and only configures the browser's
Axios client. Add client boundaries only where hooks, browser APIs, or user
interaction need them. Shared UI stores must be created per Provider rather
than as mutable module singletons reused by server requests. Keep initial server
and client state compatible during hydration. Use the existing next-themes
provider instead of duplicating theme state in Zustand.

Client SWR requests should use the configured fetcher and stable feature keys.
Do not add mock route handlers or authentication APIs as scaffolding for an
unrelated page. Server-only variables are not browser API configuration.

## Static sites and documentation

nextjs-site and fumadocs-docs use static export. Keep localized en/zh routes,
HTML language, navigation, theme, and SEO metadata consistent. NEXT_PUBLIC_SITE_URL
is public site metadata configuration. Server components can run at build time;
request-time cookies, server actions, runtime API handlers, and private dynamic
data need a deliberately chosen server deployment rather than an incidental
change to the output mode.

Fumadocs' api/search route generates the static search index during the build;
preserve it. It is part of the exported site's core behavior, even though it is
located under an API route directory. Content/search work follows the installed
one-fumadocs guidance.

Run typecheck, lint/format, and production build tasks. Check exported localized
pages and search after static content changes. When both Expo and Next projects
share a workspace, use explicit React type imports in reusable Web components
and verify against the target project's React types rather than a global React
namespace that may resolve to Expo's version.
