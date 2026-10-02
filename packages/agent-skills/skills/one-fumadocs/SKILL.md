---
name: one-fumadocs
description: Maintain Next.js and Fumadocs documentation projects in One workspaces, including MDX sources, localized navigation, static export, and search.
---

# One Fumadocs

Inspect the project's package.json, source.config, content tree, app routes,
and Next configuration. Resolve Next documentation from that project's
installed version. Fumadocs Core, MDX, and UI can have different version lines.

The One docs template uses `fumadocs-ui` aliased to `@fumadocs/base-ui`.
Existing workspaces can use the regular UI package. Follow the installed
provider, import paths, and components.json; do not switch UI primitives
while editing content.

Keep content and navigation synchronized in English and Chinese unless the
user scopes the change to a language. Match each language's metadata, links,
and routes. Use maintained source loaders and generated MDX types rather than
building a second content pipeline or checking generated output into Git.

For static-export projects, retain `output: 'export'`, generate required
localized route parameters, and build static search data. Do not introduce
a runtime search API or server-only feature that cannot work in the exported
site. Resolve links and search URLs with the actual base path and locale.

Run the project's checks/build using One tasks. Verify an exported page and
the generated search data for both languages when routing/search changes.
Business skills remain opt-in and live in separate repositories.
