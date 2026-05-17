# 贡献 one cli

> 这是给"要改 one cli 自己的代码"的人看的。要"用 one cli 起项目"看 [README](./README.md) + [文档站](https://1cli.dev)。

## 一次性环境

```bash
brew install go go-task node    # macOS；Linux 用 apt / dnf 类比
npm i -g pnpm                   # 或 corepack enable && corepack prepare pnpm@10
git clone https://github.com/1cli-team/one-cli
cd one-cli
task install-local              # 编译 + symlink packages/cli/bin/one 到 ~/.local/bin/one
one --version                   # 验证装好
```

工具链：**Go 1.25+**、**Node 20+**、**pnpm 10+**。`go-task`（不是 GNU make）是任务总线，跨平台一致。

> **fresh-clone 提示**：`packages/cli/internal/bundled/` 整个目录是 gitignore 的——
> registry / skills / templates / dashboard dist 都由 `task sync-bundled` +
> `task sync-web` 按需重建，作为 `task vet` / `test` / `build` 的依赖自动跑。
> 第一次 `task install-local` 会触发 `pnpm install + vite build`，~30s；之后
> task fingerprint 命中，几乎零成本。如果你直接跑 `go build` 而不走 Taskfile，
> 会看到 `pattern all:_templates: no matching files found` 这种报错——跑一次
> `task sync-bundled && task sync-web` 就好。

## 日常开发

所有命令都通过 Taskfile：

```bash
task --list                 # 看可用任务（这是真源）
task build                  # 编译到 packages/cli/bin/one
task test                   # 全套 Go 测试 + race detector
task vet                    # go vet
task fmt                    # gofmt
task install-local          # build + symlink（开发用）
task pre-push               # 推前必跑（含上面所有 + verify-docs）
```

`build` / `test` / `vet` 都隐式依赖 `sync-bundled` + `sync-web`，所以你不用
手动跑这两个——除非要让 gopls 立刻看到 `packages/skills/` 或 `apps/dashboard/`
的改动。

## 提交流程

1. 起一个分支：`git checkout -b feat/<short-name>` 或 `fix/<short-name>`
2. 改代码 + 测试
3. **必跑** `task pre-push` 全绿
4. 提交：commit 消息走 [conventional commits](https://www.conventionalcommits.org/)
   （`feat:`、`fix:`、`chore:`、`docs:`、`test:`、`refactor:` 等）
5. 推送 + 开 PR

CI 会跑跟 `task pre-push` 等价的检查。

## 改不同部分的注意事项

### 改 Go 代码（`packages/cli/internal/` / `packages/cli/pkg/`）

- 公开 API（`packages/cli/pkg/`）改动要考虑 semver；详见 [CLAUDE.md 的 Public API stability](./CLAUDE.md)
- 加新错误码：在 `packages/cli/internal/errors/codes.go` 注册 `Code` 常量 + `Codes` map 条目；测试会强制对应；改完跑 `task gen-error-codes` 刷新文档

### 改 skills（`packages/skills/<name>/`）

- 遵循 [agentskills.io](https://agentskills.io/specification) 规范
- `task vet` / `test` / `build` 会自动重跑 `sync-bundled`；想让 gopls 立刻
  看到改动，手动 `task sync-bundled`
- 详细规则见 [CLAUDE.md](./CLAUDE.md)

### 改 templates（`packages/templates/<id>/`）

- 模板会被 `go:embed` 进二进制（`task sync-bundled` 是同步入口，自动跑）
- 加新模板：在 `packages/templates/registry.json` 登记 + 加 `packages/templates/<id>/` 目录 + 写 `ai/common.md` 工程契约

### 改 dashboard（`apps/dashboard/`，`one serve` 的 UI）

- React + Vite，pnpm 管理
- 本地开发：`pnpm --dir apps/dashboard dev`
- 改完后 `task vet` / `test` / `build` 会自动跑 `sync-web`（pnpm install + vite build）
  并刷 `packages/cli/internal/bundled/_web/`

### 改文档站（`apps/docs/`）

- 文档站是 Next.js + Fumadocs SSG
- 本地预览：`pnpm --dir apps/docs install && pnpm --dir apps/docs dev`
- 线上部署：Vercel 项目 Root Directory 指向 `apps/docs`，Output Directory 用 `dist`，域名绑定 `1cli.dev`
- `apps/docs/content/docs/reference/error-codes.md` **不要手工编辑**——跑 `task gen-error-codes` 重生成
- 新增页面要更新对应目录的 `meta.json`（sidebar 顺序）

### 改 install.sh（`apps/docs/public/install.sh`）

- 改完跑 `task test` —— `packages/cli/internal/cli/install_sh_test.go` 会做静态检查（语法、必要 sentinels、wrap-in-main 不变量）

## 测试约定

```bash
task test                                                 # 默认全套
(cd packages/cli && go test ./internal/foo)               # 单个包
(cd packages/cli && go test -run TestX ./...)             # 单个 test
(cd packages/cli && UPDATE_SNAPSHOTS=1 go test ./internal/cli)   # 重生成 e2e snapshot fixtures
```

E2E snapshot 测试位于 `packages/cli/internal/cli/snapshot_e2e_*_test.go`，依赖 `packages/cli/bin/one` 存在 —— 跑 `task build` 之后再跑。

## 发布流程

```bash
# 1. 改 VERSION
echo "0.4.3" > VERSION

# 2. 改 CHANGELOG.md（unreleased 段升级为新版本）

# 3. 跑 e2e 测试里 hard-coded 的 want 字面量
sed -i.bak 's/want := "0.4.2/want := "0.4.3/' packages/cli/internal/cli/snapshot_e2e_test.go

# 4. task pre-push 全绿

# 5. commit
git add VERSION CHANGELOG.md packages/cli/internal/cli/snapshot_e2e_test.go
git commit -m "chore(release): v0.4.3"

# 6. push + tag
git push origin master
git tag v0.4.3
git push origin v0.4.3
# → cli workflow 触发，自动 cross-compile + 上传 GitHub Release assets
```

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
  internal/bundled/              # go:embed 镜像，目录整个 gitignore，
                                 # 由 task sync-bundled + sync-web 重建
  testdata/                      # Go 测试 fixtures
  tools/                         # 内部生成器 / 校验器
                                 #   gen-error-codes / verify-versions / verify-cli-references
packages/templates/              # 模板源 + registry.json（被 go:embed）
packages/skills/                 # bundled skill 源（被 go:embed）
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
