# Adapt skills to the installed stack

Read the target project's package.json/go.mod, configuration, and task files
before applying upstream advice. Root skills may serve several projects;
choose references for the project being changed, not every installed skill.

Empty templates have no prescribed stack. When dependencies are not yet
defined, follow the user's choices rather than applying another template's
defaults. The adaptations below describe projects already using those stacks.

For template-specific work, select the installed adaptation skill:

- `one-nestjs`: NestJS services, Drizzle, and service startup.
- `one-go`: Go API services and public libraries.
- `one-expo`: native styling, development clients, and mobile data/state lifecycle.
- `one-web`: React SPA and Next.js; also Web UI in Electron and Fumadocs.
- `one-electron`: desktop process boundaries and packaging.
- `one-fumadocs`: documentation content, navigation, and static search.

Read the relevant skill and its references only when needed. If it is not
installed, inspect the project's README and existing implementation rather than
assuming its absence changes the stack. The brief adaptations below also apply
to existing workspaces that have only one-cli installed.

- **Web UI:** inspect components.json to identify shadcn style and primitive
  library. Generated web templates use Base UI; existing applications may
  still use Radix. Base UI composition uses `render`; do not copy Radix's
  `asChild` blindly. Generated templates use Base UI Toast, not Sonner.
  Preserve Tailwind version, theme, translations, and accessible interactions.
- **Next.js:** resolve `next` from the target subproject and consult its
  `node_modules/next/dist/docs` version-matched documentation when available.
  Do not rely on the retired next-best-practices package. Static-export
  projects cannot adopt server-only runtime behavior without a product change.
- **NestJS:** preserve Nest modules/providers, class-validator, Drizzle ORM/Kit,
  and Jest. The starter has no database connection or driver. Add a driver,
  schema, and migration tasks only after the user chooses persistence. An upstream ORM or logging
  example does not authorize replacing the project's data or log stack.
- **Expo:** the One template uses React Native styles, Axios/SWR, MMKV, and
  development clients for native dependencies. Do not introduce NativeWind,
  Tailwind, or replace the data stack merely because a skill prefers them.
  Check Expo SDK compatibility before adding a native dependency.
- **Go:** reuse current Gin/Gorm/Viper/Zap choices in services and the actual
  module API in libraries. Do not introduce Makefiles or another check runner.
- **Vite/Vitest/tsdown/pnpm:** use project-local versions and config; the root
  workspace owns package-manager settings. Keep Oxlint/Oxfmt when present.
  Do not install unrelated Vue/Nuxt/ESLint conventions from the same skill repo.

Keep credentials used by One/Infisical separate from application account
authentication. Business behavior is implemented only after the user requests
that feature; the starter templates do not contain JWT, login, user CRUD, or default
databases. Do not add these as prerequisite infrastructure.

Use Axios as a thin HTTP client and SWR for remote data/cache ownership. Do not
add token interceptors or duplicate SWR data in Zustand. Zustand is for shared
UI state; use React state locally and create Next.js stores per Provider rather
than as mutable server singletons. Expo's MMKV stores UI preferences, not a
second remote cache. Retain the chosen dependencies when removing demo code.
