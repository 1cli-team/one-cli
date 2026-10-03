# 贡献 one cli

> 这是给"要改 one cli 自己的代码"的人看的。要"用 one cli 起项目"看 [README](./README.md) + [文档站](https://1cli.dev)。

## 一次性环境

```bash
brew install mise               # macOS；其他系统先安装 mise
git clone https://github.com/1cli-team/one-cli
cd one-cli
mise trust                      # 信任当前 checkout 的根配置
mise install                    # 安装 mise.toml 固定的工具版本
mise run install                # 打包 Dashboard + CLI，再创建本地启动器
one --version                   # 验证装好
```

工具版本以根 `mise.toml` 为准：Go 1.27.1、Node 24.21.0、pnpm 12.3.4、Task 3.53.1、Process Compose 1.122.0。通过 One 执行时由官方 Process Compose 调度，mise 负责工具环境，Go 子项目使用 Task，Node 项目使用 pnpm。

> **fresh-clone 提示**：`packages/cli/internal/resources/bundled/` 整个目录是 gitignore 的——
> registry / templates / dashboard dist 都由 `mise run sync-bundled` +
> `mise run sync-web` 按需重建。这些任务作为 `mise run vet` / `build` 的依赖自动运行；
> One 使用官方 Process Compose；贡献者先安装 mise，再使用根任务准备和构建资源。
> 第一次 `mise run install` 会准备依赖并构建 Dashboard；之后输入未变时复用 Dashboard 产物缓存。
> 如果你直接跑 `go build` 而不走 mise，
> 会看到 `pattern all:_templates: no matching files found` 这种报错——跑一次
> `mise run sync-bundled && mise run sync-web` 就好。

## 日常开发

仓库本身也是 One CLI 工作区，`one.manifest.toml` 登记了四个项目：

| 项目 | 目录 | 用途 |
|---|---|---|
| `dashboard` | `apps/dashboard` | React / Vite 管理界面 |
| `docs` | `apps/docs` | Next.js / Fumadocs 文档站 |
| `cli` | `packages/cli` | Go CLI 和公开 Go 包 |
| `kernel` | `packages/kernel` | 共享 Go 内核 |

`packages/templates` 是源码素材，不登记为项目。`packages/cli` 保留现有路径，因为公开 Go 包的 module path 已包含该目录。

已有 checkout 升级后也先运行 `mise run install`，重新构建本地新版并将 `one` 启动器指向 `packages/cli/bin/one`，避免继续调用旧发行版。

完成安装后，可以直接用 One CLI 开发自身：

```bash
one                             # 查看当前工作区
one run                         # 查看根任务和项目任务
one run dev                     # Go Dashboard API + Vite UI，管理当前真实工作区
one run dev -p docs              # 文档站：http://localhost:3000
one run build -p cli             # 准备嵌入资源并构建 CLI
one run check                   # 完整仓库检查；也可简写 one check
one serve                       # 在 Dashboard 中管理当前真实仓库
```

单独运行 `one run dev -p dashboard` 只启动 Vite 前端，需要在另一终端运行 `one run dev -p cli` 提供 API；通常直接使用联合任务 `one run dev`。开发 API 与 `one serve` 都管理当前真实仓库。构建、静态检查和开发不需要绑定 Infisical。

`one run dev --ui tui` 使用 One 的运行中列表、完整任务树和日志界面，官方 Process Compose 以 headless 模式调度；`--ui stream` 使用流式输出。Tab 切换焦点，方向键展开/定位；拖选后 y 复制，c 关闭鼠标报告并固定全宽日志供终端原生复制，Esc 返回。无需声明 service。状态来自实际命令开始/结束事件，不从输出猜测。多任务交互终端默认 TUI，CI 和结构化输出使用 stream；全部完成后自动关闭，Ctrl+C 取消并恢复终端。完整日志保存在私有临时 journal，不受上游 10,000 行缓存限制。

mise 安装固定的 Process Compose 1.122.0，One 生成每次运行的私有配置；变量通过内存认证通道传给叶子进程。无需 Rust、补丁或嵌入调度器构建。首版仅支持静态任务子集，能力和限制见 [one run 文档](apps/docs/content/docs/zh/run.md)。Process Compose 没有全局并发上限，不再承诺任意 `--concurrency` 生效。

根 `mise.toml` 继续维护原生任务，供首次安装和本地检查使用：

```bash
mise run install-deps           # 安装锁定的 Node workspace 依赖
mise run check                  # Go、Dashboard 和文档静态检查
mise run build                  # 编译到 packages/cli/bin/one
mise run install                # 打包并安装本地启动器，无需预先安装 one
```

项目任务来自各自的 `package.json` 或 `Taskfile.yml`，只在根 `mise.toml` 登记 `cli:build`、`dashboard:dev` 这样的任务入口，并通过 `dir` 指定子项目目录。根文件也声明 Dashboard、模板和嵌入资源的前置步骤。新增或修改任务目录后，运行 `one init mise` 同步这些受版本控制的原生任务。执行和查询不会重新生成配置；Dashboard 从进程输出发现访问地址，dev 命令在 mise.toml 中定义，由 Process Compose 执行。

## 提交流程

1. 起一个分支：`git checkout -b feat/<short-name>` 或 `fix/<short-name>`。
2. 改代码，运行 `mise run check` 和相关构建。
3. 提交：commit 消息走 [conventional commits](https://www.conventionalcommits.org/)
   （`feat:`、`fix:`、`chore:`、`docs:`、`refactor:` 等）。
4. 推送并开 PR，附上本地验证结果。

## 改不同部分的注意事项

打包与发布流程见 [RELEASING.md](./RELEASING.md)。先运行 `mise run release -- plan patch` 查看版本与源码，再通过 `build` 验证产物、通过 `publish` 发布；GoReleaser 固定为 2.18.0。流程会在独立源码副本中检查和构建，并保留草稿及失败重试规则。

For packaging and publishing, see [RELEASING.md](./RELEASING.md). Start with `mise run release -- plan patch`, then validate with `build` and publish with `publish`. GoReleaser is pinned to 2.18.0; checks and builds use an independent source checkout and retain draft/retry safeguards.

### 改 Go 代码（`packages/cli/internal/` / `packages/cli/pkg/`）

- 公开 API（`packages/cli/pkg/`）改动要考虑 semver；详见 [CLAUDE.md 的 Public API stability](./CLAUDE.md)
- 加新错误码：在 `packages/cli/internal/platform/errors/codes.go` 注册 `Code` 常量 + `Codes` map 条目；改完跑 `mise run gen-error-codes` 刷新文档

### mise 运行时

工具环境优先使用 `ONE_MISE_BINARY`，其次使用 PATH 中兼容的 mise，否则复用或下载官方固定版本。所有 UI 模式的任务调度均使用官方 Process Compose；mise 只准备工具环境，旧的托管事件扩展和 Rust 构建流程已移除。最低兼容版本和托管版本分别维护；程序删除后，下一次使用时按需恢复。

托管程序位于 `$XDG_DATA_HOME/one/runtimes/mise/<version>/<platform>/`，默认 `~/.local/share/one/runtimes/mise/`；工具、配置、状态和缓存分别使用对应 XDG 根下的 `one/mise/`。One 在托管子进程中设置四个 `MISE_*_DIR`，并关闭自动更新。外部 mise 沿用原目录。旧版 One 缓存中校验通过的同版本程序可以离线迁移，原缓存保留。

升级时验证上游校验文件的签名，更新 `packages/cli/internal/adapters/runtime/mise/miserelease/release.go` 中的版本、压缩包和程序 SHA256，再更新需要提高的 runtime 最低版本及相关文档。托管版本随 One 更新，不通过 `mise self-update` 维护。上游许可证保留于 `third_party/mise/LICENSE` 和 One 发布归档。

### 改 templates（`packages/templates/<id>/`）

- 模板会被 `go:embed` 进二进制（`mise run sync-bundled` 是同步入口，自动跑）
- 加新模板：在 `packages/templates/registry.json` 登记 + 加 `packages/templates/<id>/` 目录

### 改 dashboard（`apps/dashboard/`，`one serve` 的 UI）

- React + Vite，pnpm 管理
- 前后端联调：在仓库根目录运行 `one run dev`，打开 `http://localhost:5173/`
- 开发 API 使用当前仓库的真实项目与任务；页面保存项目配置会更新当前仓库的 manifest，账号使用本机 One 登录会话
- 本地开发：先在仓库根目录运行 `pnpm install`，再运行 `pnpm --filter one-serve-web dev`
- 静态检查：`mise run check:dashboard`；完整仓库检查：`mise run check`
- 改完后 `mise run vet` / `build` 会自动跑 `sync-web`（pnpm install + vite build）
  并刷 `packages/cli/internal/resources/bundled/_web/`

### 改文档站（`apps/docs/`）

- 文档站是 Next.js + Fumadocs SSG
- 本地预览：在仓库根目录运行 `one run dev -p docs`
- `apps/docs/content/docs/reference/error-codes.md` **不要手工编辑**——跑 `mise run gen-error-codes` 重生成
- 新增页面要更新对应目录的 `meta.json`（sidebar 顺序）

### 改 install.sh（`apps/docs/public/install.sh`）

- 改完运行 `bash -n apps/docs/public/install.sh` 检查脚本语法。

## 环境变量（开发时常用）

| 变量 | 用途 |
|---|---|
| `ONE_BINARY_PATH` | 让 wrapper / 子 shell 用某个特定 binary |

## 仓库布局

```
packages/cli/                    # Go module（module path 含 /packages/cli 后缀）
  cmd/one/main.go                # 二进制入口（薄壳）
  internal/                      # 业务逻辑
  pkg/                           # 公开 Go API（semver 保护）
  internal/resources/bundled/              # go:embed 镜像，目录整个 gitignore，
                                 # 由 mise run sync-bundled + sync-web 重建
  tools/                         # 内部生成器 / 校验器
                                 #   gen-error-codes / verify-cli-references / verify-help
packages/templates/              # 模板源 + registry.json（被 go:embed）
apps/docs/                       # 文档站 Next.js + Fumadocs
apps/dashboard/                  # `one serve` 用的 React + Vite UI（被 go:embed）
mise.toml                        # 本地任务编排
pnpm-workspace.yaml              # apps/* + packages/*
DESIGN.md / apps/docs/design/    # 设计源
```

## 还有问题？

- 找 issues / discussions on GitHub
- 看 [CLAUDE.md](./CLAUDE.md) 的"Don't"列表（避免常踩的坑）
- 查命令：`mise tasks ls` / `one --help`

## License

MIT — 见 [LICENSE](./LICENSE)。
