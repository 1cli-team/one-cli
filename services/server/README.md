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
# Read the administrator token from stdin; the CLI saves it in a private file.
read -rs ONE_TOKEN
printf '%s' "$ONE_TOKEN" | one server add team https://one.example.com --token-stdin
unset ONE_TOKEN
one server use team
one server register /path/to/workspace
one server bind team --project api
```

CLI 仅使用 `$ONE_HOME/config/server-credentials.json` 单独保存 Server 令牌；Unix 上文件权限为 0600、目录权限为 0700，无需 Secret Service。文件以明文保存令牌，通过文件系统权限保护。普通连接配置、`one server list` 及管理导出不包含令牌。凭据文件损坏、版本过新、权限过宽或为符号链接时会报错并保留原内容；修复后重试。

CLI 不提供 keyring 选项，也不会读取旧钥匙串。旧连接若只在钥匙串中保存过令牌，使用上面的 `--token-stdin` 命令重新保存，或在 Desktop 的连接表单中重新输入。连接配置中旧的存储方式标记不会改变文件存储行为。

临时/CI 连接可注入 `ONE_SERVER_URL` 和 `ONE_SERVER_TOKEN`，环境令牌不会落盘；只指定 URL 时会读取已保存的凭据。Desktop 通过原生凭据入口复用同一文件，新连接也保存在此处。浏览器直接打开服务 URL，使用 HttpOnly 会话 cookie。远程 Server 无需安装 Secret Service；Infisical 凭据加密保存在服务端数据库中。

远程 Infisical 使用机器身份。可以在启动 Server 时注入下表中的环境变量，或通过对应的 `_FILE` 变量读取 Docker / Kubernetes 挂载的 Secret 文件。Client ID 与 Client Secret 必须一起提供；实例地址默认使用 `https://app.infisical.com`，组织 ID 可选。

| 环境变量 | 用途 |
|---|---|
| `ONE_INFISICAL_SITE_URL` | Infisical 实例根地址 |
| `ONE_INFISICAL_CLIENT_ID` | 机器身份 Client ID |
| `ONE_INFISICAL_CLIENT_SECRET` | 机器身份 Client Secret |
| `ONE_INFISICAL_ORGANIZATION_ID` | 可选组织 ID |

每项都支持 `_FILE` 后缀，例如 `ONE_INFISICAL_CLIENT_SECRET_FILE=/run/secrets/infisical_client_secret`。同一项不能同时提供非空的直接值和文件路径；空值视为未配置。文件中的首尾空白会被移除，内容必须是非空、不超过 16 KiB 的单行值。

部署注入优先于数据库中的旧身份。Server 启动时验证配置并认证，配置不完整、Secret 文件不可读或认证失败会直接报错，不会回退旧身份。注入的身份和访问令牌仅保留在内存中，过期后自动重新认证，不写入数据库；轮换 Secret 后需重启 Server。移除部署配置后，原有数据库配置仍可使用。

Desktop / Web 根据 Server 返回的模式和配置来源显示：部署管理的连接显示来源及只读状态，不提供修改或登出；相应 API 也拒绝修改。未注入配置时，远程 Server 保留机器身份表单及以下 CLI 配置方式，凭据加密入库；本地 Server 保留浏览器登录。CLI 将机器身份提交到选中的 Server，由 Server 连接 Infisical。

```sh
# Set ONE_INFISICAL_SECRET securely before running this example.
printf '%s' "$ONE_INFISICAL_SECRET" | one login --client-id <identity-client-id> --client-secret-stdin --organization-id <organization-id>
```

使用 Secret 文件的 Compose 部署，可先设置 `ONE_INFISICAL_CLIENT_ID`，并将 `ONE_INFISICAL_CLIENT_SECRET_PATH` 指向宿主机上的私有密钥文件，然后运行：

```sh
docker compose -f services/server/compose.yaml -f services/server/compose.infisical-secrets.yaml up --build -d
```

此覆盖文件将密钥挂载到容器的 `/run/secrets/infisical_client_secret`，不会把凭据构建进镜像。直接使用环境变量时，运行基础 Compose 文件即可。

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

CLI connections use only `$ONE_HOME/config/server-credentials.json`, separate from connection metadata and management exports. On Unix the plaintext credential file is protected with mode 0600 in a 0700 directory. No Secret Service is required. Invalid files, newer schemas, unsafe permissions and symbolic links fail without overwriting existing data. There is no keyring option or fallback, including for legacy profiles. If a token was previously saved only in a keyring, resave it with `--token-stdin` or Desktop's connection form. Old storage markers do not change file storage behavior.

Desktop uses the same file through its native credential bridge, including for new connections. `ONE_SERVER_TOKEN` overrides saved credentials without persistence; an address-only `ONE_SERVER_URL` override can reuse saved credentials. Remote Server processes do not require Secret Service.

Configure deployment-managed Infisical with `ONE_INFISICAL_SITE_URL`, `ONE_INFISICAL_CLIENT_ID`, `ONE_INFISICAL_CLIENT_SECRET` and optional `ONE_INFISICAL_ORGANIZATION_ID`. All four accept a `_FILE` suffix for mounted Secret files. A direct value and file path for the same setting cannot both be non-empty. Empty values count as unset; file contents are trimmed and must be a non-empty single line of at most 16 KiB. The default site is `https://app.infisical.com`; Client ID and Client Secret are required together.

Deployment inputs take precedence over saved database identities. Startup validates and authenticates them; errors never fall back to an old identity. Deployment credentials and access tokens remain only in process memory, with automatic reauthentication when a token expires. Restart after rotating inputs. Removing deployment inputs leaves the previous database configuration available.

Desktop and Web show the Server mode and configuration source. Deployment-managed identities are read-only in both the UI and API, including logout. Without deployment inputs, remote Servers accept machine identity configuration from the account page or `one login --client-id ... --client-secret-stdin --organization-id ...`, then authenticate and encrypt it in the database. Local Servers retain browser sign-in.

For Docker Secrets, set `ONE_INFISICAL_CLIENT_ID` and `ONE_INFISICAL_CLIENT_SECRET_PATH` (the private host file), then run the Compose command above with both files. The override mounts the credential at runtime and does not bake it into the image. For direct environment injection, use the base Compose file.

Exports include management metadata but exclude credentials, browser sessions, client paths and server identity. Import is atomic and requires an empty destination workspace store. Original local JSON configuration is backed up and imported once; the old files remain available for recovery. Keep the encryption key with your database backup.
