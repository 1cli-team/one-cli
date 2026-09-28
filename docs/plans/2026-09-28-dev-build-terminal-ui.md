# one dev / one build 终端界面与多项目执行规划

> 有限 build 的调度及显示已由 [工作区任务统一方案](2026-09-28-workspace-task-unification.md) 更新为 mise stream/raw；开发 supervisor 与 TUI 保留。下文保留历史设计。

状态：已实施前三阶段的 Unix 版本；Windows ConPTY TUI 留待第四阶段。下文保留设计时的现状调查，实际行为以文末实施记录为准。

## 1. 目标与默认体验

- 一个实际执行任务：直接连接原生终端，保留命令自身的颜色、排版、进度刷新和输入交互。
- 多个实际执行任务：在交互式终端默认进入 TUI，按项目查看独立输出。
- dev 和 build 共用终端与进程会话能力，各自保留常驻服务、有限构建任务的执行规则。
- CI、管道、重定向和 JSON/YAML 输出使用适合自动化的输出方式。
- 同一 Workspace 内可以指定多个项目，也可以继续一次启动全部项目。

用户提到的参考界面应是 Turborepo 的任务 TUI。Turbopack 是相关生态中的打包器；本方案参考 Turborepo 的项目列表、任务状态、独立日志和输入模式。

## 2. 当前实现调查

已核对本机 `one --help`、`one dev --help`、`one build --help` 和仓库源码。

| 功能 | 当前行为 |
| --- | --- |
| `one dev` | 并行启动所有声明了 `domains.dev.command` 的项目 |
| `one dev web` | 启动一个项目；支持项目名或相对路径 |
| `one dev web api` | 不支持，Cobra 目前限制最多一个位置参数 |
| dev 输出 | stdout/stderr 按行加项目前缀；没有连接子进程 stdin |
| dev 生命周期 | 任一进程退出后停止全部进程；Unix 使用进程组，Windows 使用 Job Object |
| `one build` | 按本地 Node 依赖排序后逐个执行；首个失败后停止后续任务 |
| `one build web` | 只构建该项目，不自动加入它的本地依赖 |
| build 输出 | 接受 stdin，但 stdout/stderr 经过同一个按行前缀 writer |
| `one run` | 负责 runtime、PATH、环境变量注入，项目命令已经继承标准输入输出 |
| 现有依赖 | 已有 Lip Gloss、间接依赖 Bubble Tea v2，以及用于终端测试的 creack/pty |

关键入口：

- `packages/cli/internal/transport/cobra/dev/cmd.go`
- `packages/cli/internal/modules/development/process/ops.go`
- `packages/cli/internal/modules/development/process/supervisor*.go`
- `packages/cli/internal/transport/cobra/build/cmd.go`
- `packages/cli/internal/modules/build/plan.go`
- `packages/cli/internal/modules/build/service.go`
- `packages/cli/internal/transport/cobra/run/cmd.go`
- `packages/cli/internal/platform/process/`
- `packages/cli/internal/platform/output/mode.go`

当前颜色与动态输出失真的原因：外层 writer 让子进程看到的是管道，并按换行缓冲、添加前缀。仅删除前缀或设置 FORCE_COLOR，不能恢复终端检测、回车刷新和输入交互。

## 3. 命令设计

以下为拟新增的用法，当前版本尚未实现：

```sh
one dev web api                 # 并行启动指定项目
one dev apps/web services/api   # 同样支持相对路径
one dev --select                # 搜索并多选项目，再启动
one dev web --ui=tui            # 单项目也可以主动使用 TUI
one dev web api --ui=stream     # 带项目标识的连续日志
one build web api              # 构建指定项目
one build --concurrency=4      # 按依赖关系同时构建最多四个项目
```

保留已有 `one dev` / `one build` 不带参数的全部项目行为；不额外加入必经选择界面。单个 `-p/--project` 保持兼容，第一版用多个位置参数表达多选，避免同时扩展两套多选语法。`-p` 与多个位置参数混用应明确报错。

选择器先解析项目名和路径、去重、验证项目操作，再开始准备依赖。显式选中了没有对应命令的项目时直接报错；无参数的全部项目模式继续跳过没有对应操作的项目。名称与路径别名应复用现有解析逻辑。

界面参数：`--ui=auto|raw|tui|stream`，默认 `auto`。

| 场景 | auto 的行为 |
| --- | --- |
| 真实终端，只有一个执行任务 | raw：直接连接终端，无项目日志前缀 |
| 真实终端，有多个执行任务 | tui：列表 + 当前任务终端 |
| 管道、CI、TERM=dumb、终端能力不足 | stream / 结构化结果，不进入全屏界面 |
| `-o json` / `-o yaml` | stdout 保留结果协议，子进程日志写 stderr |
| `--dry-run` | 返回执行计划，不启动 TUI、不启动进程、不安装依赖 |

任务数量以执行计划中实际要运行的任务为准，不计跳过的项目。若将来支持自动补齐构建依赖，也按展开后的任务数判断。

显式 `--ui=tui` 遇到非交互终端或结构化输出应给出清晰错误；`auto` 才做自动降级。多任务不能使用 raw，否则多个应用会争用同一个终端。`--ui=raw` 与结构化输出冲突时明确报错。

## 4. TUI 布局与交互

```text
One dev · workspace                              2 running · 1 failed
┌───────────────────────┬───────────────────────────────────────────┐
│ Projects              │ web · running · 00:24                    │
│ > ● web       running │                                           │
│   ● api       running │   VITE                                    │
│   × worker    failed  │   Local: http://localhost:5173/           │
│                       │                                           │
│                       │   当前项目的终端输出                      │
└───────────────────────┴───────────────────────────────────────────┘
↑↓ select · Enter interact · r restart · / search · ? help · Ctrl+C stop
```

- 左侧：项目名、运行状态、退出码或耗时。区分 starting、running、stopping、stopped、failed；build 另有 pending、succeeded、blocked、skipped。
- 右侧：只展示选中项目，不加逐行项目前缀。颜色、换行、回车覆盖、清屏与进度刷新由该项目独立的终端状态管理。
- 底部：显示当前可用快捷键、是否正在跟随最新输出、是否进入输入模式。
- 日志可滚动；查看历史时不被新日志强制拉回底部；提供恢复跟随按钮或快捷键。
- 窄终端可隐藏项目列表；极小尺寸显示简化界面，仍保留退出能力。
- 运行状态只说明进程存活，不在没有健康检测依据时标记 ready。
- One 的依赖安装、runtime 准备与需要用户回答的提示在进入 TUI 前完成，避免多个准备进程抢占 stdin。

输入模式必须与导航模式明确区分：

- 导航模式：方向键切换项目，`r` 重启当前 dev 项目，`s` 停止当前 dev 项目；Ctrl+C 停止会话内全部进程。
- Enter：把键盘输入交给当前项目；普通快捷键随之交给子进程，例如 Vite 的 h/r。
- Ctrl+]：退出输入模式，回到项目导航；底部持续显示提示。
- 输入模式里的 Ctrl+C 交给当前项目；全局停止需先退出输入模式。
- 搜索模式单独处理 Esc，不将搜索文字转发给子进程。

## 5. 保留原始输出的实现

### 单任务 raw

将子进程的 stdin、stdout、stderr 连接至真实终端，继续通过 `one run` 执行项目命令，复用环境变量和 runtime 逻辑。原生输出的保真度最高；One 不给业务日志添加前缀，也不按行重写日志。

还要正确处理前台进程组、信号、退出码和终端状态恢复。不能只在现有 Setpgid 逻辑中加入 stdin，否则后台进程组读取控制终端可能被暂停。复核整个 `one → one run → mise → __exec → 应用` 链路，避免重复发送中断信号。

### 多任务 TUI

每个任务拥有一个独立伪终端（PTY），应用据此判断自己运行在终端中。输出按字节增量流入终端模拟器，由模拟器维护屏幕和滚动历史，再由 TUI 绘制当前任务。

- Unix：评估复用已有 creack/pty。
- TUI 外壳：复用 Bubble Tea v2 和 Lip Gloss。
- ANSI/VT 终端模拟器：先验证现有 Go 实现对需要的控制序列的兼容性，再确定依赖；不把普通文本 viewport 当作终端模拟器。
- resize：向所有任务同步当前终端窗格大小，包含暂时没有选中的任务。
- 日志高流量时限制渲染频率，保持读取持续进行；每个任务的滚动历史设上限并提示截断，避免长时间开发无限占内存。
- PTY 输出通常将 stdout/stderr 合并，这是终端模式的明确边界；stream 模式继续区分来源。
- ANSI 解析不等同于完整终端兼容。验收覆盖目标工具的颜色、回车、清屏、Unicode 和输入；不承诺任意嵌套全屏程序都完全一致。
- `NO_COLOR` 等用户设置继续有效，不能无条件覆盖为 FORCE_COLOR。

Windows 需要 ConPTY 适配，不能直接使用仅支持 Unix 的 creack/pty。第一阶段 raw 和 stream 在 Windows 保持可用；TUI 若暂不支持，则 auto 明确降级，正式宣称跨平台 TUI 前需完成 Windows 验收。

## 6. 进程规则与构建调度

### dev

首版保留当前“任一进程退出则停止整组”的默认策略，UI 展示各项目最后状态和失败输出，行为不因使用 TUI 或 stream 而改变。

第二阶段增加 `--keep-going`：一个项目退出后，其他项目继续运行；TUI 保留失败项，支持单独重启。这个模式更适合长期联调，后续是否成为默认值应作为独立行为变更记录。所有项目结束时退出；用户退出后不留下后台常驻服务。

单独停止/重启是用户操作，不应触发“意外退出停止整组”。每次重启前必须确认旧进程树已退出，避免端口被旧进程占用。重启输出应标注运行批次。

### build

先复用 TUI 和 raw 输出，保留串行执行、首个失败后不再启动后续任务。

之后加入 `--concurrency=N`，初始默认值保持 1：

- 先根据本地 Node 依赖构建有向无环图；只有前置构建成功才允许启动依赖它的项目。
- 多项目显式选择同样需要处理所选项目之间的依赖，目前此逻辑只在完整工作区构建时启用。
- 继续保持显式项目选择的范围；不悄悄将单项目 build 改成包含所有依赖的构建。将来可另加 `--with-deps` 显式展开依赖闭包。
- 未选中的依赖假定已准备好，并在执行计划中说明；有依赖但缺少 build 操作的源码包不能被视为构建失败。
- Node 以外的跨项目构建依赖不能凭项目排列猜测；首次沿用现有支持范围，额外依赖关系单独设计。
- 出现失败后停止启动新任务，允许已经运行的无关任务结束；依赖失败项的任务标记 blocked，其余未启动任务标记 not_run。
- 用户 Ctrl+C 立即进入整组取消与进程树清理。
- 全部完成后自动退出 TUI，在普通终端保留结果摘要与失败任务日志尾部；构建成功和失败都不能等待用户按键才返回 shell。
- 单任务返回原退出码；多任务采用稳定的任务顺序汇总首个实际失败码；用户中断保持 130/143。

## 7. 代码组织

建议按现有分层实现以下能力，具体包名在实施时依据架构测试确定：

1. 项目选择与计划：复用 application/execution 的名称、路径解析；dev/build 分别形成任务计划。
2. 共享进程会话：提供 Start、WriteInput、Resize、Stop、Wait 与状态/输出事件，不依赖 TUI。
3. 平台适配：Unix 进程组 + PTY；Windows Job Object + ConPTY。每个项目可独立清理整个进程树。
4. 输出适配：raw、stream、tui 共享执行结果与错误码，避免复制环境注入逻辑。
5. Cobra 层：解析多项目、ui、select、concurrency、keep-going，并维持帮助、国际化和 dry-run 协议。

`one run` 保持项目命令的执行边界；本轮不在 dev/build 内复制密钥加载或 mise 启动逻辑。结构化结果继续通过现有 output 包发出，新字段和状态需同步协议快照。

## 8. 实施顺序

### 第一阶段：单项目原生终端与多项目选择

- 为 dev/build 增加多个位置参数，复用项目集合验证。
- 单任务默认 raw，多任务先沿用 stream。
- 完成真实 TTY 输入、颜色、信号、进程树退出、JSON/YAML 分流和 dry-run 回归。
- 这一阶段即可交付“一个项目保留原输出”和“指定多个项目一起启动”。

### 第二阶段：多项目 dev TUI

- 先用实际 Vite、Next、Go 服务和模拟动态终端输出验证 PTY + 终端模拟器。
- 实现项目列表、独立终端、输入模式、滚动、resize、停止与重启。
- 增加 --select、--keep-going；Unix 完整交付，Windows 明确能力边界。

### 第三阶段：build TUI 与依赖并行

- 接入 build 的任务状态、耗时、完成摘要和失败输出。
- 保持 concurrency=1 默认，开放按依赖图的有限并行。
- 验证失败依赖不会继续构建、被取消任务不会留下进程。

### 第四阶段：Windows TUI 与体验补齐

- 完成 ConPTY、输入、resize、Job Object 清理和 Windows Terminal 验收。
- 再考虑保存常用项目组合；跨 Workspace 编排另行规划。

## 9. 验收标准

- 单项目 dev/build 在伪终端测试中能检测到真实 TTY，保留 ANSI、回车覆盖和无需换行的输出，键盘能到达应用。
- 多项目选择支持名称、路径、去重和缺少命令报错；准备共享依赖不重复执行。
- TUI 切换项目时屏幕互不污染，重绘与窗口变化不会丢失退出状态；中文、emoji、长行与高频输出可用。
- 开启输入模式后应用快捷键有效；退出输入模式和停止全部会话可预测。
- 停止或重启 Node/pnpm/mise 包装的进程后没有遗留子进程，端口被释放；正常退出、失败、启动失败和强制取消都恢复终端。
- CI/管道不出现全屏控制序列；JSON/YAML stdout 可解析；--dry-run 不安装、不加载密钥、不启动进程。
- 构建并发不超过限制，依赖顺序正确，循环依赖在执行前被拒绝，失败和中断退出码稳定。
- 同步中英文帮助、文档、参考快照；通过相关单元测试、PTY E2E 和仓库 task check。

## 10. 参考

- [Turborepo 开发任务与终端 UI](https://turborepo.dev/docs/crafting-your-repository/developing-applications)
- [Turborepo 配置：ui、persistent、interactive](https://github.com/vercel/turborepo/blob/main/apps/docs/content/docs/reference/configuration.mdx)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [creack/pty](https://github.com/creack/pty)
- [待评估的 Go VT 实现](https://github.com/charmbracelet/x/tree/main/vt)


## 实施记录

- 已实现多项目名称/路径选择与去重、--select、--ui、--keep-going 和 build --concurrency。
- dev/build 共用 platform/taskrun 的调度、状态和进程控制；旧执行器的进程树回归测试已迁移。
- 单任务原始输出在 Unix 通过透明 PTY 转发实现，而非简单继承文件描述符。这使原始输入/输出与进程组清理同时成立，并保留程序的 TTY 检测、ANSI、回车刷新和交互。
- 多任务通过 Bubble Tea、Lip Gloss、charmbracelet/x/vt 和独立 PTY 实现项目切换、输入、窗口尺寸同步、滚动、搜索、停止和重启；历史上限为每任务 3000 行。
- build 按所选项目本地依赖调度，默认并发 1，失败依赖标记 blocked；构建结束自动退出并保留失败输出。
- 管道/JSON/YAML 的 stdout 保持可解析，dev 日志改到 stderr；这修复了原有 dev JSON 混入日志的问题。
- Windows 保留原生与 stream 输出，每个任务使用独立 Job Object 清理；未实现 ConPTY，不宣称 Windows TUI 已可用。

### 本轮验证

- `task check` 通过：文档/帮助/架构约束、Go vet/gofmt、全量 Go 与端到端测试、Dashboard 检查和 60 项前端测试。
- taskrun、build、execution 和 development/process 的 race 检测通过。
- 真实 PTY 端到端验证：单任务 dev/build 的 TTY 检测、ANSI 原样输出与输入；多项目 dev 的输入、尺寸变化、单独重启和 Ctrl+C 清理；build TUI 失败摘要与自动退出。
- Windows amd64 与 macOS arm64 交叉编译通过；这不代表 Windows/macOS 真机交互已验证。
- 新增 VT 依赖已锁定版本；单项目 raw 和多项目 TUI 在当前 Linux 环境验证。
