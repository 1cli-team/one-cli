> 调度接入继续使用。本计划中的原生 TUI 已由 [One 定制界面](2026-09-30-process-compose-custom-tui.md) 替代；下文保留首版验收记录。

# Process Compose 首版接入

用户已批准使用官方 Process Compose 的调度器和原生 TUI。此前定制 mise 方案和 Go 自研调度器提案由本计划替代。

## 边界

- 原版 mise 安装工具与 Process Compose，首版固定 Process Compose 版本；不维护补丁或 Rust 构建。
- One 的命令、项目选择和变量加载保持统一入口。首版读取现有 mise.toml 的静态任务子集，转换为本次调用私有的 Process Compose 配置，避免要求用户先迁移工作区。
- Process Compose 负责依赖、并发执行、状态、日志与 TUI。接受上游界面布局；VS Code 的复制体验按上游能力实测，不能将旧 One TUI 的功能报告为已接入。
- One 的任务叶子执行器负责顺序命令、参数、工具环境和变量快照；不另写调度器。变量只在本次进程内存和认证的本地通道中流转，生成配置不含变量值。
- depends 转为成功完成条件；wait_for 只连接本次已选节点。聚合节点等待依赖完成。实际执行与 dry-run 使用同一静态计划；不支持的任务行为在启动前明确报错。

## 实施步骤

1. 校验 mise 注册表和官方 CLI，增加固定版本工具准备任务；删除定制 mise 的构建与发布入口。
2. 统一静态任务发现与计划，处理别名、通配符、等待边和循环，验证支持字段、文件任务、配置覆盖。
3. 编写配置转换与认证叶子执行协议，直接传入项目变量快照，保留批量加载与项目隔离；禁用隐式 dotenv 与配置替换。
4. 接入官方 TUI / stream / raw，并保留退出码、信号和子进程清理。One 自定义 TUI 实现暂时保留为独立模块，不参与这一运行路径。
5. 通过真实 Process Compose 验证共享依赖、安静长任务、顺序步骤、健康检查放行、失败、取消、参数和变量；同步中英文文档与 CLI 文案。
6. 运行受影响检查、CLI 测试和真实终端验收；以 Drama/Composer 静态图核对首版范围。记录实际通过项与尚未覆盖的能力。

## 支持范围

第一版支持 run 字符串/数组、dir、description、alias、raw_args、raw/interactive、depends、wait_for、本地文件任务、静态任务 env、sources/outputs 的时间戳新鲜度和 force。复杂模板、usage、depends_post、远程 include、任务继承和产物缓存不静默转换。显式并发限额只有在上游可等价表达时启用，否则给出明确错误。

CI/安装引导现有的原版 mise 任务入口先保持，发布路径移除定制 runtime。业务 one run 走 Process Compose。当前首版不会自动改写 Drama/Composer 的用户配置或直接提交代码。

## 验证记录

- 官方 Process Compose 1.122.0 已通过 mise 安装并运行，注册表指向官方项目。运行路径没有 mise 补丁或定制 runtime。
- `one run check --ui stream` 通过：文档引用与帮助快照、Go vet/gofmt、Go 单元测试与 CLI E2E、Dashboard 静态检查和 101 项测试。
- `one run test:go --ui stream` 通过：CLI 与 Kernel 的 Go race 检查，包含认证通道、项目环境、并发调用和 CLI E2E。
- `one run build-all --ui stream` 通过：darwin/amd64、darwin/arm64、linux/amd64、linux/arm64、windows/amd64。未在 Windows/Linux 本机运行 UI 验收。
- 真实 Process Compose 回归覆盖：共享别名依赖去重、顺序命令、无输出任务、wait_for 放行、所选不支持字段提前拒绝、参数原样传递、项目变量隔离与轮换、依赖准备阶段取消、业务失败退出码、子孙进程清理，以及原生 TUI 缩放、滚动历史、依赖图和终端恢复。
- PTY 复制模式回归通过：Ctrl+S 冻结日志时任务继续执行，再次 Ctrl+S 退出，F5 恢复跟随。实际剪贴板写入与 VS Code 鼠标拖选尚未完成人工验收；界面控制工具出现剪贴板读取超时和屏幕捕获失败，不能将该项报告为通过。
- Drama 与 Composer 的 `dev` dry-run 静态任务图通过。未由本任务启动完整业务服务。
- E2E 发现并修复了两项执行问题：保留第一个业务失败退出码，避免上游关闭兄弟任务的 130 覆盖它；准备阶段按进程树清理 mise 创建的独立进程组。
- 代码未由本任务提交。
