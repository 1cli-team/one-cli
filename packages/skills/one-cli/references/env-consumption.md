# Consuming env vars in service code

`one env set` stores env vars and `one run` / `one dev` inject them
as **environment variables** (flat, POSIX-style names) into the child
process. Service code reads those env vars; **One CLI never generates
`config.json` / `config.yaml`**. Modern frameworks bind nested config
fields (`database.url`) to flat env vars (`DATABASE_URL`) at runtime —
let the framework do the mapping, don't have the CLI render files.

## Naming rule

`one env set` rejects keys that don't match the POSIX env-var pattern
(`^[A-Za-z_][A-Za-z0-9_]*$`). Use `DATABASE_URL`, not `database.url`,
even if the consuming framework reads it as `database.url`. The
mapping is the framework's job.

| Config tree path | Secret KEY to set |
|---|---|
| `database.url` | `DATABASE_URL` |
| `jwt.secret` | `JWT_SECRET` |
| `app.env` | `APP_ENV` |
| `cors.allowed_origins` | `CORS_ALLOWED_ORIGINS` (comma-split) |

The convention is `a.b.c` ↔ `A_B_C` — adopted by Viper (Go) and Spring
Boot relaxed binding. NestJS's `@nestjs/config` reads `process.env.*`
directly and maps to a `registerAs` namespace.

## Recommended service-side patterns

### Go (Viper)

The `go-api` template already wires this up
(`templates/go-api/internal/config/config.go`):

```go
v := viper.New()
v.SetConfigName("config")
v.AddConfigPath("./configs")
v.AutomaticEnv()
v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
_ = v.ReadInConfig()

dbURL := v.GetString("database.url") // env DATABASE_URL takes priority
```

### NestJS

The `nestjs-api` template uses `registerAs` factories
(`templates/nestjs-api/src/core/config/configuration.ts`):

```ts
export const databaseConfig = registerAs("database", () => ({
  url: process.env.DATABASE_URL,
}));

// in a service:
configService.get("database.url");
```

### Plain Node

Read `process.env.<NAME>` directly. dotenv loading is already wired by
the workspace; no additional setup.

## What to tell users who ask "How do I get a config.json?"

Don't generate one. Walk them through:

1. `one env set <KEY>=<value> -p <project>` for each value (uppercase,
   underscore-separated).
2. Make sure the service consumes via the framework's env-aware loader
   (Viper `AutomaticEnv`, NestJS `@nestjs/config`, etc.).
3. The committed `config.example.yaml` / `application.yml` stays as
   schema documentation + local default, not as a render target.

If the service is hand-written `encoding/json.Unmarshal` against a
file: refactor it to use Viper or equivalent. That's a one-time service
change, not a CLI feature.

## Out of scope (separate concerns)

- **`one container build`** must NOT bake env vars into the image.
  Inject env vars at runtime (k8s `Secret` / `ConfigMap`, `docker run -e`).
- **`one deploy`** to k8s: env vars reach the cluster as `Secret`
  / `ConfigMap` resources, not via this skill. Sync flow is a separate
  feature.
