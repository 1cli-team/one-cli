# Project layouts inside a One workspace

The outer convention is `apps/` for runnable clients/sites, `services/` for
backends/workers, and `packages/` for reusable libraries. Preserve existing
project structure and public import paths. Add directories when they hold
real code; do not generate empty architectural scaffolding.

Empty templates leave the project's internal layout and stack to the user.
The examples below apply only when the project uses that stack; they do not
require an empty project to adopt it. Follow the selected framework's layout
and the user's architecture while keeping the One workspace placement.

- **Go services:** keep binaries under `cmd/<binary>` and private code under
  `internal/`. Group handlers, application logic, persistence, and configuration
  according to the existing service. Use constructor/function injection where
  needed. Libraries should expose deliberate public packages; `pkg/` is not
  required for every Go module. Module paths follow the project's actual
  publishing/import identity. See the [official Go module layout guide](https://go.dev/doc/modules/layout).
- **Node services:** organize substantial business domains into modules that
  own their transport, application logic, and data access. Keep reusable
  infrastructure separate. For NestJS, follow its modules/providers and
  Drizzle when persistence is requested instead of transplanting Express directories.
  The minimal starter does not pre-create domains, repositories, or user models.
  See [NestJS modules](https://docs.nestjs.com/modules) and
  [Node Best Practices](https://github.com/goldbergyoni/nodebestpractices).
- **React SPA:** use `src/app` for composition/routing and `src/features/<domain>`
  when a domain warrants colocated API code, components, hooks, and state.
  Shared UI and utilities stay outside features. Avoid cross-feature cycles;
  do not move small existing applications just to match an example.
  See [Bulletproof React's layout reference](https://github.com/alan2207/bulletproof-react/blob/master/docs/project-structure.md).
- **Next.js:** routing belongs in `app/` or `src/app/`; preserve the chosen
  location. Keep route-specific code colocated, reusable business code in
  features/modules, and shared UI outside the route tree. Static sites and
  docs keep their static-export constraints.
- **Expo:** keep Expo Router route entries in `app/` or `src/app/` and reusable
  screens/components outside routing directories. Respect the committed
  layout; the upstream project-structure skill is for new unstructured apps.
- **TypeScript libraries:** keep public exports deliberate, implementation in
  `src/`, and tests near the behavior they cover. Preserve tsdown output,
  package exports, and the project's actual test convention.
- **Electron:** a desktop project is a group with `<name>-renderer` under
  apps, `<name>-main` under services, and `<name>-preload` under packages.
  Consult the installed `one-electron` skill for the process boundaries.

Upstream examples may use Make, ESLint, npm, or a different DI style. Reuse
One tasks, the workspace package manager, existing lint tooling, and existing
functional factories rather than creating competing build systems.
