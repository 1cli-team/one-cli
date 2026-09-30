---
title: one run
description: 使用官方 Process Compose 和 mise 工具环境执行工作区任务。
---

`one run` 从 `mise.toml` 读取支持的静态任务，准备依赖和项目变量，再交给官方 **Process Compose 1.122.0** 调度。mise 继续负责安装工具和准备版本环境。One 不修改 mise，也不另写一套任务调度器。

```sh
one run
one run --verbose
one run --list -p web -o json
one run dev --ui tui
one run build -p web --ui stream
one run test -p api -- -run TestHandler
one run build --dry-run -o json
```

`one <任务名>` 是 `one run <任务名>` 的简写，内置命令优先。终端列表省略根任务的 `//:`，项目任务使用 `web:build` 等名称。缺少任务时报错；执行与查询不生成缺失任务。调整 package scripts 或 Taskfile 后，显式运行 `one init mise` 同步定义。

## 工具安装

One 复用版本恰好为 1.122.0 的 Process Compose 程序。没有匹配版本时，调用 mise 安装 `process-compose@1.122.0`。首次运行需要网络，后续可复用已安装版本。使用官方程序，无需补丁或 Rust 构建。

```sh
mise use process-compose@1.122.0
```

工作区可以在 `[tools]` 中声明 `process-compose = "1.122.0"`。One 为每次调用生成临时配置，用户无需维护 `process-compose.yaml`。原生 HTTP API 关闭，不占用 8080 端口。

## 支持的任务配置

创建项目和 `one init mise` 仍将 package scripts 与 Taskfile 映射到根任务：

```toml
[tasks."web:dev"]
dir = "apps/web"
run = "pnpm run dev"

[tasks."api:dev"]
dir = "services/api"
run = "task dev --"

[tasks.dev]
depends = ["web:dev", "api:dev"]
```

首版支持以下静态子集：

| 配置 | 行为 |
| --- | --- |
| 字符串或字符串数组 `run` | 按顺序执行 shell 命令；任一步失败即结束任务。 |
| `depends` | 选择依赖，成功完成后才启动当前任务；共享依赖只运行一次。 |
| `wait_for` | 仅等待本次已选中的匹配任务成功，不额外选择任务。 |
| `dir`、别名、项目作用域 | 执行前解析工作目录和规范任务名称。 |
| 可执行文件任务、本地 TOML includes | 查询只读取定义，不执行脚本。 |
| 普通任务 `env`、取消变量、`tools`、`shell` | 图启动前准备各任务工具和环境。 |
| `sources`、`outputs`、`--force` | 时间戳新鲜度判断，支持递归 `**` glob；不恢复产物缓存。 |
| `raw`、`interactive`、`--ui raw` | 仅包含一个可执行任务的图可以独占终端。 |

任务模板、带参数依赖、`depends_post`、任务环境插件、usage 参数规范和可执行缓存输入暂不支持。One 在**启动任务命令前**指出所选定义与不支持的字段，不回退到 mise 任务调度。未选中的任务可以保留这些字段。这是明确的兼容子集，不是完整 mise 任务兼容实现。

任务命令可以调用 pnpm、Go、Task 等叶子工具。如果已有脚本本身调用 `mise run`，它仍会显式使用嵌套调度器；完整迁移工作区时需要调整这些脚本调用。

本地配置和 profile 通过静态方式读取，查询、正常规划与 dry-run 使用同一任务图。dry-run 不安装工具、读取远端变量、计算模板或执行缓存输入。

保留 `# one:managed-v1` 增量生成标记；生成字段冲突仍报 `MISE_CONFIG_CONFLICT`。仓库首次安装、CI 和 Git hooks 可以独立继续使用原版 `mise run`。

## 参数与 raw 模式

`--` 后的参数只传给一个入口任务的最后一条命令，或直接传给文件任务，不传给依赖。多个入口同时传参时报错。`:::` 继续保留。

POSIX shell 参数保留空格、引号、中文和元字符。在 Windows 上，文件任务支持参数；带参数的内联命令需要显式 PowerShell shell。cmd.exe 参数转发在执行前拒绝，避免静默展开参数内容。

`--ui raw` 使用 Process Compose 前台执行，保留一个可执行任务的 stdin/stdout/stderr。包含多个可执行任务的图使用 stream 或 TUI。`one exec web -- pnpm add axios` 仍用于带项目工具环境和 One 变量的任意命令。

## 项目变量与结果

任务归属按实际工作目录匹配最深的已注册项目。根聚合任务不合并各子项目变量。

项目启用环境变量时，每次运行只向 Infisical 发起一次递归读取，在内存中生成各项目的不可变变量快照；再次运行重新读取最新值。`--env` 选择已声明环境，默认 `dev`。每个项目依次继承根目录、父目录和自身目录的变量，较近目录覆盖同名键，其他项目和更深层子目录的变量不会混入。变量覆盖同名 shell、mise 和任务变量；每个叶子只接收自己的项目快照，包含空值。后续子进程继承相同环境。

Infisical 递归读取最多支持 20 层目录。超过此深度的项目会在请求前报错；响应中的变量缺少绝对目录路径时也会报错，避免分发不完整或归属不明的变量。

变量和命令通过本地认证通道交给私有叶子进程。生成的 YAML 只含任务图元数据和 worker 标识，不含注入变量值；不生成环境插件或临时绑定 TOML。运行结束后关闭通道，清理私有配置目录。

启动进度写入 stderr，取消会中断准备与执行。保留失败命令的退出码；SIGINT 和 SIGTERM 分别返回 130、143。任务结果使用 `succeeded`、`failed`、`cancelled`、`cached` 或 `skipped`，未启动的后续任务标记跳过。子进程自行打印变量仍会出现在日志中，One 不过滤子进程输出。

JSON/YAML 计划使用 `one-cli/task-plan/v1`，其中 `runtime: process-compose`；结果使用 `one-cli/task-result/v1`。结构化模式的子进程日志写入 stderr。

## 新鲜度与并发

不使用产物缓存，没有 `--cache` 参数。已有 `cache.enabled` 元数据不会启用产物存储。普通 `sources`、`outputs` 仅在所有输出存在，且不早于输入和任务定义时跳过任务。输入、输出缺失或不可读时执行。路径相对 `dir`，所选任务的输出位置不可重叠。

```toml
[tasks."web:build"]
dir = "apps/web"
run = "pnpm run build"
sources = ["src/**/*", "package.json", "../../pnpm-lock.yaml"]
outputs = ["dist"]
```

`--force`、额外参数和 raw 模式绕过新鲜度检查。注入远端变量的任务及其下游也绕过，确保变量轮换、删除或空值不复用旧输出。

Process Compose 在依赖允许时启动任务，没有全局并发槽上限。兼容参数 `--concurrency` 会拒绝小于所选命令任务数的上限；正常调度省略此参数即可。

## One TUI 与复制

`--ui tui` 使用 One 界面，官方 Process Compose 在后台调度。左上按实际启动顺序显示已经开始且尚未结束的任务，无输出任务也会显示；等待启动的任务和纯聚合节点不冒充运行进程。左下保留完整依赖树，包括运行中的任务和已结束的结果。共享依赖用 ↪，等待关系用 ◇。无需声明 service，也无需修改上游工具。

Tab 切换侧栏和日志焦点。侧栏方向键选择、折叠/展开，并将上方任务定位到树中；Enter 展开分支或跳转共享依赖。深层分支默认折叠，上下区域独立滚动，高度保持稳定。选中的运行任务结束后，选择定位到树中同一节点，继续保留日志、搜索和阅读位置。小于 70 列时，Tab 在全宽日志和侧栏之间切换。

日志按区域宽度自动换行。`/` 搜索，Home/End 浏览历史或跟随尾部，`f` 恢复跟随。标题显示选中任务的完整名称；按 `y` 再按 `n` 也可以复制完整名称。

任务日志前缀采用紧凑的 `[任务名] 正文` 格式，不补齐任务名宽度；正文自身的缩进保留。前缀自动从 12 色调色板分配不同颜色，在同一任务图中保持一致，包括 stdout 和 stderr；任务更多时复用颜色。日志正文保留自己的颜色，复制文本去掉 ANSI 样式。环境中显式关闭颜色时，前缀也不着色。

在日志区域拖选后按 **y** 复制。选择时固定日志画面，后台任务和日志收集继续运行。没有选区时，**y** 打开菜单：**v** 复制可见日志，**n** 复制完整任务名，**l** 复制当前任务日志历史（搜索时只复制匹配记录）。复制去掉 ANSI 和界面装饰，保留 Unicode 和真实换行，软换行不插入额外换行。切换任务清除选区，同一任务的上下入口和共享引用保留选区；窗口缩放重排固定文本。

按 **c** 显示固定的全宽日志快照并关闭鼠标报告。在 VS Code 中可直接拖选，再使用终端的复制快捷键（macOS 为 Cmd+C）。**Esc** 返回并恢复原阅读/跟随状态，**f** 返回并跟随新日志。Ctrl+C 始终取消任务；搜索输入时 y/c 作为普通文字。macOS 本地使用 pbcopy，Windows 使用 Set-Clipboard，Linux 在可用时使用 wl-copy/xclip。远程会话使用 OSC 52，只报告“已发送请求”，因为终端支持情况没有可靠回执。过大的 OSC 请求明确失败，不静默截断；复制失败保留选区，任务继续运行。

One 使用私有临时日志文件保存**本次运行完整历史**，结束后删除；内存仅保留索引和有上限的渲染缓存。全部任务结束后，复制或搜索状态也会自动退出；取消和失败同样恢复终端。

`auto` 在交互终端的多任务运行中使用 TUI，其余使用 stream；CI 与结构化输出使用 stream。`--ui` 覆盖 `~/.config/one/preferences.json` 中可选的 `taskUI: "tui"` 或 `"stream"` 偏好。
