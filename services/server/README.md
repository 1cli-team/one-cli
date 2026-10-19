# One Server

统一的管理服务：本地部署使用 SQLite，远程部署使用 PostgreSQL。CLI、Desktop、Web 调用相同的 `/api/v1`。服务端只管理数据与配置，本地任务仍由 CLI/Core 在客户端运行。架构与数据归属见根目录 [ARCHITECTURE.md](../../ARCHITECTURE.md)。

## Local / 本地

CLI 安装包包含 `one` 与 `one-api`。执行数据命令或打开 Desktop 时按需启动服务，无需单独启动 Desktop。

```sh
one server
one server register /path/to/workspace
one server bind local
one server export --file /path/to/management.json
```

本地数据库位于 `$ONE_HOME/data/one.db`（默认 `~/.one`）。保留 `config/server.key`：它用于解密数据库中的服务凭据。`state/server.json` 是私有服务发现文件，不应复制到远程主机。“移除工作区”只隐藏登记项，配置仍保留，再次登记后恢复显示。所有客户端退出后服务继续运行；前台运行可用 `one-api serve --local`。同一存储目录只允许一个本地实例。

## Remote / 远程

```sh
# Set these through your deployment secret manager or environment.
export ONE_DATABASE_URL='postgres://one:password@postgres:5432/one?sslmode=require'
export ONE_SERVER_PUBLIC_URL='https://one.example.com'
export ONE_SERVER_LISTEN='127.0.0.1:8080'
export ONE_SERVER_TOKEN="$(openssl rand -hex 32)"
export ONE_SERVER_ENCRYPTION_KEY="$(openssl rand -base64 32)"
one-api serve
```

通过 HTTPS 反向代理访问该端口。管理员令牌授权整个实例；当前不提供多用户权限隔离。加密密钥必须与数据库一起备份并在重启时保持一致。启动时自动应用数据库迁移，较新 schema 会拒绝由旧程序打开。

```sh
# Read the administrator token from stdin; the CLI defaults to a private file.
read -rs ONE_TOKEN
printf '%s' "$ONE_TOKEN" | one server add team https://one.example.com --token-stdin
unset ONE_TOKEN
one server use team
one server register /path/to/workspace
one server bind team --project api
```

CLI 默认使用 `$ONE_HOME/config/server-credentials.json` 单独保存 Server 令牌；Unix 上文件权限为 0600、目录权限为 0700，无需 Secret Service。文件以明文保存令牌，通过文件系统权限保护。普通连接配置、`one server list` 及管理导出不包含令牌。凭据文件损坏、版本过新、权限过宽或为符号链接时会报错并保留原内容；修复后重试。

需要系统钥匙串时，在 `one server add` 后添加 `--credential-store keyring`。旧连接继续使用钥匙串；执行 `one server add team https://one.example.com --credential-store file` 可将已有令牌复制到文件，其他同地址连接也会同步切换存储。旧项保留供其他安装使用；若原钥匙串不可用，通过 `--token-stdin` 重新保存即可。

临时/CI 连接可注入 `ONE_SERVER_URL` 和 `ONE_SERVER_TOKEN`，环境令牌不会落盘；只指定 URL 时会读取已保存的凭据。Desktop 新地址默认保存到钥匙串，也能复用 CLI 已保存的文件连接。浏览器直接打开服务 URL，使用 HttpOnly 会话 cookie。远程 Server 无需安装 Secret Service；Infisical 凭据加密保存在服务端数据库中。

远程 Infisical 使用机器身份：通过管理界面的账号页配置，或执行：

```sh
# Set ONE_INFISICAL_SECRET securely before running this example.
printf '%s' "$ONE_INFISICAL_SECRET" | one login --client-id <identity-client-id> --client-secret-stdin --organization-id <organization-id>
```

提供 `compose.yaml` 与多阶段 `Dockerfile`。在仓库根执行 `docker compose -f services/server/compose.yaml up --build -d`；先通过环境注入 compose 中要求的密码、令牌、密钥与外部 HTTPS URL。数据库不暴露宿主机端口。镜像同时包含由 Desktop React 源码构建的 Web 管理界面。

接口契约位于 `api/openapi.json`；连接后可通过 `GET /api/v1/openapi.json` 读取。

## Build and verify / 构建与验证

```sh
mise run server:web    # prepare embedded Web assets
mise run build:server # packages/cli/bin/one-api
mise run test:client
mise run test:management
```

`ONE_SERVER_WEB_ROOT` 可覆盖静态资源目录；生产打包前由 `server:web` 准备内嵌资源，普通 Go 开发构建可仅使用 API。设置 `ONE_TEST_POSTGRES_URL` 后，存储测试在该数据库内创建独立临时 schema，运行与 SQLite 相同的测试，再清理 schema。

## English

One Server owns management data and provider integrations. A local instance uses SQLite; a remote instance uses PostgreSQL. All clients share the same versioned API. Core executes tasks locally, including tasks whose project configuration comes from a remote server. Workspace identity and engineering declarations stay in Git, while managed bindings and preferences live in the database.

CLI and Desktop discover the same local service per `ONE_HOME`. Closing Desktop leaves that service running. Remote deployments require a database URL, a stable base64 32-byte encryption key, an administrator token of at least 32 characters, and an HTTPS public origin behind a reverse proxy. The token grants instance-wide access; this is a single-owner service. Remote provider access uses an Infisical machine identity. Browser sessions use HttpOnly cookies.

CLI connections default to `$ONE_HOME/config/server-credentials.json`, separate from connection metadata and management exports. On Unix the plaintext credential file is protected with mode 0600 in a 0700 directory, so Secret Service is optional. Invalid files, newer schemas, unsafe permissions and symbolic links fail without overwriting existing data. `--credential-store keyring` selects the system keyring. Existing profiles retain keyring behavior; re-adding the same URL with `--credential-store file` copies its saved token and switches all aliases to that store. Old credentials remain for other installations; if the original store is unavailable, resave with `--token-stdin`.

Desktop defaults to the keyring for new addresses and reuses the selected store for existing CLI connections. `ONE_SERVER_TOKEN` overrides saved credentials without persistence; an address-only `ONE_SERVER_URL` override can reuse saved credentials. Remote Server processes do not require Secret Service.

Exports include management metadata but exclude credentials, browser sessions, client paths and server identity. Import is atomic and requires an empty destination workspace store. Original local JSON configuration is backed up and imported once; the old files remain available for recovery. Keep the encryption key with your database backup.
