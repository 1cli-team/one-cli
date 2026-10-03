# NestJS API

NestJS · TypeScript · Drizzle ORM/Kit · class-validator · Swagger · Pino · Jest.

## 中文

初始项目只提供 `GET /`、`GET /health` 和 `/api/docs`，无需数据库或认证配置即可运行。

- `pnpm start:dev`：开发；`pnpm build` / `pnpm start:prod`：构建与启动。
- `pnpm check`：类型、代码与格式检查。
- 使用 One CLI 注入 `PORT`、`ALLOWED_ORIGINS`，参见 `.env.example`。
- 已保留 Drizzle。需要数据持久化时，先选择并安装驱动，再编写 schema、连接和 Drizzle Kit 配置。迁移由独立任务显式执行。
- 业务功能与认证通过手动安装业务 skills 或项目代码添加。

## English

The starter exposes only `GET /`, `GET /health`, and `/api/docs`. It starts without database or authentication settings.

- `pnpm start:dev`: development; `pnpm build` / `pnpm start:prod`: build and run.
- `pnpm check`: type, code, and formatting checks.
- Inject `PORT` and `ALLOWED_ORIGINS` with One CLI; see `.env.example`.
- Drizzle is retained. Choose and install a driver before adding your schema, connection, and Drizzle Kit configuration. Run migrations as explicit, separate tasks.
- Add business features and authentication through manually installed business skills or application code.
