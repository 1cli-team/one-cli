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

工具版本以根 `mise.toml` 为准：Go 1.27.1、Node 24.15.0、pnpm 10.14.0、Task 3.53.1。工作区由 mise 调度，Go 子项目使用 Task，Node 项目使用 pnpm。

> **fresh-clone 提示**：`packages/cli/internal/resources/bundled/` 整个目录是 gitignore 的——
> registry / templates / dashboard dist 都由 `mise run sync-bundled` +
> `mise run sync-web` 按需重建。这些任务作为 `mise run vet` / `test` / `build` 的依赖自动运行；
> mise 程序不嵌入 One 发布文件；贡献者先安装 mise，再使用根任务。
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

`packages/templates` 和测试 fixture 是源码素材，不登记为项目。`packages/cli` 保留现有路径，因为公开 Go 包的 module path 已包含该目录。

已有 checkout 升级后也先运行 `mise run install`，重新构建本地新版并将 `one` 启动器指向 `packages/cli/bin/one`，避免继续调用旧发行版。

完成安装后，可以直接用 One CLI 开发自身：

```bash
one                             # 查看当前工作区
one run                         # 查看根任务和项目任务
one run dev                     # Go Dashboard API + Vite UI，管理当前真实工作区
one run dev -p docs              # 文档站：http://localhost:3000
one run build -p cli             # 准备嵌入资源并构建 CLI
one run test -p kernel           # 测试共享 Go 内核
one run check                   # 完整仓库检查；也可简写 one check
one serve                       # 在 Dashboard 中管理当前真实仓库
```

单独运行 `one run dev -p dashboard` 只启动 Vite 前端，需要在另一终端运行 `one run dev -p cli` 提供 API；通常直接使用联合任务 `one run dev`。开发 API 与 `one serve` 都管理当前真实仓库。构建、测试和开发不需要绑定 Infisical。

`one run dev --ui tui` 显示分组日志界面，`--ui stream` 使用流式输出。交互终端中的多任务默认使用 TUI；也可在 `~/.config/one/preferences.json` 设置 `"taskUI": "tui"` 或 `"taskUI": "stream"`。CI 和结构化输出会回落 stream。所有任务名称共用自动并发策略，`--concurrency` 可指定上限。

TUI 保留任务前缀及日志原有样式，长行随窗口缩放自动换行，完整会话历史支持滚动和搜索。滚轮或 ↑/↓ 滚动，Tab 切换依赖树焦点，Home/End 跳转首尾，`f` 恢复跟随。依赖树以本次入口任务为根，←/→ 和 Enter 展开或折叠，共享依赖显示 `↪` 引用；不足 70 列时 Tab 切换全宽依赖树与日志。mise 执行结束后，TUI 自动恢复终端并返回原退出码，无需按键。

根 `mise.toml` 继续维护原生任务，供首次安装、CI 和 Git hooks 使用：

```bash
mise run install-deps           # 安装锁定的 Node workspace 依赖
mise run check                  # 完整仓库检查
mise run build                  # 编译到 packages/cli/bin/one
mise run test                   # Go race 测试 + Dashboard 测试
mise run install                # 打包并安装本地启动器，无需预先安装 one
mise run pre-push               # 推送前检查，包含 race 测试
```

项目任务来自各自的 `package.json` 或 `Taskfile.yml`，只在根 `mise.toml` 登记 `cli:build`、`dashboard:dev` 这样的任务入口，并通过 `dir` 指定子项目目录。根文件也声明 Dashboard、模板和嵌入资源的前置步骤，CLI 测试会先构建 E2E 使用的二进制。新增或修改任务目录后，运行 `one init mise` 同步这些受版本控制的原生任务。执行和查询不会重新生成配置；Dashboard 从进程输出发现访问地址，dev 命令只由 mise 定义。

## 提交流程

1. 起一个分支：`git checkout -b feat/<short-name>` 或 `fix/<short-name>`
2. 改代码 + 测试
3. 提交：commit 消息走 [conventional commits](https://www.conventionalcommits.org/)
   （`feat:`、`fix:`、`chore:`、`docs:`、`test:`、`refactor:` 等）。仓库的
   pre-commit hook 会自动运行与 PR CI 相同的 `mise run check`；如果当前 checkout 尚未
   启用 hook，先运行 `mise run hooks:install`。hook 会拒绝混合已暂存、未暂存或未跟踪的
   文件，确保本地检查的内容与即将提交、随后由 CI 检查的快照一致
4. **必跑** `mise run pre-push` 全绿（包含 Go race detector）
5. 推送 + 开 PR

PR CI 在 Linux 上并行执行 `mise run check:static`、`mise run check:test` 与
`mise run test:go`（Go race detector），同时在 Windows 上执行 `mise run check`、
macOS 上执行 `mise run check:test`。master 的保护规则要求 `lint`、`test`、
`test-windows`、`test-macos`、`test-race` 五项检查全部通过才能合并。
本地 `mise run check` 和 `mise run pre-push` 只验证当前操作系统，不能代替其他平台的
CI；`mise run pre-push` 额外运行 Go race detector。远端五项检查会在 PR、master
推送和手动触发的工作流中运行。

## 改不同部分的注意事项

### 改 Go 代码（`packages/cli/internal/` / `packages/cli/pkg/`）

- 公开 API（`packages/cli/pkg/`）改动要考虑 semver；详见 [CLAUDE.md 的 Public API stability](./CLAUDE.md)
- 加新错误码：在 `packages/cli/internal/platform/errors/codes.go` 注册 `Code` 常量 + `Codes` map 条目；测试会强制对应；改完跑 `mise run gen-error-codes` 刷新文档

### mise 运行时

One 发布文件不包含 mise 程序或压缩包。实际运行时优先使用 `ONE_MISE_BINARY`，其次使用 PATH 中兼容的 mise，否则复用或从官方 GitHub Release 下载固定版本。最低兼容版本和托管版本分别维护；系统或托管程序被删除后，下一次需要 runtime 时重新解析并按需恢复。

托管程序位于 `$XDG_DATA_HOME/one/runtimes/mise/<version>/<platform>/`，默认 `~/.local/share/one/runtimes/mise/`；工具、配置、状态和缓存分别使用对应 XDG 根下的 `one/mise/`。One 在托管子进程中设置四个 `MISE_*_DIR`，并关闭自动更新。外部 mise 沿用原目录。旧版 One 缓存中校验通过的同版本程序可以离线迁移，原缓存保留。

升级时验证上游校验文件的签名，更新 `packages/cli/internal/adapters/runtime/mise/miserelease/release.go` 中的版本、压缩包和程序 SHA256，再更新需要提高的 runtime 最低版本及相关文档。托管版本随 One 更新，不通过 `mise self-update` 维护。上游许可证保留于 `third_party/mise/LICENSE` 和 One 发布归档。

下载器及安装器测试使用本地 HTTP fixture，覆盖并发、重试、取消、摘要和删除修复。`ONE_TEST_MISE_BINARY=/absolute/path/to/mise go test ./packages/cli/tests/e2e -run Mise` 启用真实配置与信任测试；其中托管迁移测试要求与仓库固定摘要一致的官方程序。真实下载和多平台冒烟验证在发布前单独执行，不作为普通构建的资源依赖。

### 改 templates（`packages/templates/<id>/`）

- 模板会被 `go:embed` 进二进制（`mise run sync-bundled` 是同步入口，自动跑）
- 加新模板：在 `packages/templates/registry.json` 登记 + 加 `packages/templates/<id>/` 目录

### 改 dashboard（`apps/dashboard/`，`one serve` 的 UI）

- React + Vite，pnpm 管理
- 前后端联调：在仓库根目录运行 `one run dev`，打开 `http://localhost:5173/`
- 开发 API 使用当前仓库的真实项目与任务；页面保存项目配置会更新当前仓库的 manifest，账号使用本机 One 登录会话
- 本地开发：先在仓库根目录运行 `pnpm install`，再运行 `pnpm --filter one-serve-web dev`
- 静态检查：`mise run check:dashboard`；架构护栏和交互测试包含在 `mise run check:test`
- 改完后 `mise run vet` / `test` / `build` 会自动跑 `sync-web`（pnpm install + vite build）
  并刷 `packages/cli/internal/resources/bundled/_web/`

### 改文档站（`apps/docs/`）

- 文档站是 Next.js + Fumadocs SSG
- 本地预览：在仓库根目录运行 `one run dev -p docs`
- 线上部署：Vercel 项目 Root Directory 指向 `apps/docs`，Output Directory 用 `dist`，域名绑定 `1cli.dev`
- `apps/docs/content/docs/reference/error-codes.md` **不要手工编辑**——跑 `mise run gen-error-codes` 重生成
- 新增页面要更新对应目录的 `meta.json`（sidebar 顺序）

### 改 install.sh（`apps/docs/public/install.sh`）

- 改完跑 `mise run test` —— `packages/cli/tests/e2e/install_sh_test.go` 会做静态检查（语法、必要 sentinels、wrap-in-main 不变量）

## 测试约定

```bash
mise run test                                                 # 默认全套
(cd packages/cli && go test ./internal/foo)               # 单个包
(cd packages/cli && go test -run TestX ./...)             # 单个 test
(cd packages/cli && UPDATE_SNAPSHOTS=1 go test ./tests/e2e)     # 重生成 e2e snapshot fixtures
```

E2E snapshot 测试位于 `packages/cli/tests/e2e/snapshot_e2e_*_test.go`，依赖 `packages/cli/bin/one` 存在 —— 跑 `mise run build` 之后再跑。

## 发布流程

发布统一从 GitHub Actions 的 **Build and Release** 手动触发：

1. 在 `master` 上运行工作流，选择 `patch`（默认）、`minor` 或 `major`。
2. 工作流从最高稳定 tag 自动计算下一版本，并把结果作为 `RELEASE_VERSION`；若最高 tag 位于 `master` 且尚未完成发布，则优先续跑该版本。
3. 工作流执行完整 `mise run pre-push` 和 GoReleaser no-publish 预构建；验证通过后创建计算出的 tag，生成 5 个平台归档及 `checksums.txt`。
4. 只有 asset 集合完整时，GitHub Release 才会从 draft 转为公开发布。

不需要为了发布修改或提交任何版本文件，也不要手工推 tag。若发布在 tag 或 draft 创建后失败，修复问题后重新运行；工作流会自动识别安全的未完成 tag 并复用 draft。已经完整发布的版本不会被重复发布。

发布 channel：

- **GitHub Releases** — `install.sh` 下载二进制和 `checksums.txt` 的来源
- **Vercel** `https://1cli.dev` — 文档站和 `install.sh`
- ~~**npm `qzkpwoxtl`**~~ — v0.4.1 起停发

## 环境变量（开发时常用）

| 变量 | 用途 |
|---|---|
| `ONE_BINARY_PATH` | 让 wrapper / 子 shell 用某个特定 binary |
| `UPDATE_SNAPSHOTS=1` | E2E 测试重写 snapshot |
| `INFISICAL_UNIVERSAL_AUTH_*` | secrets 测试需要（一般 mock，跳过 live） |

## 仓库布局

```
packages/cli/                    # Go module（module path 含 /packages/cli 后缀）
  cmd/one/main.go                # 二进制入口（薄壳）
  internal/                      # 业务逻辑
  pkg/                           # 公开 Go API（semver 保护）
  internal/resources/bundled/              # go:embed 镜像，目录整个 gitignore，
                                 # 由 mise run sync-bundled + sync-web 重建
  testdata/                      # Go 测试 fixtures
  tools/                         # 内部生成器 / 校验器
                                 #   gen-error-codes / verify-cli-references / verify-help
packages/templates/              # 模板源 + registry.json（被 go:embed）
apps/docs/                       # 文档站 Next.js + Fumadocs
apps/dashboard/                  # `one serve` 用的 React + Vite UI（被 go:embed）
.github/workflows/               # ci / cli / docs
mise.toml / .goreleaser.yaml  # 顶层编排（路径都按上面这套）
pnpm-workspace.yaml              # apps/* + packages/*
DESIGN.md / apps/docs/design/    # 设计源
```

## 还有问题？

- 找 issues / discussions on GitHub
- 看 [CLAUDE.md](./CLAUDE.md) 的"Don't"列表（避免常踩的坑）
- 查命令：`mise tasks ls` / `one --help`

## License

MIT — 见 [LICENSE](./LICENSE)。
