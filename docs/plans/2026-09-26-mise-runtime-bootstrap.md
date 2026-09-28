# mise 按需获取与 One 托管 Implementation Plan

> 2026-09-28：runtime 获取策略仍保留；命令与任务执行路径以 [工作区任务统一方案](2026-09-28-workspace-task-unification.md) 为准。

> 使用 `executing-plans` 分步执行。用户已授权实施；验证结果记录在文末。

**Goal:** One 发布包不包含 mise 程序或压缩包；优先使用系统中可用的 mise，否则按需从官方获取，并能在外部安装或托管程序被删除后恢复运行。

**Architecture:** 在现有 mise adapter 中统一解析显式指定、系统 PATH、One 托管这三种来源；installer 负责下载、校验、加锁和原子安装。每次需要 runtime 时重新判断实际文件，不保存“安装时已检测到 mise”这样的永久状态；托管版本及校验摘要继续随 One 发布。

**Tech Stack:** Go、net/http、gofrs/flock、现有 fsutil、Cobra、Task、GoReleaser；沿用 tar.gz / zip 平台资源。

---

## 1. 需求和建议的边界

用户明确要求：

- 构建产物不嵌入 mise。
- 系统已有 mise 时优先使用。
- 系统没有或后来被删除时，能够从官方下载。
- One 下载的 mise 使用 One 管理的本地目录。

本计划建议的默认行为：

- 延迟到首次执行需要 mise 的命令时获取；安装 One 本身不强制联网下载 mise。
- 系统版本须满足现有最低版本 `2026.9.7`；过旧、不可执行或探测失败时使用托管版本。
- 保留 `ONE_MISE_BINARY` 显式指定入口；配置错误直接报错，避免悄悄切换用户明确指定的程序。
- 采用完整托管：mise 的程序、工具、配置、缓存和状态均使用 One 管理的目录。
- 系统 mise 与显式指定的外部 mise 保留其现有工具和配置目录。
- 首轮不增加后台更新服务、全局 shell 激活或新的日常命令参数。

方案比较：

| 方案 | 效果 | 结论 |
| --- | --- | --- |
| 运行时系统优先，缺失后按需托管 | 安装包小，可覆盖卸载、误删和多种 One 安装方式 | 推荐 |
| 只在 install.sh / install.ps1 中检测并安装 | 实现表面简单，但安装后被删、手动解压安装仍需运行时补救 | 不能作为唯一入口 |
| One 始终下载和使用私有 mise | 版本行为更统一，但重复已有安装 | 不符合系统优先要求 |

## 2. 当前仓库情况

- `packages/cli/internal/adapters/runtime/mise/mise.go`：当前只有显式路径与内嵌版本两条分支，没有系统 PATH 自动发现。
- `packages/cli/internal/adapters/runtime/mise/install.go`：已有 SHA256 校验、跨进程锁、二次检查、限量解压、临时文件和原子替换。
- `packages/cli/internal/adapters/runtime/mise/miserelease/release.go`：已有 `2026.9.7` 的五平台官方资源名、压缩包及程序摘要。
- `packages/cli/internal/adapters/runtime/mise/embed_*.go`：按平台嵌入 mise 压缩包。
- `packages/cli/tools/sync-mise/`：构建时下载；已有代理兼容的 HTTP 下载、有限重试、摘要和失败保留原文件测试。
- `Taskfile.yml`：`sync-bundled` 间接运行 `sync-mise`，`build-all` 运行 `sync-mise-all`。
- `.goreleaser.yaml`：before hook 下载所有平台压缩包。
- 旧程序缓存位于 `${XDG_CACHE_HOME:-~/.cache}/one/runtimes/mise/<version>/<platform>/`；目前工具、配置和状态使用 mise 原有目录。

## 3. 查找和恢复规则

```text
需要 mise 的实际执行入口
  ├─ ONE_MISE_BINARY 已设置 → 校验后使用；失败则明确报错
  ├─ 有兼容的系统 PATH mise → 使用系统安装
  ├─ One 托管目标版本完整可用 → 直接复用
  ├─ 旧 One 缓存存在且摘要匹配 → 校验并迁移程序
  └─ 从官方 Release 下载 → 校验 → 原子安装 → 使用
```

| 场景 | 行为 |
| --- | --- |
| 安装 One 前已有兼容的系统 mise | 第一次需要时找到系统程序，不下载 |
| 原来的系统 mise 被删除或 PATH 改变 | 下次执行重新解析，转用托管版本；托管版本缺失时下载 |
| 系统 mise 版本过旧或文件不可执行 | 保留外部安装，说明回退原因并使用托管版本 |
| One 托管 mise 被删除或校验失败 | 重新下载并修复；不要求重装 One |
| 系统 mise 后来重新安装 | 下次执行恢复系统优先；不自动删除托管目录 |
| 系统或托管程序可用，但下载源不可访问 | 继续使用；解析阶段不检查网络上的最新版本 |
| 所有程序都不可用，下载又失败 | 返回可操作错误，说明重试原命令或使用显式外部路径 |
| 显式指定的 mise 不存在或版本不支持 | 返回现有显式路径错误，不自动回退 |
| 帮助、版本、创建配置、dry-run | 不探测、下载或安装 mise；原来需要实际执行依赖安装的分支仍按运行时规则处理 |

查找实现要求：

- 使用待执行命令的有效 `Env/PATH`，避免全局进程 PATH 与子命令 PATH 不一致。Windows 处理 PATH 大小写和可执行后缀。
- “系统已有”具体指能在有效 PATH 找到的外部可执行程序；shell alias / function 不视为独立程序。PATH 外的安装使用 `ONE_MISE_BINARY` 指定。
- 路径需要绝对化，并识别符号链接目标是否位于 One 管理目录。系统发现阶段跳过 One 当前及旧托管目录，避免嵌套 `one` 因继承 PATH 把托管程序误当系统安装；显式指定托管目录中的程序仍采用托管环境和校验。
- 版本探测最多 5 秒，关闭探测过程的 mise 自动更新与更新提示；父 context 被取消时立即退出，不再开始下载。
- 系统发现不支持的本机平台仍可使用兼容的外部程序；只有走托管下载时才验证官方资源平台支持。
- 不写入固定的“上次来源”配置。可在一次执行准备过程中复用已验证结果，但不能跨命令永久缓存存在状态。
- 程序已启动后的命令失败保持原始退出码，不因非零退出而重新安装、重复执行用户命令。检查与启动之间被删除的极小竞态可以明确报错，下次执行自动恢复。

内部解析结果应包含来源，供环境和诊断共同使用，而不是继续用 `ONE_MISE_BINARY == ""` 推断所有权：

```go
// 示意内部类型，不变更公共 runtime.Provider 接口。
type resolvedBinary struct {
    Path    string
    Source  binarySource // explicit、system、managed
    Version string
    Managed bool         // 显式路径也可能指向 One 的托管目录
}
```

## 4. One 托管目录

沿用仓库已有的 XDG + `userdirs.Home()` 习惯，统一封装解析；以下未设置 XDG 时的路径也适用于 Windows 的有效用户 Home，路径分隔符由 Go 处理。测试通过临时 HOME / XDG 完全隔离。

| 内容 | 默认目录 | 环境根 |
| --- | --- | --- |
| mise 程序 | `~/.local/share/one/runtimes/mise/<version>/<platform>/mise[.exe]` | `XDG_DATA_HOME` |
| Node、Go、插件等 | `~/.local/share/one/mise/` | `XDG_DATA_HOME` |
| mise 全局配置 | `~/.config/one/mise/` | `XDG_CONFIG_HOME` |
| mise 状态和信任记录 | `~/.local/state/one/mise/` | `XDG_STATE_HOME` |
| mise 可清理缓存 | `~/.cache/one/mise/` | `XDG_CACHE_HOME` |

托管程序子进程中设置：

```text
MISE_DATA_DIR   = <One data root>/mise
MISE_CONFIG_DIR = <One config root>/mise
MISE_STATE_DIR  = <One state root>/mise
MISE_CACHE_DIR  = <One cache root>/mise
```

这四个变量在托管模式下由 One 设置；已有同名变量仅在系统或显式外部模式沿用。文档需明确这个行为变化，不能让目录归属依赖用户偶然继承的变量。项目中的 `mise.toml` 和 `.mise/conf.d/one.toml` 仍在项目中按原规则读取；这里管理的是用户级运行数据，不移动项目配置。

程序、安装的工具与可删除缓存分开，清理 mise 缓存不能导致程序或工具丢失。One 负责目录选择与 mise 程序生命周期；Node、Go 等工具的安装仍交给 mise。mise 官方支持这些目录变量，并明确区分缓存和安装目录。[目录说明](https://mise.jdx.dev/directories.html)。

托管模式通过子进程环境关闭 mise 自动升级，固定版本随 One 更新；不写系统环境或 shell 配置。`mise self-update` 不作为托管版本升级入口；若用户自行替换托管程序，下次摘要检查会按 One 的固定版本修复。

系统与托管目录彼此独立，因此首次切换到托管模式可能需要重新安装工具和重新建立配置授权。不会自动复制系统配置、工具目录或信任记录，也不会自动信任项目。`one mise trust` 与实际运行使用同一个解析器和相同目录。

## 5. 官方下载和版本策略

直接下载官方 GitHub Release 的平台资源。官方安装文档提供 Release 二进制及校验方式；无需在运行时执行 shell 安装脚本。[官方分发说明](https://mise.jdx.dev/installing-mise.html#github-releases)。

```text
https://github.com/jdx/mise/releases/download/v<version>/<asset-filename>
```

- 首轮沿用仓库当前已固定的版本和五平台摘要，不顺带更换 mise 版本；最低兼容版本与托管发布版本分开维护。
- One 包内只保留下载元数据、校验摘要和许可说明，不保留 mise 程序或压缩包。
- 系统版本通过最低版本检查即可；托管版本精确固定，不请求 latest，不依赖 GitHub API 查询。
- 保持 Linux musl、macOS x64 / arm64、Windows x64 的现有支持矩阵。
- 官方资源通过 HTTPS 获取并允许正常 Release CDN 重定向；拒绝降级到 HTTP，保留 TLS 校验。使用标准代理设置，不在错误信息中输出代理凭据。
- 复用现有下载错误分类：连接中断、HTTP 408 / 429 / 5xx 可有限重试；404、摘要错误、本地写入错误不重试。建议保持 4 次上限、可取消退避，整个下载阶段设置有限总时限。
- 保留现有压缩包 256 MiB、可执行文件 512 MiB 上限；限量读取，不把未知体积响应直接读入内存。
- 请求、等待锁、解压和重试等待均响应 context 取消。
- 下载进度和回退说明走 stderr，不污染用户命令 stdout 或 JSON 输出。

安装顺序：验证现有目标 → 获取按版本/平台区分的锁 → 再次验证 → 下载临时归档 → 校验归档摘要 → 只提取预期程序 → 校验程序摘要 → 设置权限 → 原子替换 → 同步目录。

临时文件在目标版本目录内创建，成功及失败均清理。首轮不持久保存完整归档，托管程序删除后需要重新下载；已有验证通过的旧 One 缓存可以离线迁移。解压路径不直接拼接归档成员名，保留现有防越界实现。并发 `one dev` / hooks / run 只应产生一次有效安装。

维护者更新托管版本时验证上游校验文件的签名，再更新版本、两类摘要及平台测试；用户运行时以 One 中已审核的摘要校验下载内容。固定版本仍需随 One 常规维护，避免长期停在旧发布。

## 6. 旧版本迁移和离线行为

- 发现旧缓存中的同版本、同平台程序时，验证固定摘要后通过临时文件原子复制到新 data 目录；避免升级 One 后无意义地重新下载。
- 不删除旧缓存，使旧版 One 仍可使用。首轮不做自动旧版本回收，避免并发 One 版本使用期间误删。
- 不移动系统 mise 的工具和用户配置；完整托管目录是新的运行数据空间。
- 新安装且没有系统 mise 的离线机器，不能再承诺首次运行成功。预装兼容 mise / 显式指定路径，或提前联网完成一次准备。
- mise 程序可离线使用不代表 Node、Go、hk 和项目依赖均已安装；对应工具和依赖仍需要预先准备。
- 安装脚本继续只安装 One。未来若需要安装时预热，应复用同一个运行时获取逻辑，不能在 shell / PowerShell 中再维护一套安装器。

## 7. 分步实施

### Task 1：解析来源和兼容策略

**Files:**
- Modify: `packages/cli/internal/adapters/runtime/mise/mise.go`
- Modify: `packages/cli/internal/adapters/runtime/mise/mise_test.go`
- Modify: `packages/cli/internal/ports/runtime/runtime.go`（仅在需要集中版本解析时）

1. 写优先级测试：显式路径、兼容系统路径、过旧系统路径、托管复用；模拟下载器确认命中前两种时请求数为零。
2. 运行测试确认当前系统发现用例失败。
3. 加入来源类型、有效 PATH 解析和兼容性探测；把当前重复的最低版本数字收敛为单一来源。
4. 增加符号链接指向托管目录、继承托管 PATH、上下文取消和外部路径包含空格的用例。
5. 定向测试通过后保留可审查 diff；不要改调用方的命令参数、环境注入和退出码契约。

Run: `go test ./packages/cli/internal/adapters/runtime/mise/...`

### Task 2：运行时下载与原子安装

**Files:**
- Create: `packages/cli/internal/adapters/runtime/mise/download.go`
- Create: `packages/cli/internal/adapters/runtime/mise/download_test.go`
- Modify: `packages/cli/internal/adapters/runtime/mise/install.go`
- Modify: `packages/cli/internal/adapters/runtime/mise/install_test.go`
- Modify: `packages/cli/internal/adapters/runtime/mise/miserelease/release.go`
- Reference / subsequently remove: `packages/cli/tools/sync-mise/main.go`, `main_test.go`

1. 将已有下载逻辑和有价值的测试迁入 adapter；HTTP client、资源地址及 fixture 摘要通过内部依赖注入，不新增生产用的任意下载地址环境变量。
2. 用 `httptest` 验证“缺失下载 → 关闭服务后复用 → 删除目标 → 恢复服务后重新下载”；当前内嵌安装器不满足该用例。
3. 保留锁内二次检查，将内嵌字节来源改为按需下载的临时文件。tar.gz 从文件流读取，zip 使用文件 ReaderAt。
4. 验证超时、取消、4xx/5xx、截断响应、两个摘要失败、大小限制、恶意归档、目录无权限和并发首次执行；失败不发布半成品。
5. 验证通过后保留可审查 diff。

Run: `go test -race ./packages/cli/internal/adapters/runtime/mise/...`

### Task 3：目录归属和旧缓存迁移

**Files:**
- Create: `packages/cli/internal/adapters/runtime/mise/paths.go`
- Create: `packages/cli/internal/adapters/runtime/mise/paths_test.go`
- Modify: `packages/cli/internal/adapters/runtime/mise/install.go`
- Modify: `packages/cli/internal/adapters/runtime/mise/mise.go`
- Modify: `packages/cli/internal/adapters/runtime/mise/mise_test.go`
- Reference: `packages/cli/internal/platform/userdirs/home.go`

1. 写临时 HOME / 各 XDG 根测试，验证缺省值、Windows 路径、空格和非绝对 XDG 路径错误；不创建真实用户目录。
2. 测试托管子进程的四个目录覆盖，以及系统/显式外部模式保持原环境；输入 Env 切片不能被原地修改。
3. 实现路径解析和托管环境；保留项目配置与工作目录。
4. 写旧缓存有效、损坏、缺失和离线迁移用例；仅复制验证通过的程序，旧文件不删除。
5. 增加 trust 与 run 使用同一状态目录，以及切换外部/托管来源不会误删任一数据空间的测试；通过后保留可审查 diff。

Run: `go test ./packages/cli/internal/adapters/runtime/mise/... ./packages/cli/internal/platform/userdirs/...`

### Task 4：去掉内嵌资源及构建下载

**Files:**
- Delete: `packages/cli/internal/adapters/runtime/mise/embed_darwin_amd64.go`
- Delete: `packages/cli/internal/adapters/runtime/mise/embed_darwin_arm64.go`
- Delete: `packages/cli/internal/adapters/runtime/mise/embed_linux_amd64.go`
- Delete: `packages/cli/internal/adapters/runtime/mise/embed_linux_arm64.go`
- Delete: `packages/cli/internal/adapters/runtime/mise/embed_windows_amd64.go`
- Delete: `packages/cli/internal/adapters/runtime/mise/embed_other.go`
- Delete after test migration: `packages/cli/tools/sync-mise/main.go`, `main_test.go`
- Modify: `Taskfile.yml`, `.goreleaser.yaml`, `.gitignore`
- Modify: `packages/cli/internal/adapters/runtime/mise/install_test.go`

1. 下载和并发测试已迁移后移除 embed 文件、内嵌资源测试和旧构建下载工具。
2. 移除 `sync-mise` / `sync-mise-all` 任务、依赖边和 build sources 中的归档项；保留模板、技能和 Dashboard 的正常资源同步。
3. 删除 GoReleaser 的 mise 下载 hook；保留上游许可文件及说明。普通构建不再请求 mise 官方站点。
4. 在没有 mise assets 的干净环境验证本机及五平台构建；记录前后产物体积，不预先承诺减少的具体数值。
5. 检查发布归档仅含预期 One 产物与文档，没有 mise 程序或压缩包；通过后保留可审查 diff。

Run: `task build`、`task build-all`；验证不再有 `sync-mise` 调用。这里“离线构建”只约束 mise 资源，不承诺首次获取 Go / Node 依赖也可离线。

### Task 5：集成测试、帮助和用户文档

**Files:**
- Modify: `packages/cli/tests/e2e/mise_runtime_test.go`
- Review: `packages/cli/tests/e2e/mise_runtime_unix_test.go`
- Modify as needed: `.github/workflows/ci.yml`, `.github/workflows/cli.yml`
- Modify: `packages/cli/internal/platform/errors/codes.go`
- Modify: `packages/cli/internal/platform/i18n/locales/en-US.json`, `zh-CN.json`
- Refresh: `packages/cli/testdata/reference/help/mise.txt`, `hk.txt`
- Modify: `CONTRIBUTING.md`
- Modify: `apps/docs/content/docs/zh/installation.md`, `run.md`, `create.md`, `hk.md`
- Review existing English pages: `apps/docs/content/docs/en/installation.md`, `run.md`, `create.md`（目前没有对应的英文 hk 页面）
- Regenerate: `apps/docs/content/docs/zh/error-codes.md`
- Update superseded runtime distribution description: `docs/plans/2026-09-14-mise-runtime-adoption.md`

1. 将“首次离线解压成功”替换为系统复用、首次获取、删除恢复、已有托管版本离线复用和全缺失离线失败的契约测试。
2. 普通 PR 检查使用本地 HTTP fixture / fake mise，不访问公网；下载和平台安装全链路可在 adapter 集成测试注入资源，真实 CLI e2e 验证外部发现与错误传播。
3. 真实 mise 环境/信任测试继续支持 `ONE_TEST_MISE_BINARY`；在明确的联网集成验证中预取官方固定版本，验证真实托管缓存复用。测试准备不得重新成为生产构建的依赖。
4. 原生 Linux、macOS、Windows 验证来源选择、路径、替换、并发和启动行为；平台 smoke test 不能仅靠交叉编译替代。
5. 更新错误提示中的 bundled/extract-only 语义，保留现有错误码兼容；下载失败须给出版本、平台、目标目录和重试方式。
6. 更新帮助快照、中英文文档，写明系统优先、首次联网、目录归属、旧缓存迁移和离线限制；通过检查后记录提交。

Run: `task gen-error-codes`、`task check`；必要时使用仓库现有方式更新帮助快照，然后检查 diff。最终运行 `task test:go` 完成 Go race 检查。

## 8. 最终验收清单

- [x] 五平台发布文件不包含 mise 程序或压缩包，构建不下载 mise。
- [x] PATH 已有兼容 mise 时无 bootstrap HTTP 请求。
- [x] 外部 mise 被删除后下一次需要 runtime 的命令成功转入下载/托管路径。
- [x] 托管程序损坏或丢失可自动恢复，已有正常程序不重复下载。
- [x] 显式外部路径无效时不静默回退。
- [x] 托管程序、工具与缓存归属清晰，清理缓存不删除工具或信任记录。
- [x] 系统 mise 和系统工具目录不被 One 的安装器覆盖或清理。
- [x] 并发首次执行只安装一次，取消和失败不遗留可执行半成品。
- [x] 旧缓存有效时可离线迁移，原有文件保留。
- [x] `one run`、`one dev`、`one hk`、`one mise` 共用同一解析策略。
- [x] 原参数、输出、退出码、项目环境隔离和配置授权行为保持契约。
- [x] 帮助、静态配置生成和 dry-run 不下载、不探测 mise。
- [x] 用户文档不再宣称“没有外部/托管 mise 时首次运行也可离线”。

## 9. 实施与验证结果（2026-09-26）

已完成系统优先解析、运行时官方下载、完整托管目录、旧缓存迁移、取消/并发/摘要验证，移除全部 mise embed 文件与构建下载工具，并更新中英文安装说明、命令帮助及错误码文档。

验证结果：

- `task check` 通过：文档与帮助校验、Go vet/gofmt、全部 Go 普通测试、Dashboard 检查及 56 个测试。
- `task test:go` 通过：全部 Go race 测试。
- `task build-all` 通过并实际生成 darwin/amd64、darwin/arm64、linux/amd64、linux/arm64、windows/amd64 五个产物。验证过程中发现旧 `for: { var: PLATFORMS, default: [...] }` 没有执行循环，改为显式平台列表。
- 在 macOS arm64 的全新临时 Home/XDG 目录中，真实官方 2026.9.7 下载成功，程序 SHA256 与固定摘要一致；阻断代理后复用成功，删除程序后重新官方下载成功。
- 将该固定官方程序用于 `ONE_TEST_MISE_BINARY=... go test ./packages/cli/tests/e2e -run Mise -count=1 -v`，全部真实集成测试通过，包括托管迁移与环境、配置分层、信任、生成任务及多进程退出。
- 本机已有二进制约 63 MiB，改动后重新构建约 31 MiB。构建任务和 GoReleaser 配置不再获取或嵌入 mise；归档仍保留上游许可证。
- 差异检查通过。保留旧 assets 目录的 ignore 规则，仅用于忽略旧 checkout 的下载残留；这些文件不参与构建。

验证范围：原生运行环境为 macOS arm64；其他平台本次完成交叉编译，原生 Linux/Windows 行为仍由仓库现有 CI 作最终验证。本次未执行 GoReleaser 发布/归档流程；提交信息以 Git 历史为准。
