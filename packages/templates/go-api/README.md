# one-template-go-api

Gin · Gorm · Viper · Zap · OpenAPI · Taskfile.

## 中文

- `task run`：启动服务（默认端口 3000）。`task build`、`task check`：构建与静态检查。
- 提供 `GET /`、`GET /health`、`/api/docs`，没有登录、用户 CRUD 或默认数据库。
- 配置支持 `configs/config.yaml` 和环境变量，参见 `configs/config.example.yaml`。One CLI 注入的环境变量由 Viper 读取。
- Gorm 已保留。选择驱动后，显式调用 `database.Open(dialector)` 并将连接传给自己的业务代码；迁移单独执行。初始 HTTP 服务不创建连接。

## English

- `task run`: start on port 3000 by default. `task build`, `task check`: build and static checks.
- Exposes `GET /`, `GET /health`, and `/api/docs`, with no login, user CRUD, or default database.
- Configuration reads `configs/config.yaml` and environment variables; see `configs/config.example.yaml`. Viper reads variables injected by One CLI.
- Gorm is retained. Choose a driver, explicitly call `database.Open(dialector)`, and pass the connection to your business code. Run migrations separately. The initial HTTP service does not open a connection.
