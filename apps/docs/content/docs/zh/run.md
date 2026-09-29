---
title: one run
description: 执行原生 mise 任务，注入项目环境，并选择 stream 或 TUI 输出。
---

`one run` 只执行 mise 中实际存在的任务。One 负责项目选择、依赖准备、项目环境和输出展示；单个 mise 进程负责调度任务图并运行原生命令。

```sh
one run
one run --verbose
one run --list -p web -o json
one run dev
one run build -p web -p api
one run test -p api -- -run TestHandler
one run build --dry-run -o json
one run dev --ui tui
one run dev --ui stream
```

不带任务名时，终端列表按工作区入口、项目命名空间分组，默认只展示任务名与描述。根任务省略 `//:` 前缀，显示的名字可直接用于 `one run <任务名>`。`one run -p web` 筛选项目相关任务，`one run --verbose` 显示来源、缓存及交互设置。窄终端自动换行；`-o json` / `-o yaml` 继续返回完整任务数据。

## 任务名与参数

`one <task>` 保留为 `one run <task>` 的通用简写，没有独立内置的 dev 命令。`one run dev` 要求根 dev 任务存在；`one run dev -p web` 要求该项目的对应任务存在。缺失时直接报错，不回退到 start、start:dev、Go 目录或 manifest 命令。

内置命令优先：`one env` 管理环境，`one run env` 执行同名 mise 任务。使用 `-- --help` 将帮助参数传给子命令。`--` 后的参数只传给一个选中任务；多项目选择不接受透传参数。`:::` 保留给 mise 分隔任务。

`one exec web -- pnpm add axios` 使用项目的 mise 工具环境和 One 变量运行任意命令，不选择任务图，也不安装应用依赖。

## 原生配置

创建工作区、添加项目以及显式执行 `one init mise` 时，One 将已有包脚本和 Taskfile 任务映射到根配置：

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

执行、任务列表、Dashboard 查询和 dry-run 都不会补出缺失任务或刷新工作区配置。新增包脚本后，显式更新 mise 声明：

```sh
one init mise --dry-run -o json
one init mise
```

保留文件末尾的 `# one:managed-v1` 标记以支持增量生成。用户编辑和注释会被保留；同一生成字段的冲突修改会返回 `MISE_CONFIG_CONFLICT`。已有项目 mise 配置、别名、`.mise/tasks/` 可执行脚本及 `[task_config] includes` 继续可用。

列表和实际执行读取 mise 的有效任务目录。dry-run 使用静态解析，不启动 mise、不安装工具、不获取远端变量，也不执行缓存输入。无法静态解析的动态配置需要通过正常的 mise 检查确认。

生成的任务可以直接用 `mise run` 执行，不再调用 One 内部命令。直接运行 mise 时使用它自己的环境，不通过 One 获取 Infisical 变量。

## 项目变量

`one run` 按任务实际工作目录确定项目归属，嵌套项目采用最长路径匹配。根任务即使叫 `serve-backend`，只要声明 `dir = "services/api"`，也能取得 api 的变量。根聚合任务不会合并各子项目的变量。

配置 Infisical 后，One 每次运行只为每个项目获取一次变量快照。`--env` 选择已声明环境，默认 `dev`；远程目录从项目路径推导，并合并共享及祖先目录变量。各任务只接收自身项目的快照，并覆盖同名 shell / mise 变量。变量值保留在内存中，通过带会话认证的回环服务传递，不写入 TOML、`.env` 或 context JSON。

One 内置 mise 环境适配器，在对应配置作用域创建临时 `.one-run-*/bindings.toml`。文件只含任务引用，不含变量值，退出时清理。新工作区和 `one init mise` 会将 `.one-run-*/` 加入 Git 忽略规则。用户无需维护逐任务插件配置行。

启动前会核对命令、目录、依赖和环境绑定。如果 profile 在运行时元数据之后重新定义整个任务，可能覆盖绑定，One 会在启动前报错。此时将命令保留在基础或 local 配置中，让 profile 只调整环境设置。

## 缓存与并行

普通原生任务保留 mise 的缓存声明。已知确定性的模板构建任务会声明 sources、outputs 和环境输入，不需要 One 指纹命令：

```toml
[tasks."web:build"]
dir = "apps/web"
run = "pnpm run build"
sources = ["src/**/*", "package.json", "tsconfig.json", "../../pnpm-lock.yaml"]
outputs = ["dist"]
cache = { enabled = true, env = ["NODE_ENV"] }
```

注入远端变量的任务及其下游可执行任务会跳过产物缓存和新鲜度跳过，并显示简短提示。这样变量更新、删除或变为空字符串时，不会复用旧产物。One 不跨运行缓存密钥快照。

```sh
one run build --cache local-only
one run build --cache off --force
one run build --cache read-only
one run build --concurrency 4
```

缓存模式还包括 `read-write`、`write-only`；远端后端直接在 mise 中配置。source / output 路径相对于 `dir`，选中任务的产物位置不能重叠。透传额外命令参数会关闭产物缓存并强制执行。

默认按任务图节点数分配并行额度，不对 dev 名称特殊处理。显式并行上限会被保留，常驻前置任务仍遵循 mise 的依赖语义。

## 终端偏好

在 `~/.config/one/preferences.json` 中设置可选的 `taskUI`（遵循 `XDG_CONFIG_HOME`）：

```json
{
  "version": 1,
  "locale": "auto",
  "taskUI": "tui"
}
```

`taskUI` 接受 `stream` 或 `tui`，`--ui` 覆盖个人偏好。`auto` 在交互终端的多任务运行中使用 TUI，单任务使用 stream。CI、非终端及 JSON/YAML 输出使用 stream。`--ui raw` 保留原生终端输入，并交由 mise 处理 raw 输出；交互任务需要独占终端。

TUI 保留 mise 任务前缀、Task 命令回显、ANSI 颜色和文字样式、缩进及空行。长行按日志区域宽度自动换行，支持中文和 emoji；查看历史时，窗口缩放会尽量保留原来的阅读位置。常见进度输出中的回车、退格和清行操作在日志区域内呈现；全屏交互程序应使用 `--ui raw`。

左侧按本次执行计划展示依赖树，入口任务位于根部。共享依赖只展开一次，其他分支以 `↪` 引用，选中后查看同一份任务日志。焦点在树中时，↑/↓ 选择任务，←/→ 折叠或展开，Enter 或空格切换展开状态；在共享引用上按 Enter 或 → 定位首次出现处。终端不足 70 列时，Tab 在全宽依赖树与日志之间切换。

在日志区域中，鼠标滚轮或 ↑/↓ 逐行滚动，PgUp/PgDn 翻页，Home 查看最早日志，End 或 `f` 跟随最新输出。向上滚动会暂停跟随，新日志继续收集。Tab 在日志与依赖树之间切换键盘焦点；`[` / `]` 也可切换任务，`a` 查看全部日志。各任务分别保存阅读位置。`/` 打开历史搜索，Enter 应用，Esc 清空筛选。

本次会话的完整历史保存在私有临时文件中，内存维护精简索引和有容量限制的渲染页面缓存，较早的日志不会被丢弃。正常退出时删除临时文件；存储失败会明确报错并停止运行。mise 执行结束后，无论成功或失败，TUI 都会自动退出并返回原退出码；正在滚动或搜索时也会正常结束，无需按键。执行期间 Ctrl+C 或 `q` 停止进程树并恢复终端。退出后，普通终端中保留简短的尾部日志。

对于经过管道就默认关闭颜色的常见工具，TUI 会声明颜色支持，并保留用户显式设置的颜色偏好。依赖真实 TTY 界面的工具仍需使用 raw 模式。

任务标签只表示是否观察到输出，不通过子进程文字推断就绪、成功或缓存命中。最终结果以 mise 退出码为准；由于 mise 没有可靠的结构化生命周期事件，单任务完成状态保持 `unknown`。

One 原样转发子进程日志，不扫描或替换密钥值。One 自身的提示和结构化结果不展示注入值；子进程主动打印的值会出现在 stream、TUI、Dashboard 和原生日志缓存中。

JSON/YAML 计划使用 `one-cli/task-plan/v1`，执行结果使用 `one-cli/task-result/v1`；结构化模式下子进程日志进入 stderr。
