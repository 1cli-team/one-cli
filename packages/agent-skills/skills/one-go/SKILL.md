---
name: one-go
description: Develop Go API and library modules in One CLI workspaces, applying service-specific Gin/Gorm/Viper/Zap conventions and deliberate public library APIs.
---

# One Go

Read go.mod, Taskfile.yml, the One manifest, and the target module's entry points.
Discover tasks with `one run` and use the module's One tasks for checks, builds,
and tests. Respect the actual module identity and existing stack; an arbitrary
Go module or an empty template does not need to adopt Gin or Gorm.

## API services

The generated go-api lives under services/<module>, with cmd/server for the
binary and internal/ for private implementation. It uses Gin, Gorm, Viper, Zap,
and OpenAPI. Its initial routes are application information, database-independent
health, and documentation. It has no JWT, login, user model, CRUD, database driver,
default connection, or migration. Add domain packages only for requested behavior;
small handlers need not grow a transport/service/repository hierarchy.

Use functions and constructors with explicit dependencies. Keep assembly in
internal/app and HTTP setup in internal/http; place persistence behind the
business code that actually needs it. Avoid package globals for mutable service
resources or a DI container that obscures resource ownership.

Viper reads One-injected environment variables and optional module config.
Preserve its existing nested-key mapping; do not add an independent dotenv
loader or generate config files containing secrets. Keep Zap logging, request
IDs, panic recovery, exact CORS origin matching, and graceful HTTP shutdown.
Update api/openapi.yaml when real routes change.

Gorm is retained without a driver. When the user requests persistence, install
the driver for the selected database and pass its dialector to the existing
internal/platform/database.Open entry point. Own and close the resulting
connection explicitly, then inject it into the feature that needs it. Run
migrations as explicit tasks. Do not open an implicit in-memory database or
silently fall back when a configured connection fails.

## Libraries

The generated go-lib lives under packages/<module>. Keep public exports small,
with behavior tests near their package, and retain its Go module and import
identity. Its pkg/ layout is an existing convention, not a requirement for every
Go library. Do not introduce service dependencies, environment loading, Gin, or
Gorm into a library that does not use them.

For service routes, use httptest to verify externally observable status/body and
middleware behavior. For library APIs, use table-driven cases where inputs vary.
Run existing formatting, vet, tests, and build tasks; use the module's race
checks when changing concurrent ownership. Authentication and user CRUD belong
to requested business work, and business skills remain manually installed.
