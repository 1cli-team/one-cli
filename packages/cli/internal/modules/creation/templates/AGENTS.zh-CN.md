# 工作区开发约定

## 了解工作区

- 从当前目录向上查找最近的 `one.manifest.toml`，确认项目名称、路径和工具链。
- `apps/` 存放应用，`services/` 存放后端服务与 worker，`packages/` 存放共享库。
- 修改项目之前，阅读相关的 `README.md` 和适用的 `AGENTS.md`。
- 使用 `one --help` 和具体命令的 `--help` 确认用法，不猜测命令或参数。添加项目时先用 `one templates -o json` 查询模板，再使用 `one add`。

## 统一执行入口

- 测试、检查、构建、依赖准备和项目脚本必须通过 `one run <任务>` 执行；项目任务使用 `one run <任务> -p <项目>`。
- 先用 `one run` 查看已有任务，优先复用。不直接运行 `pnpm test`、`go test`、`node`、`docker` 或 `mise run` 等命令绕过任务入口；任务内部可以调用这些原生工具。
- 缺少需要的任务时，先在项目的 `package.json`、`Taskfile.yml` 或根 `mise.toml` 中定义。修改项目任务后，按需运行 `one init mise` 同步，再通过 `one run` 执行。
- One CLI 的帮助、任务查询、项目创建和变量管理命令可以直接使用，无需包装成任务。
- 修改完成后，通过 `one run` 执行受影响范围的检查、测试和必要构建；任务名以实际配置为准。

## 复杂脚本

- 多步骤、复杂分支或反复使用的操作，可以编写为 `scripts/<名称>.mts`，用 TypeScript 表达，避免拼接长串 shell 命令。
- 跨项目脚本放在工作区根目录的 `scripts/`，仅服务于一个项目的脚本放在该项目的 `scripts/`。
- 在 mise 中配置脚本需要的 Node 工具版本，把脚本注册为任务，由任务调用 `node scripts/<名称>.mts`，Agent 仍通过 `one run` 执行。
- 脚本从进程环境读取变量，不在代码、参数或日志中写入、展开或打印密钥值。

## 环境变量

- 环境变量必须通过 One CLI 或 One Dashboard 管理。不要另建 `.env` 文件作为变量来源，也不要将变量值写入源码、`one.manifest.toml` 或 `mise.toml`。
- 应用运行时变量配置到工作区或对应项目，执行项目任务时由 One 按项目注入。默认环境是 `dev`，需要其他环境时显式使用 `--env`。
- 脚本和业务代码直接读取进程环境，不自行连接 Infisical 或读取本机凭据文件。

## 全局操作凭据

- Docker 镜像上传账号、密码，以及发布或运维所需的凭据，放在 One 的全局变量中，不混入应用运行时变量。
- 缺少凭据时，说明所需的变量名称、用途、环境和目录，提示用户运行 `one serve`，在 Dashboard 的“共享凭据”页面配置。不要要求用户在聊天中发送凭据值。
- 用户配置完成后，可以通过 `one env list --global --env <环境> --path <目录>` 检查变量名称，再让任务内部使用 `one exec --global` 注入所需变量；显式指定 `--env`、`--path` 和 `--keys`。
- Agent 的执行入口仍然是 `one run`。例如，已有 `scripts/push-image.mts` 时，可以在根 `mise.toml` 定义：

```toml
[tasks.push-image]
run = "one exec --global --env dev --path /docker --keys REGISTRY_USER,REGISTRY_PASSWORD -- node scripts/push-image.mts"
```

随后使用 `one run push-image`。目录和变量名应与用户在 Dashboard 中的配置一致。

## 自动化与交付

- 需要解析 One CLI 结果时使用 `-o json`，根据 `error.code` 和 `error.context` 处理错误，不解析翻译后的消息；子进程输出按日志处理。
- 完成后说明改动内容、实际执行的验证及结果，并明确哪些验证尚未执行。

本文件由团队维护。后续 `one add` 和语言切换不会重写已有内容。
