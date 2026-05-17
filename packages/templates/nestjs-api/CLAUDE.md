# nestjs-api — Agent Guide

NestJS-based API service. Stack: **NestJS + TypeScript (strict) + class-validator + Drizzle ORM + Jest + structured logging**.

## Project layout

```
src/
├── main.ts                   # bootstrap
├── app.module.ts             # root module
├── core/
│   ├── config/               # ConfigModule + configuration registry
│   ├── exceptions/           # BusinessException + error codes
│   ├── filters/              # HttpExceptionFilter (consistent JSON response)
│   ├── interceptors/         # logging, response shaping
│   └── guards/, pipes/       # cross-cutting concerns
├── modules/<feature>/
│   ├── <feature>.module.ts   # declarative wiring only
│   ├── <feature>.controller.ts # HTTP routing only — ≤ 50 lines per controller
│   ├── <feature>.service.ts    # business logic — stateless singleton
│   ├── dto/                    # DTO classes with class-validator decorators
│   └── *.spec.ts               # unit tests (next to source)
├── dao/repositories/         # The ONLY layer touching Drizzle / DB
└── shared/utils/             # pure helpers
test/                         # *.e2e-spec.ts — real HTTP layer
```

## Architecture boundaries — NEVER violate

- **Controller**: HTTP routing + parameter parsing only. ≤ 50 lines. No business logic, no DB access. Inject services and call them.
- **Service**: All business logic lives here. Stateless, singleton scope. Inject repositories and other services. Never reach into HTTP, never call another controller.
- **Repository** (`src/dao/repositories/`): The only layer that touches Drizzle / the database. Returns typed entities, never raw rows. Never expose ORM types upward.
- **Module** (`src/modules/<feature>/`): Declarative wiring only. No logic in the module file itself.
- Cross-cutting concerns (auth, logging, validation, metrics) belong in **Interceptors / Guards / Pipes / Filters** — never sprinkled across services.

## Engineering discipline — mandatory

These steps are mandatory before declaring any change "done". Not suggestions.

1. `pnpm typecheck` exits 0
2. `pnpm lint` exits 0
3. `pnpm test` passes — and any new code must come with new tests
4. Stage files explicitly with `git add <file>`. Never `git add -A` — it picks up `.env`, `dist/`, debug files.
5. Conventional commit message: `feat(auth): add JWT refresh endpoint`. One commit = one logical change.
6. Never commit secrets. Use `one secrets set <KEY> --env <env>`.
7. Run any destructive operation with `--dry-run` first.

If any fails, stop. Read the failure, fix the underlying problem, then re-run.

## Testing conventions

- Unit tests: `<name>.spec.ts` next to source. Use `Test.createTestingModule` from `@nestjs/testing`.
- Mock external dependencies (DB, HTTP, FS) — never real IO in unit tests.
- **Do NOT mock your own services**. If a test "needs" to mock another service in the same module, the design is wrong — fix the design.
- E2E tests: `test/*.e2e-spec.ts` — real HTTP layer.
- Every new controller endpoint requires: 1 happy path + 1 401/403 (if guarded) + 1 400 (validation error).
- Test names: `should <do something> when <condition>`.
- Coverage target: `src/modules/` 80% branch, `src/dao/` 60%.

## Code style

**DTOs and validation**

- Every external input must be a DTO class (not an inline `{ a: string }`).
- Validate with `class-validator`: `@IsString()`, `@IsEmail()`, `@IsInt() @Min(0)`, etc.
- Output is a DTO or typed response interface — never return raw entities (entities leak `password_hash`, internal FKs, etc.).

**Error handling**

- Domain errors → `BusinessException(code, message, context?)` from `src/core/exceptions/`. Code is machine-readable, message is for humans, context is structured.
- HTTP errors → NestJS built-ins (`NotFoundException`, `BadRequestException`, `UnauthorizedException`).
- `HttpExceptionFilter` normalizes all errors. Trust it — don't write your own response shaping in controllers.
- ❌ NEVER `throw new Error("...")` (no code, no recovery hint).
- ❌ NEVER `try { ... } catch (e) { /* swallow */ }`. Every error either handled meaningfully or rethrown.

**Logging**

- Inject NestJS `Logger` (or pino if configured). NEVER `console.log/error/warn`.
- Structured logs: `logger.log({ user_id, action: 'auth.login' }, 'user logged in')` — not `'user ' + id + ' logged in'`.
- Hot path code: don't log large objects.
- Levels: `debug` for dev, `log/info` for business events, `warn` for recoverable anomalies, `error` for unrecoverable.

**Types**

- TypeScript `strict` mode is on. Keep it.
- ❌ NEVER `any`. Use `unknown` and narrow.
- Public methods must have explicit return types.
- `type` for unions/intersections; `interface` for classes/extension.

**Configuration**

- Inject `ConfigService` from `@nestjs/config`. NEVER `process.env.X` in business code.
- Every env var registered in `src/core/config/configuration.ts` with default + validation.
- Secrets always go through `one secrets`, never raw `.env`.

## Anti-patterns — do not do these

- ❌ SQL strings in services. Use the repository.
- ❌ Controller calling another controller. Inject the relevant service.
- ❌ `process.env.X` in business code. Use `ConfigService`.
- ❌ Hand-rolled DI (passing dependencies through constructors manually). Use NestJS providers.
- ❌ Returning `null` to mean "not found" without context. Throw a typed exception.
- ❌ Catching errors and returning `null` / `undefined` / `false`. Errors must propagate with context.
- ❌ `console.log` in committed code.
- ❌ Tests that mock the system under test.
- ❌ Skipping validation because "this endpoint is internal".

## Quality gates — every box checked before declaring done

- [ ] `pnpm typecheck` exits 0
- [ ] `pnpm lint` exits 0
- [ ] `pnpm test` passes
- [ ] New code paths have new spec files
- [ ] No `console.log/error` introduced
- [ ] No `any` introduced
- [ ] No swallowed errors
- [ ] No `process.env` reads outside `core/config`
- [ ] Commit message conventional
- [ ] `git status --short` shows only intentional files
- [ ] No `.env` / `dist/` / `*.log` / `.DS_Store` staged
