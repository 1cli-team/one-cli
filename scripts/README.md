# Repository scripts / 仓库脚本

Shared build and release automation lives here. Entrypoints parse arguments and call workflows; workflow modules own their stages and rules. Use `.mts` and the small, built-in Node command runner for commands combined with program logic. Short command sequences can stay in mise/Task tasks or platform-specific Shell scripts.

共享构建与发布脚本放在本目录。入口解析参数并调用流程，具体阶段和规则由业务模块负责。需要命令与程序逻辑配合的流程使用 `.mts` 和基于 Node 内置 API 的轻量执行器。简单命令可保留在 mise/Task 任务或对应平台的 Shell 脚本中。

## Layout / 目录结构

```text
scripts/
├── desktop.mts, desktop-macos.mts   # Desktop entrypoints / 桌面入口
├── release.mts, desktop-release.mts # Release entrypoints / 发布入口
├── desktop/                        # Configuration, sidecars, bundles / 配置与构建
│   └── macos/                      # Keychain, signing, notarization / 签名与公证
├── release/                        # Plan, build, verify, publish / 发布各阶段
│   └── fixtures/                   # Release verification helper / 发布验证辅助程序
├── lib/                            # Shared infrastructure / 通用工具
├── release-assets.mts              # Platform and filename definitions / 平台与文件名
└── release-rules.mts               # Pure release validation / 纯发布规则
```

Keep entrypoint paths and exported compatibility APIs stable. Importing an entrypoint does not run it. Follow the named stages in `desktop/build.mts`, `release/build.mts`, and `release/publish.mts` when changing a workflow; add helpers where their behavior belongs, rather than collecting business logic in a generic utility module.

保留原有入口路径和兼容导出，导入入口模块不会执行流程。修改流程时，从 `desktop/build.mts`、`release/build.mts` 和 `release/publish.mts` 中有明确名称的阶段入手，按职责放置辅助函数，避免把业务逻辑集中堆进通用工具模块。

## Run / 运行

Use the Node version pinned in the root `mise.toml` to run `.mts` files directly. Node strips their TypeScript types; no compilation, `tsconfig` or npm dependencies are needed for the scripts. The command runner uses Bash on every platform; Windows development requires Git Bash with `bash` available on PATH. Frontend projects manage their own tooling dependencies.

使用根 `mise.toml` 固定的 Node 直接运行 `.mts`。Node 会移除 TypeScript 类型，脚本无需编译、`tsconfig` 或 npm 依赖。命令执行器在所有平台使用 Bash，Windows 开发需安装 Git Bash 并将 `bash` 加入 PATH。前端项目自行管理其工具依赖。

```sh
node scripts/release.mts --help
mise run check
```

Run the repository's `mise run check` after changes. The release workflow also retains its artifact verification.

修改后运行仓库的 `mise run check`。发布流程继续执行自身的产物校验。

## Command execution / 命令执行

Import `$` for captured output or `shell` to specify a working directory, environment or inherited output. Calls are synchronous to preserve build and publication ordering. Failures throw immediately; use `nothrow: true` only when the caller checks the result explicitly.

通过 `$` 获取命令输出，通过 `shell` 指定工作目录、环境或原样输出。命令同步执行，保持构建和发布顺序；失败立即抛出错误。只有调用方显式检查结果时才使用 `nothrow: true`。

```ts
import { $, shell } from "./lib/shell.mts";

const commit = $`git rev-parse HEAD`.stdout.trim();
const args = ["--locked", "--release"];
const run = shell({ cwd: "apps/desktop/src-tauri", stdio: "inherit" });
run`cargo build ${args}`;

const result = shell({ nothrow: true })`git diff --quiet`;
if (!result.ok) {
  // Handle the expected non-zero result. / 处理预期的非零结果。
}
```

The runner uses `spawnSync` with Bash's `-euo pipefail`. Literal template text is shell syntax; strings and numbers are quoted as single arguments, and arrays expand to separate quoted arguments. Interpolate values directly; do not concatenate complete command strings or add quotes around interpolations. Parse JSON from `stdout` only. Keep file operations in Node APIs so path handling stays consistent across platforms.

执行器使用 `spawnSync` 和 Bash 的 `-euo pipefail`。模板中的固定文本是 Shell 语法，字符串和数字会转义为单个参数，数组展开为多个独立参数。直接插值传入值，不拼接完整命令字符串，不在插值外再加引号。JSON 只解析 `stdout`，文件操作使用 Node API，保持跨平台路径处理一致。

Captured commands keep stdout and stderr separate; `stdio: "inherit"` preserves original output and ANSI formatting. Results expose `ok`, `exitCode`, `signal`, `stdout` and `stderr`; failures throw `CommandError` with the result attached. Commands are never echoed. macOS commands use named policies in `desktop/macos/commands.mts`: `secret` suppresses credential output and sanitizes errors; `appleCommand` retries known transient Apple network failures. Never log private command arguments or error objects.

捕获命令分别保留 stdout 与 stderr；`stdio: "inherit"` 保持原始输出和 ANSI 格式。结果提供 `ok`、`exitCode`、`signal`、`stdout` 和 `stderr`，失败时抛出携带结果的 `CommandError`。执行器不回显命令。macOS 在 `desktop/macos/commands.mts` 中使用明确的策略名称：`secret` 隐藏凭据输出并清理错误信息，`appleCommand` 仅重试已知的 Apple 临时网络故障。禁止记录私密命令的参数或错误对象。

## Modules / 模块

- `lib/shell.mts`: dependency-free command execution / 无 npm 依赖的命令执行器。
- `lib/files.mts`: SHA256, JSON files and disposable temporary directories / SHA256、JSON 文件与临时目录清理。
- `lib/repository.mts`: repository paths and Git queries / 仓库路径和 Git 查询。
- `lib/github.mts`: GitHub API calls, release lists and asset downloads / GitHub API、发布列表与附件下载。
- `lib/output.mts`: bilingual messages and errors / 双语消息与错误。
- `lib/entrypoint.mts`: main-module detection and top-level errors / 入口检测与顶层错误处理。
- `lib/retry.mts`: bounded retries; callers choose retryable errors / 有限次数重试，由调用方决定可重试错误。

Reuse these focused modules directly. `readJson<T>` parses a file, while the caller validates its schema. `withTempDirectory` cleans up after synchronous work on success or failure; release recovery directories remain owned by the release workflow and are retained.

按职责直接复用公共模块。`readJson<T>` 负责解析文件，调用方负责校验结构。`withTempDirectory` 在同步操作成功或失败后清理目录；用于发布恢复的目录仍由发布流程管理并保留。

For actual publication commands and invariants, follow [RELEASING.md](../RELEASING.md).

实际发布命令及约束见 [RELEASING.md](../RELEASING.md)。
