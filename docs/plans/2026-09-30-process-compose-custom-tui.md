# Process Compose Custom TUI Implementation Plan

**Goal:** 恢复此前批准的运行中列表、完整任务树和 VS Code 终端复制体验，使用官方 Process Compose 执行任务。

**Architecture:** Process Compose 1.122.0 以 headless 模式调度；One 的 Bubble Tea 界面负责展示。复用已有、按调用隔离且经过认证的 worker 状态通道，实际命令成功启动后才标记 running，整个任务结束后移除上方入口。原版 mise 继续提供工具环境，不修改两个上游项目。

**Tech Stack:** Go、Bubble Tea v2、Lip Gloss、现有私有磁盘日志 journal、zh-CN/en-US 字典。

---

用户已批准产品规则及实施。本计划替代旧计划的 mise 补丁和原生 Process Compose 界面部分，保留静态任务兼容范围和调度语义。保留已暂存工作；完成实现和验证，不自动提交。

## 1. 状态和执行入口

文件：tasks/broker.go、service.go、taskui.go；taskui/state.go。

- 为 broker 增加不可变、仅含任务身份/状态/时间的快照；不传递环境或业务日志。开始时间在多步骤任务内保持不变。
- TUI 每 100ms 读取快照；无输出任务也能显示。未收到事件的节点保持等待，不通过输出猜测。
- Process Compose 使用 --tui=false --no-server；One 接管输入和呈现，退出与取消仍走现有进程清理。stream/raw 保持既有行为。
- 将颜色能力注入实际业务子进程，同时尊重显式关闭颜色的环境。
- 依赖树包含 depends 与本次计划已有的 wait_for 节点；等待边用独立符号标识，不改变调度或增加任务。

## 2. 两区侧栏

文件：taskui/sidebar.go、tree.go、ui.go；sidebar_test.go、tree_test.go。

```text
全部日志                  │ 当前任务完整名称与日志
运行中任务 · 3            │
  ● server-run            │
  ● desktop-renderer      │
  ● desktop-host          │
任务依赖树                │
▾ dev                     │
├─ ▸ ● server-run         │
└─ ▸ ● desktop-host       │
```

- 两区各自滚动，上方约三分之一高度，成员变化不挤动下方树。空列表显示空状态。
- 上方按启动时间稳定排序；下方始终包含全部节点和运行中节点。共享依赖用 ↪ 跳转。
- 默认只展开入口和下一层；用户自行展开的分支不会被状态更新折叠。
- Tab 切换侧栏/日志；侧栏上下跨区选择。上方 → 定位树中主节点，树中箭头/Enter 保留折叠和引用跳转。
- 上方选中任务结束后，定位下方同一任务，日志、搜索、选区、阅读位置和日志焦点保留。
- 狭窄终端在全宽侧栏/日志间切换；选中项完整名称在标题换行，状态符号附有本地化说明。

## 3. 文本复制

文件：taskui/copy.go、clipboard.go、ui.go；copy_test.go。

- 日志左键拖动建立字素边界选区，冻结当前日志视图；业务继续执行，日志仍写 journal。
- y 复制选区；没有选区时打开菜单，v 复制可见范围、n 复制完整任务名、l 复制当前任务日志历史（无过滤时完整日志，搜索时只复制匹配记录）。
- 复制剥离 ANSI 和界面装饰，保留日志前缀、真实换行、缩进和空行；软换行不会插入额外换行，长行不截断。
- c 进入全宽纯日志快照，关闭鼠标报告，VS Code 可直接拖选并 Cmd+C。Esc 返回原焦点/阅读状态，f 恢复跟随；Ctrl+C 始终取消任务。
- 本地使用系统剪贴板，异步写入并按实际退出结果报告成功/失败。SSH 等远程环境使用 OSC 52，仅提示已发送请求；不读取用户剪贴板、不静默截断。
- 选区绑定当前任务和冻结文本；同任务上下区切换保持，切换任务清除。窗口缩放重排同一文本，进度行刷新不能改写选中内容。

## 4. 文案和文档

同步两个 locale、CLI run 帮助快照、README、CONTRIBUTING、中英文 run 文档、CHANGELOG。准确说明官方调度/One 界面边界、键位和复制范围。

## 5. 验收

1. 单元回归：无输出 running、快任务结果、多步骤开始时间、失败/取消、完成后稳定选择、共享依赖、上下区滚动、wait_for、窄屏与两种语言。
2. 复制回归：中英文/emoji/组合字符、ANSI、软换行、反向拖选、进度行冻结、resize、搜索、异步后端失败；使用假剪贴板避免修改用户剪贴板。
3. 真实 Process Compose PTY：两区、无输出任务、全历史滚动、自动结束、取消树清理、退出码、固定画面期间任务继续运行、鼠标报告恢复。
4. 通过 one run cli:fmt、针对性 cli:test、帮助快照、完整 check、相关 race 和多平台编译。代码状态相同且已通过的检查不重复执行。
5. VS Code 图形验收与 PTY 验收分开记录；若 UI 自动化不可用，明确保留人工验证，不把 OSC 52 输出视为成功粘贴。

## 6. 实施与验证记录

状态：实现和自动化验收已完成。官方 Process Compose 保持原版，One 的 taskui 负责界面；本次没有修改或暂存 Git index。

- broker 快照驱动两区列表；无输出任务、聚合节点、结束状态、共享引用和 wait_for 展示已覆盖。TaskUI 不参与调度。
- 日志选区使用最后交给渲染器的不可变视口，避免新日志抵达但尚未显示时选错内容；复制帧保留有上限的换行缓存。Task 切换、同身份引用、resize、搜索及进度行覆盖均有回归。
- 本地剪贴板端到端测试使用 PATH 中的假 pbcopy，并检查中文/emoji/组合字符和 ANSI 去除结果；没有读取或改写开发者真实剪贴板。
- 真实官方 Process Compose PTY 验证两区、无输出任务、选中任务完成、共享依赖、12,000 行完整历史、缩放、复制期间继续执行、鼠标恢复、自动退出、业务失败码和取消清理。
- `one run check --ui stream` 通过，含 CLI 全量测试、静态/文档检查和 Dashboard 101 项测试；后续上一帧选择修复重跑相关单元和 PTY 测试，最终代码通过完整 Go race。
- race 首轮暴露既有 Infisical 错误分类缺陷：错误文本中 URL 端口或 request ID 的 401 被识别为登录失效。已用确定性回归复现并改为 SDK 结构化 HTTP 状态；相关并发/错误回归连续五次通过，随后完整 `one run test:go --ui stream` 通过，CLI E2E 为 138.529s。
- 最终 `one run cli:check --ui stream` 和 `one run build-all --ui stream` 通过，五个构建目标为 darwin/amd64、darwin/arm64、linux/amd64、linux/arm64、windows/amd64。其他平台仅验证编译，本机实际执行为 macOS arm64。
- `one` 本机启动入口已经指向本仓库 packages/cli/bin/one，后续启动即可使用新界面；已有运行进程继续使用启动时的版本。

验证边界：VS Code 界面控制在创建独立验收终端时返回“用户切换了应用”，未继续操作用户正在使用的终端。实际 VS Code 鼠标拖选、Cmd+C/y 后粘贴，以及 SSH/tmux 的真实终端剪贴板尚未验收；PTY 或 OSC 52 输出不能替代这些结果。

## 7. 任务前缀颜色

用户要求右侧不同任务名前缀使用不同颜色。自动分配比手写配置更适合已有任务图；仅对调度器前缀着色，避免覆盖业务日志的错误、高亮等样式。

- 在日志收集前，根据排序后的完整任务身份和固定调色板分配颜色；先消解冲突，12 色用完后均衡复用。相同任务图不受日志到达或任务发现顺序影响。
- 解析到完整任务身份后，同一任务的短名、完整名、stdout/stderr、未完成行和历史记录共用颜色。不识别的前缀保持原样。
- 前缀样式在正文前重置，正文继续保持每个输出流原有的 ANSI 状态；复制依然输出纯文本。显式颜色关闭设置停用前缀装饰。
- 不修改 Process Compose，不增加用户配置。相关回归覆盖身份、跨流/跨任务样式、中文、复制、换行与历史；通过 One 入口运行受影响检查和构建。

验收：任务前缀颜色回归通过 race；相关日志/复制回归和真实 Process Compose PTY 回归通过，PTY 用时 13.243s。`one run cli:check --ui stream`、`one run cli:verify-docs --ui stream` 和 `git diff --check` 通过，cli:test:race 的构建依赖已更新本机 One。此前通过的全量 Go race 与五平台编译未为这次展示修改重复执行；实际 VS Code 颜色观感留待下次启动查看。

用户随后选择取消前缀宽度补齐，采用 `[任务名] 正文`。识别到调度器前缀后移除方括号内用于补齐的空格/Tab，保留原显示名、颜色及一个调度器分隔空格；正文缩进与 Tab 仍按原规则处理。TUI、复制和结束后打印的日志摘要使用同一份紧凑记录；stream/raw 延续官方工具输出。复用颜色、正文样式与复制回归，并覆盖实际带空格和 Tab 的调度器标签。

紧凑前缀验收：相关 `cli:test:race` 通过，taskui 为 9.812s，真实终端 E2E 为 11.446s；`cli:check`、`cli:verify-docs` 和 `git diff --check` 通过。本机启动入口指向已重新构建的 packages/cli/bin/one，下次运行即可生效；没有修改 Git index。
