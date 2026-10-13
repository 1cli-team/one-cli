# 工作区开发约定

## 了解工作区

- 从当前目录向上查找最近的 `one.manifest.toml`，确认项目名称、路径和工具链。
- `apps/` 存放应用，`services/` 存放后端服务与 worker，`packages/` 存放共享库。
- 修改项目之前，阅读相关的 `README.md` 和适用的 `AGENTS.md`。
- 使用 `one --help` 和具体命令的 `--help` 确认用法，不猜测命令或参数。添加项目时先用 `one templates -o json` 查询模板，再使用 `one add`。

## 技术栈与项目结构

- `empty-app`、`empty-service` 和 `empty-library` 不预设技术栈。语言、框架、UI、数据库及开发工具由用户选择；不要从工作区其他项目或已安装的 skills 推断空项目必须使用的技术栈。
- One 约定项目位于 `apps/`、`services/` 或 `packages/`，项目内部目录按用户选择的技术栈与架构组织。用户尚未指定时，根据需求提供方案，不自动套用其他模板的依赖或目录。
- 修改已有项目时，以该项目的实际依赖、配置和团队约定为准。Skills 中的技术栈和目录示例只适用于对应项目；确定技术栈后再配置相应工具链、任务和开发 skills。

## 统一执行入口

- 测试、检查、构建、依赖准备和项目脚本必须通过 `one run <任务>` 执行；项目任务使用 `one run <任务> -p <项目>`。
- 先用 `one run` 查看已有任务，优先复用。不直接运行 `pnpm test`、`go test`、`node`、`docker` 或 `mise run` 等命令绕过任务入口；任务内部可以调用这些原生工具。
- 缺少需要的任务时，先在适用的项目任务文件（如 `package.json`、`Taskfile.yml`）或根 `mise.toml` 中定义。修改项目任务后，按需运行 `one init mise` 同步，再通过 `one run` 执行。
- One CLI 的帮助、任务查询、项目创建、skills 管理和变量管理命令可以直接使用，无需包装成任务。
- 修改完成后，通过 `one run` 执行受影响范围的检查、测试和必要构建；任务名以实际配置为准。

## 复杂脚本

- 多步骤、复杂分支或反复使用的操作，可以编写为 `scripts/<名称>.<扩展名>`，使用项目已选用的语言，避免拼接长串 shell 命令。使用 TypeScript 时可采用 `.mts`；不要仅为脚本引入另一套技术栈。
- 跨项目脚本放在工作区根目录的 `scripts/`，仅服务于一个项目的脚本放在该项目的 `scripts/`。
- 在 mise 中配置所选脚本语言需要的工具版本，把脚本注册为任务；Agent 仍通过 `one run` 执行。
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

- 使用 `.agents/skills` 中已安装的工作区 skills 了解 One 用法、项目目录结构和目标项目的技术栈。Manifest 和团队约定优先于上游示例。
- 根据 skill 的名称与描述选择相关指导，任务需要时再阅读对应 `SKILL.md`，引用资料和脚本也按需读取，不预先阅读全部已安装 skills。
- 在工作区根目录安装项目级 skills，保存到 `.agents/skills`，不使用 `--global` 安装到用户全局目录。
- `one skills` 将参数透传给 `npx skills`，其输出和帮助遵循上游工具。业务 skills 在独立 GitHub 仓库维护，仅在用户主动要求时安装。安装 skill 不代表授权实现业务功能。
- 需要解析 One 自身的结构化结果时使用 `-o json`，根据 `error.code` 和 `error.context` 处理错误，不解析翻译后的消息。`one skills`、`one mise` 和 `one hk` 保留上游输出与参数，不使用 One 的 JSON 包装；子进程输出按日志处理。
- 完成后说明改动内容、实际执行的验证及结果，并明确哪些验证尚未执行。

本文件由团队维护。后续 `one add` 和语言切换不会重写已有内容。
