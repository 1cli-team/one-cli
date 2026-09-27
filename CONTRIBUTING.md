# 贡献 one cli

> 这是给"要改 one cli 自己的代码"的人看的。要"用 one cli 起项目"看 [README](./README.md) + [文档站](https://1cli.dev)。

## 一次性环境

```bash
brew install go go-task node    # macOS；Linux 用 apt / dnf 类比
npm i -g pnpm@10.14.0           # 与根 package.json 的 packageManager 一致
git clone https://github.com/1cli-team/one-cli
cd one-cli
task install                    # 打包 Dashboard + CLI，再创建当前平台的本地启动器
one --version                   # 验证装好
```

工具链：**Go 1.26+**、**pnpm 10.14.0**。Node 推荐使用 **24.x（至少 24.15.0）**，也支持 22.x（至少 22.22.2）或 26+；Dashboard 测试依赖的 jsdom 不再支持 Node 20。`go-task`（不是 GNU make）是任务总线，跨平台一致。

> **fresh-clone 提示**：`packages/cli/internal/resources/bundled/` 整个目录是 gitignore 的——
> registry / templates / dashboard dist 都由 `task sync-bundled` +
> `task sync-web` 按需重建。这些任务作为 `task vet` / `test` / `build` 的依赖自动运行；
> mise 不再是内嵌资源，构建与普通测试不需要下载 mise。
> 第一次 `task install` 会触发 `pnpm install + vite build`，~30s；之后
> task fingerprint 命中，几乎零成本。如果你直接跑 `go build` 而不走 Taskfile，
> 会看到 `pattern all:_templates: no matching files found` 这种报错——跑一次
> `task sync-bundled && task sync-web` 就好。

## 日常开发

所有命令都通过 Taskfile：

```bash
task --list                 # 看可用任务（这是真源）
pnpm install               # 从根目录安装所有 Node workspace 依赖
task dev                    # 同时启动 Go Dashboard API + Vite UI
task check                  # 在当前操作系统运行 monorepo 验证入口
pnpm check                  # 根目录快捷入口，等价于 task check
task hooks:install          # 为当前 checkout 启用提交前 PR gate
task build                  # 编译到 packages/cli/bin/one
task test                   # 全套 Go 测试 + race detector
task vet                    # go vet
task fmt                    # gofmt
task install                # 打包 + 本地启动器（开发用；install-local 仍是兼容别名）
task pre-push               # 推前必跑（含上面所有 + verify-docs）
```

`build` / `test` / `vet` 都隐式依赖 `sync-bundled` + `sync-web`，所以你不用
手动跑这两个——除非要让 gopls 立刻看到 `packages/templates/` 或 `apps/dashboard/`
的改动。

## 提交流程

1. 起一个分支：`git checkout -b feat/<short-name>` 或 `fix/<short-name>`
2. 改代码 + 测试
3. 提交：commit 消息走 [conventional commits](https://www.conventionalcommits.org/)
   （`feat:`、`fix:`、`chore:`、`docs:`、`test:`、`refactor:` 等）。仓库的
   pre-commit hook 会自动运行与 PR CI 相同的 `task check`；如果当前 checkout 尚未
   启用 hook，先运行 `task hooks:install`。hook 会拒绝混合已暂存、未暂存或未跟踪的
   文件，确保本地检查的内容与即将提交、随后由 CI 检查的快照一致
4. **必跑** `task pre-push` 全绿（包含 Go race detector）
5. 推送 + 开 PR

PR CI 在 Linux 上并行执行 `task check:static`、`task check:test` 与
`task test:go`（Go race detector），同时在 Windows 上执行 `task check`、
macOS 上执行 `task check:test`。master 的保护规则要求 `lint`、`test`、
`test-windows`、`test-macos`、`test-race` 五项检查全部通过才能合并。
本地 `task check` 和 `task pre-push` 只验证当前操作系统，不能代替其他平台的
CI；`task pre-push` 额外运行 Go race detector。远端五项检查会在 PR、master
推送和手动触发的工作流中运行。

## 改不同部分的注意事项

### 改 Go 代码（`packages/cli/internal/` / `packages/cli/pkg/`）

- 公开 API（`packages/cli/pkg/`）改动要考虑 semver；详见 [CLAUDE.md 的 Public API stability](./CLAUDE.md)
- 加新错误码：在 `packages/cli/internal/platform/errors/codes.go` 注册 `Code` 常量 + `Codes` map 条目；测试会强制对应；改完跑 `task gen-error-codes` 刷新文档

### mise 运行时

One 发布文件不包含 mise 程序或压缩包。实际运行时优先使用 `ONE_MISE_BINARY`，其次使用 PATH 中兼容的 mise，否则复用或从官方 GitHub Release 下载固定版本。最低兼容版本和托管版本分别维护；系统或托管程序被删除后，下一次需要 runtime 时重新解析并按需恢复。

托管程序位于 `$XDG_DATA_HOME/one/runtimes/mise/<version>/<platform>/`，默认 `~/.local/share/one/runtimes/mise/`；工具、配置、状态和缓存分别使用对应 XDG 根下的 `one/mise/`。One 在托管子进程中设置四个 `MISE_*_DIR`，并关闭自动更新。外部 mise 沿用原目录。旧版 One 缓存中校验通过的同版本程序可以离线迁移，原缓存保留。

升级时验证上游校验文件的签名，更新 `packages/cli/internal/adapters/runtime/mise/miserelease/release.go` 中的版本、压缩包和程序 SHA256，再更新需要提高的 runtime 最低版本及相关文档。托管版本随 One 更新，不通过 `mise self-update` 维护。上游许可证保留于 `third_party/mise/LICENSE` 和 One 发布归档。

下载器及安装器测试使用本地 HTTP fixture，覆盖并发、重试、取消、摘要和删除修复。`ONE_TEST_MISE_BINARY=/absolute/path/to/mise go test ./packages/cli/tests/e2e -run Mise` 启用真实配置与信任测试；其中托管迁移测试要求与仓库固定摘要一致的官方程序。真实下载和多平台冒烟验证在发布前单独执行，不作为普通构建的资源依赖。

### 改 templates（`packages/templates/<id>/`）

- 模板会被 `go:embed` 进二进制（`task sync-bundled` 是同步入口，自动跑）
- 加新模板：在 `packages/templates/registry.json` 登记 + 加 `packages/templates/<id>/` 目录

### 改 dashboard（`apps/dashboard/`，`one serve` 的 UI）

- React + Vite，pnpm 管理
- 前后端联调：在仓库根目录运行 `task dev`，打开 `http://localhost:5173/`
- `task dev` 的 Workspace/Project 数据来自仓库内固定 fixture；Profile 增删改查仍会
  操作本机真实的 One 配置，Profile binding 也会真实写入但只关联 fixture Workspace
- 本地开发：先在仓库根目录运行 `pnpm install`，再运行 `pnpm --filter one-serve-web dev`
- 静态检查：`task check:dashboard`；架构护栏和交互测试包含在 `task check:test`
- 改完后 `task vet` / `test` / `build` 会自动跑 `sync-web`（pnpm install + vite build）
  并刷 `packages/cli/internal/resources/bundled/_web/`

### 改文档站（`apps/docs/`）

- 文档站是 Next.js + Fumadocs SSG
- 本地预览：先在仓库根目录运行 `pnpm install`，再运行 `pnpm docs:dev`
- 线上部署：Vercel 项目 Root Directory 指向 `apps/docs`，Output Directory 用 `dist`，域名绑定 `1cli.dev`
- `apps/docs/content/docs/reference/error-codes.md` **不要手工编辑**——跑 `task gen-error-codes` 重生成
- 新增页面要更新对应目录的 `meta.json`（sidebar 顺序）

### 改 install.sh（`apps/docs/public/install.sh`）

- 改完跑 `task test` —— `packages/cli/tests/e2e/install_sh_test.go` 会做静态检查（语法、必要 sentinels、wrap-in-main 不变量）

## 测试约定

```bash
task test                                                 # 默认全套
(cd packages/cli && go test ./internal/foo)               # 单个包
(cd packages/cli && go test -run TestX ./...)             # 单个 test
(cd packages/cli && UPDATE_SNAPSHOTS=1 go test ./tests/e2e)     # 重生成 e2e snapshot fixtures
```

E2E snapshot 测试位于 `packages/cli/tests/e2e/snapshot_e2e_*_test.go`，依赖 `packages/cli/bin/one` 存在 —— 跑 `task build` 之后再跑。

## 发布流程

发布统一从 GitHub Actions 的 **Build and Release** 手动触发：

1. 在 `master` 上运行工作流，选择 `patch`（默认）、`minor` 或 `major`。
2. 工作流从最高稳定 tag 自动计算下一版本，并把结果作为 `RELEASE_VERSION`；若最高 tag 位于 `master` 且尚未完成发布，则优先续跑该版本。
3. 工作流执行完整 `task pre-push` 和 GoReleaser no-publish 预构建；验证通过后创建计算出的 tag，生成 5 个平台归档及 `checksums.txt`。
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
                                 # 由 task sync-bundled + sync-web 重建
  testdata/                      # Go 测试 fixtures
  tools/                         # 内部生成器 / 校验器
                                 #   gen-error-codes / verify-cli-references / verify-help
packages/templates/              # 模板源 + registry.json（被 go:embed）
apps/docs/                       # 文档站 Next.js + Fumadocs
apps/dashboard/                  # `one serve` 用的 React + Vite UI（被 go:embed）
.github/workflows/               # ci / cli / docs
Taskfile.yml / .goreleaser.yaml  # 顶层编排（路径都按上面这套）
pnpm-workspace.yaml              # apps/* + packages/*
DESIGN.md / apps/docs/design/    # 设计源
```

## 还有问题？

- 找 issues / discussions on GitHub
- 看 [CLAUDE.md](./CLAUDE.md) 的"Don't"列表（避免常踩的坑）
- 查命令：`task --list` / `one --help`

## License

MIT — 见 [LICENSE](./LICENSE)。
