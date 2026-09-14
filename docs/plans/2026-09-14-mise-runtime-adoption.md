# One CLI 接入 mise

日期：2026-09-14。状态：第一阶段已在工作区实现，等待用户测试。所有变更保持未提交；用户测试确认后再整理提交，不按阶段制造多个 commit。

目标：保持原有命令，让 mise 承担工具版本和运行环境，One 继续提供 workspace、模板、项目选择、密钥和部署语义。

## 命令兼容策略

```bash
one create my-app -y
cd my-app
one add react-spa --name web -y
one add go-api --name api -y
one dev
one dev web
one run web -- pnpm build
one run api -- go test ./...
```

这些命令不增加必填参数，不要求改写成 `mise run`。新 workspace 自动生成配置；根 `.mise/conf.d/one.toml` 存在时，`run` / `dev` 自动使用 mise。旧 workspace 没有该文件则保持原行为，不因升级 One 自动迁移。

旧 workspace 的一次性接入和配置修复使用 `one configure mise`，可先加 `--dry-run -o json` 审查文件差异。`ONE_RUNTIME=builtin` 是临时诊断开关；取消该变量即可恢复自动选择。当前不升级 Manifest schema，不新增 `--runtime` 日常参数。

## 职责与配置归属

| 内容 | 权威来源 | One 的职责 | mise 的职责 |
| --- | --- | --- | --- |
| Workspace、项目名称/路径、模板 | Manifest v1 | 创建、查询、修改 | 使用选定目录 |
| 工具版本 | 生成默认配置 + 用户 mise 覆盖 | 生成确定版本的默认值 | 解析、安装、提供环境 |
| 包管理器版本 | 根 package.json 的 packageManager | 映射精确版本 | 提供包管理器 |
| 应用依赖 | package / pnpm / Go 配置与锁文件 | 调用原有安装入口 | 提供执行工具 |
| dev 命令 | Manifest 的 domains.dev.command | 按项目解析，保持 Dashboard 编辑行为 | 提供工具环境 |
| build/test/lint | 项目 scripts / Taskfile / Go 命令 | 发现能力并生成引用 | 可直接执行生成任务 |
| 项目密钥、环境选择 | One dotenv / Infisical loader | 在执行末端逐项目注入 | 提供环境默认值 |
| 多服务日志与生命周期 | One supervisor | 项目选择、日志、信号、整组停止 | 每个项目的工具环境 |
| CI、部署、镜像、本机连接 | 当前 One 实现 | 本轮保持现有执行路径 | 后续接入 |

mise 支持工具环境和任务，适合用作 One 的执行基础；首轮不依赖实验性的 daemon、项目图或远程缓存。[工具管理](https://mise.jdx.dev/dev-tools/)、[任务系统](https://mise.jdx.dev/tasks/)、[Monorepo](https://mise.jdx.dev/tasks/monorepo.html)。

## 执行路径

```text
one run <project> -- <argv>
  → One 解析项目和运行方式
  → mise exec -- <同一个 One 二进制> __exec ... -- <argv>
  → __exec 注入所选项目的 One 环境变量
  → 原始 argv / 工作目录 / 标准 IO / 应用退出码

one dev [project]
  → 原有 supervisor 选择项目
  → 每个项目调用同一个 One 二进制的 run
  → 上面的 mise 执行路径
```

`__exec` 是隐藏的内部协议入口，校验协议版本且不再次进入 mise。原始命令按参数数组传递；Manifest shell 命令沿用对应平台的 shell 语义。环境覆盖顺序为父进程 → mise → 当前项目的 One 值，最后补充 node_modules/.bin，保留 mise 的工具路径。

常规 `one run` / `one dev` 绑定当前 One 二进制。手动运行生成的 mise 任务时默认使用 PATH 中的 One，也支持 `ONE_BINARY_PATH` 指定测试二进制。生成任务只引用项目/操作，不复制 Manifest 或 package scripts 的命令正文。

发布的 One 文件内置当前平台的 **mise 2026.9.7** 官方压缩包，用户只安装 One。首次运行校验内置包，从自身解压可执行文件，校验后原子写入私有缓存；运行时不下载 mise，并在内置 runtime 子进程中关闭 mise 自身的自动升级和更新检查。默认始终使用内置固定版本；`ONE_MISE_BINARY` 可显式指定兼容程序（最低 **2026.9.7**），指定程序不可用或版本不支持时直接报告错误。无需 shell 激活。[mise exec](https://mise.jdx.dev/cli/exec.html)。

运行时缓存位于 `$XDG_CACHE_HOME/one/runtimes/mise/<version>/<platform>/`，默认 `~/.cache/one/runtimes/mise/`。压缩包与可执行文件分别固定 SHA256；并发解压使用文件锁，缓存损坏从内置资源恢复。Linux 使用 musl 发行包；覆盖 One 的五个发布平台，每个 One 文件只携带对应平台资源。`sync-mise` 在构建时准备本机资源，`sync-mise-all` 和 GoReleaser 准备全部平台；资源不加入 Git。保留上游 MIT 许可证。

One 仅修改子进程 PATH，不覆盖现有 mise，不额外设置自动信任。mise 默认模式可能自动信任执行中的配置；设置 `MISE_PARANOID=1` 可要求显式授权。创建、配置生成和 dry-run 均不触发解压。代价是 One 文件增大，首次运行需要缓存空间；Node、Go 等工具和应用依赖的下载仍由各自工具处理。

提供可选的 `one mise <args...>` 原生转发入口，用同一套 runtime 解析访问 trust、doctor、exec 等操作；该入口不先运行 mise exec，也不额外加载 One 项目密钥，因此未信任配置时也能执行显式 trust。普通 `create/add/dev/run` 仍保持原用法。

## 配置与写入边界

```text
workspace/
  one.manifest.json
  package.json
  .mise/conf.d/one.toml             # One 管理：根工具和 monorepo config_roots
  mise.toml                        # 可选：用户覆盖，One 不改写
  apps/web/
    package.json
    .mise/conf.d/one.toml           # One 管理：one:dev/build/test/lint
  services/api/
    go.mod
    .mise/conf.d/one.toml           # One 管理：Go/task 工具和任务引用
```

采用 mise 原生配置片段，避免重写用户 TOML、注释和任务发现规则；同目录用户 `mise.toml` 优先于生成片段。[配置优先级](https://mise.jdx.dev/configuration.html)。

生成文件带协议标记和内容校验；用户修改过的内容不会被覆盖。配置计划是纯静态读取，记录输入修订，应用前重新检查；使用文件锁和原子替换，失败时恢复本次已写文件并报告恢复错误。已知冲突在添加项目之前检查；添加成功后若配置写入失败，保留项目与 Manifest，明确提示修复后执行 `one configure mise`，不恢复可能覆盖并发编辑的旧 Manifest 快照。

Node 默认 `24.15.0`，符合当前模板声明；包管理器从根 packageManager 取得。Go 根据 go.mod 的 go/toolchain 取得，显式 override 不允许低于最低 go 要求。重复生成沿用工具 override，无变化时无 diff。

本轮只提供精确版本默认声明；完整 npm engines 范围校验、用户覆盖冲突诊断、跨平台工具锁文件生成尚未实现。用户指定 Node 版本时需确认 engines 兼容性。每个配置根保留独立锁文件布局，不启用新的统一 monorepo 锁模式。[mise.lock](https://mise.jdx.dev/dev-tools/mise-lock.html)。

## 分阶段推进

| 阶段 | 交付与边界 | 验收 |
| --- | --- | --- |
| 第一阶段：当前试用 | 原命令自动接入、runtime 端口、创建/添加配置、静态预览、生成任务、原 supervisor | 参数/目录/退出码、项目环境隔离、动态 dev 命令、幂等/冲突、双服务停止、仓库检查 |
| 第二阶段：工具与平台完善 | Node engines、包管理器冲突、干净机器工具安装、所有平台锁文件、macOS/Windows 实机兼容 | 原生平台 shell/TTY/信号，固定工具下载结果，旧项目接入/回退 |
| 第三阶段：CI 和部署构建 | 固定版本 mise、与本地共用工具和受支持的构建任务 | 构建失败不上传，不重复 build，产物契约不变，非交互信任与缓存 |
| 第四阶段：收尾 | 诊断展示、安装体验、模板/技能说明、评估可删除代码 | 用户体验验证，再决定是否替换其他执行逻辑 |

CI 与部署必须分别梳理 Cloudflare/EdgeOne 的本地 build、S3-compatible 的独立 build、Docker 镜像构建和 Vercel 的远程构建；不能假设宿主 mise 环境自动适用于镜像。当前 One 自身 Taskfile 继续作为贡献者入口。

保留 supervisor 是第一阶段的正式方案。完整替换前须验证 Expo 终端输入、长日志、孙进程和强制清理；mise interactive/raw 的标准 IO 约束以及 daemon 的生命周期不等同于 One 前台多服务契约。[任务配置](https://mise.jdx.dev/tasks/task-configuration.html)、[Daemons](https://mise.jdx.dev/daemons.html)。

## 测试与交付

- 使用临时 workspace、假密钥和本机服务验证，不连接真实部署账户。
- `mise_runtime_test.go` 验证原参数、退出码、环境顺序、静态预览、自动生成、冲突预检和真实生成任务。
- `mise_runtime_unix_test.go` 验证 Ctrl-C / 子服务失败之后双服务释放端口。
- 设置 `ONE_TEST_MISE_BINARY` 可启用真实 mise 集成；环境与任务测试通过用户 system 工具覆盖避免下载，不能替代干净机器安装验收。
- installer 测试覆盖并发解压、tar/zip 校验、缓存损坏恢复、摘要失败和取消清理，并验证内置的真实官方压缩包。Linux 默认 e2e 在没有 PATH mise、不可用代理、空缓存的临时 workspace 中验证原始 One 命令首次执行和缓存复用。`ONE_TEST_MISE_BINARY` 可额外指定真实 mise 验证配置分层、信任和任务。
- 执行仓库规定的 `task check`，进程和配置相关包补充 race 检查；Windows 交叉编译不能代替实机验证。
- 用户测试完成后再提交。后续阶段按实际验收推进，本轮不宣称 CI、部署、完整锁文件或跨平台实机验证已完成。
