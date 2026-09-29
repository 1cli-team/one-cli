package creation

import (
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

// These are the verbatim file contents the scaffolder writes.

const agentsContent = `# One CLI workspace / 工作区

## English

- **Workspace:** Find the nearest one.manifest.json in the current directory or its ancestors; read it for project names and paths. apps/ holds applications, services/ backends and workers, packages/ shared libraries.
- **CLI:** Use One for supported workspace operations. Start with one --help, then one COMMAND --help; use one help --all for the full catalogue. Current help is authoritative; do not guess commands or flags.
- **Projects:** Discover template IDs with one templates -o json, then use one add TEMPLATE --name NAME --yes. Supply explicit inputs for non-interactive operations.
- **Tasks:** List with one run; execute with one run TASK -p PROJECT. Node commands live in package.json scripts (pnpm), Go commands in Taskfile (Task); workspace orchestration and tool versions live in root mise.toml. Run the relevant existing checks, tests and builds after changes.
- **Dependencies:** If missing, install Node dependencies once at the workspace root with pnpm install; use go mod download in each affected Go module. Use go mod tidy when imports change or module files need repair.
- **Environment:** Use one exec PROJECT -- COMMAND for ad hoc commands with project variables. Bound projects receive Infisical variables; One does not load local .env files. Application code consumes environment variables; do not write secret values into source or generated config.
- **Automation:** Request -o json for structured One output; branch on error.code and use error.context for recovery, rather than parsing translated messages. Preserve child-process logs as logs.

## 中文

- **工作区：** 从当前目录向上查找最近的 one.manifest.json，读取项目名称与路径。apps/ 放应用，services/ 放后端与 worker，packages/ 放共享库。
- **CLI：** 工作区操作优先使用 One。先看 one --help，再看 one COMMAND --help；完整目录用 one help --all。以当前帮助为准，不猜测命令或参数。
- **项目：** 用 one templates -o json 查询模板 ID，再用 one add TEMPLATE --name NAME --yes 添加项目。非交互操作显式提供所需输入。
- **任务：** 用 one run 列出任务，用 one run TASK -p PROJECT 执行。Node 命令写在 package.json scripts 中，由 pnpm 执行；Go 命令写在 Taskfile 中，由 Task 执行；工作区编排和工具版本放在根 mise.toml。修改后运行相关的现有检查、测试与构建。
- **依赖：** 缺失时在工作区根目录执行一次 pnpm install；Go 在受影响模块目录执行 go mod download。修改导入或修复模块文件时使用 go mod tidy。
- **环境：** 临时命令使用 one exec PROJECT -- COMMAND 加载项目变量。已绑定项目从 Infisical 注入变量；One 不读取本地 .env。业务代码读取环境变量，不把密钥值写入源码或生成的配置。
- **自动化：** One 的结构化输出使用 -o json；按 error.code 判断错误，结合 error.context 恢复，不解析翻译后的消息。子进程日志仍按日志处理。

This file is team-owned; one add preserves edits. / 本文件由团队维护，one add 会保留修改。
`

const pnpmWorkspaceContent = `packages:
  - "apps/*"
  - "services/*"
  - "packages/*"

# Native build steps used by the bundled templates.
allowBuilds:
  '@parcel/watcher': true
  '@scarf/scarf': false
  '@swc/core': true
  electron: true
  # The Electron template uses NSIS, not the optional Squirrel installer.
  electron-winstaller: false
  esbuild: true
  unrs-resolver: true
`

const gitignoreContent = `.one-run-*/

# dependencies
node_modules

# build output
dist
coverage

# environment
.env
.env.*
!.env.example

# secrets — private keys must NEVER be committed
# (the .secrets/.gitignore inside the dir is the primary defense; this is
# a belt-and-suspenders entry for the workspace root)
.secrets/keys/

# misc
.DS_Store
`

// Package manager spec strings shipped in the workspace-root
// package.json.
const packageManagerSpec = "pnpm@12.3.4"

// buildPackageJSON returns the workspace root package.json. Workspace-scope
// One configuration moved into one.manifest.json in v2 — this file is now
// a plain pnpm root with no `one` block.
func buildPackageJSON(name string) orderedJSON {
	return orderedJSON{
		{Key: "name", Value: name},
		{Key: "version", Value: "0.0.0"},
		{Key: "private", Value: true},
		{Key: "engines", Value: orderedJSON{{Key: "node", Value: "^24.15.0 || >=26.0.0"}}},
		{Key: "packageManager", Value: packageManagerSpec},
	}
}

// emptyManifest is the freshly-stamped one.manifest.json. Carries
// only the workspace identity (workspace.id + workspace.name) and an
// empty projects array; environment and project configuration are added later.
func emptyManifest(projectName string) orderedJSON {
	return orderedJSON{
		{Key: "version", Value: workspace.ManifestVersion},
		{Key: "workspace", Value: orderedJSON{
			{Key: "id", Value: workspace.GenerateProjectID(projectName)},
			{Key: "name", Value: projectName},
		}},
		{Key: "projects", Value: []any{}},
	}
}
