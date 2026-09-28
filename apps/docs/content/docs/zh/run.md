---
title: one run
description: 使用 mise 发现、调度和缓存工作区任务。
---

One 负责项目选择和项目环境，mise 负责工作区任务依赖与执行。Node 项目在 `package.json` scripts 中定义命令，由 pnpm 执行；Go 项目在 Taskfile 中定义命令，由 Task 执行。

```sh
one run                          # 列出任务，不执行命令
one run --list -p web -o json
one run build                    # 构建工作区及本地依赖
one run build -p web -p api
one run test -p api -- -run TestHandler
one run check --env prod
one run ci
one run build --dry-run -o json
```

## 任务简写

`one <任务名>` 是 `one run <任务名>` 的简写。`dev`、`build`、`test`、`lint` 都是普通任务名，统一遵循这条规则。例如 `one test -p api` 与 `one run test -p api` 执行同一个任务。内置命令及其别名优先：`one env` 打开环境变量管理，`one run env` 执行名为 env 的任务。未知名称按任务解析，找不到时提示任务不存在。`one <任务名> --help` 显示 One 通用任务参数；用 `-- --help` 把帮助参数传给底层命令。

## 项目选择与参数

不传 `-p` 时选择工作区聚合任务。可以重复传入 `-p`，指定项目名或工作区相对路径。`--` 后的参数原样传给一个选中的任务；同时选择多个项目时不接受这类参数。`one build -p web` 和 `one run build -p web` 共用执行链路。

任意命令使用 `one exec web -- pnpm add axios`。它不运行任务图，也不自动安装应用依赖。

`:::` 是 mise 的任务分隔符，不能作为任务参数透传。

## 配置与依赖

One 在工作区和项目根目录的 `mise.toml` 中声明工具版本、项目命令入口、聚合任务和本地 Node 构建依赖。直接编辑这个文件即可添加任务或调整设置。执行任务、添加项目或运行 `one init mise` 时，One 会补充新配置，并更新未被用户修改的生成项；自定义字段和注释保留。根 `dev`、`build`、`check`、`test` 只聚合项目实际存在的任务，`ci` 组合 build、check、test 聚合任务。

文件末尾的 `# one:managed-v1` 注释记录 One 上一次生成的字段。请保留它，以便后续增量更新。如果同一个字段被用户和 One 同时改成不同内容，会报告 `MISE_CONFIG_CONFLICT` 并指出冲突项。仅修改注释、添加自定义任务或调整默认值，不会因为整份文件改变而被拒绝。

```text
workspace/
  one.manifest.json
  mise.toml             # 工具版本、项目目录和聚合任务
  .config/hk.pkl        # 工作区 Git 检查
  apps/web/
    mise.toml           # 项目任务与缓存配置
    package.json        # pnpm 脚本
  services/api/
    mise.toml           # 项目工具与任务
    Taskfile.yml        # Go 项目命令
```

预览和刷新配置：

```bash
one init mise --dry-run -o json
one init mise
```

项目任务的规范名称为 `//apps/web:build`。本地 Node 上游存在 build 时，会在下游 build、check、test、typecheck 和 dev 前执行。Go 的模块依赖由 `go.work` 和编译器解析。One 管理的 Node 项目统一使用 pnpm。

列表和 dry-run 静态读取配置，不启动 mise、不安装工具、不读取密钥、不写文件。动态表达式和带参数的依赖无法静态预览。实际运行还会读取 mise 的有效任务清单。用户直接定义命令的原生 mise 任务使用 mise 环境；One 生成的项目入口使用 One 项目环境。

## 定义自己的任务

项目命令写入 `package.json` scripts 或 Taskfile，例如新增 `"docs:build": "typedoc"` 后，使用 `one run docs:build -p lib`。工作区级任务写入根 `mise.toml`：

```toml
[tasks.verify]
depends = ["//apps/web:check", "//services/api:test"]

[tasks.hello]
description = "Print a greeting"
run = "echo hello"
```

使用 `one run verify` 执行聚合，或 `one run hello` 执行自定义命令。也可以把可执行脚本放到 `.mise/tasks/`，或用 `[task_config] includes = ["tasks.toml"]` 加载独立任务文件。

诊断原生配置可运行 `one mise tasks ls --all --local` 和 `one mise tasks info //apps/web:build --json`。用户自行定义的原生任务可以直接用 `mise run hello`；One 生成的项目任务需要 `one run` 准备环境上下文。

## 缓存

生成配置开启 mise 实验性功能。已知模板的 **build** 任务声明输入、输出和环境指纹；其他任务需要显式声明缓存规则。修改模板构建脚本或构建配置后，自动缓存声明会关闭；在项目 `mise.toml` 中声明实际输入和输出即可重新启用。缓存存储和产物恢复交给 mise。

```sh
one run build --cache local-only
one run build --cache off --force
one run build --cache read-only
one run build --concurrency 4
```

`--cache off --force` 同时关闭产物缓存和输入新鲜度跳过，确保实际执行。还支持 `read-write` 和 `write-only`；使用远程缓存前需自行配置 mise 缓存后端，One 不创建远程服务。

在项目的 `mise.toml` 为生成任务开启缓存时，需要完整输入、输出和 One 环境指纹命令：

```toml
[tasks.build]
sources = ["src/**/*", "package.json", "tsconfig.json", "../../pnpm-lock.yaml"]
outputs = ["dist"]
cache = { enabled = true, env = ["NODE_ENV"], command_inputs = ['one __task-input --project "web" --task "build"'] }
```

只验证输入且结果确定的任务可以声明 `outputs = []`。自定义任务应包含所有上游源码、外部文件、配置值和编译器输入。输出必须位于任务目录内，且不能与同次执行其他任务的输出重叠。新配置先用 `--cache off --force` 验证。

每次运行中，One 在缓存查找前读取一次每个托管项目的环境，保存为临时上下文。指纹计算和执行使用同一份快照。环境值不会写入生成 TOML 或结构化结果。所有注入项目的变量都参与指纹；仅用于读取变量的账号凭据不参与。生成入口必须通过 `one run` 获得上下文，不能直接用 `mise run` 绕过。

## 终端与结构化输出

任务使用 mise 流式输出，`--ui raw` 保留原生终端输入并关闭产物缓存。多开发服务使用带前缀的日志并自动分配并发数；raw/interactive 服务需要单独运行。`--ui tui` 不再支持。`one dev` 与 `one run dev` 共用执行路径，包括上游构建。开发行为见 [one dev](/zh/docs/dev/)。

JSON/YAML 预览的 schema 是 `one-cli/task-plan/v1`，执行结果是 `one-cli/task-result/v1`。结构化模式下子进程日志写到 stderr。结果包含整体状态与退出码。当前 mise 版本没有可靠的结构化完成事件，因此单任务状态为 `unknown`，不会从日志文字猜测缓存命中。

## GitHub Actions

工作流继续由仓库维护，并调用与本地相同的任务。安装 One 后可使用：

```yaml
- uses: actions/checkout@v7
- uses: jdx/mise-action@v4
  with:
    version: 2026.9.7
    experimental: true
- uses: actions/cache@v4
  with:
    path: .cache/mise-task-artifacts
    key: tasks-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('**/pnpm-lock.yaml', '**/go.sum', '**/mise.toml') }}-${{ github.sha }}
    restore-keys: tasks-${{ runner.os }}-${{ runner.arch }}-
- run: one run ci --ui stream
  env:
    MISE_TASK_CACHE_DIR: ${{ github.workspace }}/.cache/mise-task-artifacts
```

缓存目录应排除在任务输入和版本控制之外。外层缓存 key 隔离运行平台，mise 验证每个产物的输入 key。读取密钥的工作流沿用仓库的权限和分支信任规则。

原生选项见 [mise 任务缓存](https://mise.jdx.dev/tasks/caching.html)和[任务配置](https://mise.jdx.dev/tasks/task-configuration.html)。
