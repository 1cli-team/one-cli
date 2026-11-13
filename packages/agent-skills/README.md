# One agent skills / One AI 技能

One's official skills use the standard [Agent Skills](https://agentskills.io/specification)
format. Sources live in `skills/`; workspace copies live in `.agents/skills/`.
`one skills` forwards to `npx skills@1.8.1`. Both use the same `skills-lock.json`
and manage the same installed files.

One 官方技能使用标准 Agent Skills 格式。源码在 `skills/`，工作区副本在
`.agents/skills/`。`one skills` 透传给 `npx skills@1.8.1`，双方使用同一份
`skills-lock.json` 和同一套已安装文件。

## First installation / 首次安装

Templates and official skills ship with the CLI. Creating a workspace installs
`one-cli`; adding a project selects its stack skills. Official files come from
the binary, without downloading the repository. One registers their actual
content hash, GitHub source, skill path and `master` branch in the standard
Skills lock file. Later updates are handled entirely by the upstream tool.

模板和官方技能随 CLI 发布。创建工作区时安装 `one-cli`，添加项目时安装对应
技术栈技能。官方文件直接取自当前 CLI，无需下载仓库。One 将文件的实际摘要、
GitHub 来源、技能路径和 `master` 分支登记到标准锁文件，后续更新完全交给上游。

New workspaces use the upstream version 2 lock format. Initializing an existing
workspace preserves its lock version and uses the corresponding content hash,
without rewriting unrelated entries. Upstream commands handle migration from
version 1 when needed.

新工作区使用上游第 2 版锁文件格式。初始化已有工作区时，保留原锁文件版本，
使用对应的内容摘要算法，不改写其他技能记录。需要迁移第 1 版锁文件时，由上游命令处理。

The bundled names are `one-cli`, `one-go`, `one-web`, `one-nestjs`, `one-expo`,
`one-electron` and `one-fumadocs`. Third-party and business skills are not embedded.
Preparing Node or installing third-party skills can require network access.
Use `--skip-skills` during creation to skip automatic skill installation.

内置技能包括上述七种；第三方和业务技能不内置。准备 Node 或安装第三方技能
仍可能需要联网。创建时可用 `--skip-skills` 跳过自动安装。

## Initialize or register an older workspace / 初始化或登记旧工作区

From the workspace root or a project inside it:

在工作区根目录或其中的项目目录运行：

```sh
one init skills --dry-run
one init skills
```

By default, this installs missing `one-cli` guidance and inspects other official
skills already present or recorded in the lock. To select specific official
skills, use `--skill one-cli,one-go` or repeat `--skill`. Add `-o json` for
structured output.

默认补齐缺失的 `one-cli`，并检查其他已经安装或登记的官方技能。可用
`--skill one-cli,one-go` 或重复 `--skill` 指定技能；加 `-o json` 可获取结构化结果。

Initialization never replaces existing skill files. Known older local copies
are registered with the GitHub update source only if their files still match
the installation hash, or are identical to the current bundle. New, changed or
deleted files count as local edits and are skipped. Unknown sources, separate
agent copies, global installations and existing GitHub source choices are
preserved. Unchanged legacy GitHub entries missing the skill path can have it
repaired without changing their selected ref. The preview reports each action.

初始化不会替换已有技能文件。旧本地副本只有在文件与安装摘要一致，或与当前
内置文件完全相同时，才会登记 GitHub 更新来源。文件新增、修改或删除都算本地
修改，会跳过。未知来源、单独维护的 Agent 副本、全局安装，以及已经选择的
GitHub 来源均保留。缺少技能路径的旧 GitHub 记录可以在内容未修改时补齐路径，
保留原有分支或固定版本。预览会说明各项操作。

## Install missing stack skills / 补装技术栈技能

Older workspaces can scan all registered projects and choose missing skills:

旧工作区可以扫描所有已登记的子项目，选择并补齐缺失技能：

```sh
one init skills --install-recommend
# Preview without downloads or changes / 只预览，不下载、不修改文件
one init skills --install-recommend --dry-run -o json
# Install the default recommendations without a prompt / 无交互安装默认推荐项
one init skills --install-recommend --yes
```

The chooser lists skill names, preselects missing recommendations, and keeps
existing skills separate. Use Space to toggle, Ctrl+A to select or clear all,
and `/` to search. After selecting skills, choose target agents, project/global
scope and symlink/copy mode once. Review the selected skills by source, then
start installation or return to change your choices. Changing scope refreshes
the installed-skill list before confirmation.

选择界面只列技能名，默认勾选推荐且缺失的项，已安装项单独列出。用空格勾选或
取消、Ctrl+A 全选或清空、`/` 搜索。选择技能后，统一选择目标 Agent、当前项目
或全局范围、软链接或复制方式。最后按来源查看安装清单，确认开始，或返回修改。
切换安装范围时，会刷新该范围内的已安装技能，并再次确认技能选择。

For scripts, `--yes` skips selection, settings, confirmation and retry prompts.
It defaults to the current project, detected agents (or `universal` when none
are detected), and symlinks. Override with `--agent codex,claude-code`,
`--global`, and `--copy`. These flags require `--install-recommend`.
Empty selection installs nothing. `--skill` cannot be combined with this mode.

脚本使用 `--yes` 跳过选择、设置、确认和重试询问。默认安装到当前项目，使用检测到
的 Agent（未检测到时使用 `universal`）和软链接；可用 `--agent codex,claude-code`、
`--global`、`--copy` 覆盖。这些参数需要配合 `--install-recommend`。全部取消则不
安装任何技能；`--skill` 不能与该模式同时使用。

Manifests do not record original template IDs. Recommendations use each
project's current dependencies, including declared package members, with the
same skill mappings used by template creation. Root tooling is not treated as
a subproject's framework. Unknown/empty stacks get common guidance; uncertain
or unreadable projects can still be handled by manual selection. Python,
Flutter and Swift currently get the common skills because the catalog has no
dedicated stack skills for them.

Manifest 没有记录最初的模板 ID。推荐会根据每个子项目的实际依赖（含其声明的
包成员）识别技术栈，并复用创建模板时的技能清单，不会把根目录的工具依赖当作
子项目框架。未知或空项目只推荐通用指引，无法识别或读取的项目可手动选择。
当前清单没有 Python、Flutter 和 Swift 专属技能，因此这些项目推荐通用项。

One provides the shared prompts and progress display. The pinned upstream
Skills API performs each source installation and owns files and lock entries;
`one skills` remains a native CLI passthrough. Sources install sequentially,
and a failure does not stop other sources. The result lists failures with their
reasons and offers to retry only failed skill/agent targets. Successful targets
are kept. A remaining failure returns a nonzero exit status.

One 负责统一询问和进度展示，固定版本的上游 Skills API 负责逐来源安装、文件和锁记录；
`one skills` 命令仍原样透传到上游 CLI。来源顺序安装，某个来源失败后继续其他来源。
结果列出具体失败原因，并可只重试失败的技能和 Agent 组合，保留已成功的目标。
最终仍有失败项时返回非零退出码。

`-o json` returns a structured `one-cli/install-skills/v1` result; progress is
written to stderr. `--dry-run -o json` keeps the recommendation preview and
never downloads or installs. A global preview reads workspace recommendations;
global installed skills are checked during the installation flow.
Regular `one init skills` continues to initialize official skills from the bundle.

`-o json` 返回 `one-cli/install-skills/v1` 结构化结果，进度写入 stderr；
`--dry-run -o json` 仍只预览推荐项，不下载、不安装。全局预览只读取工作区推荐，
安装流程中再检查全局已有技能。普通 `one init skills` 继续使用内置文件初始化官方技能。

## Updates / 后续更新

```sh
one skills list
one skills update --project
# Equivalent / 等价：
npx skills@1.8.1 update --project
# Update just one skill / 只更新一个：
one skills update one-cli --project
```

Updates follow the recorded repository branch, independently of CLI releases.
No GitHub Release is needed for skill changes merged to `master`. The first
bundled copy can be older than master; its recorded hash always describes the
files actually installed. Upgrading the CLI alone does not modify workspace
skills, and repeating `one init skills` does not replace an upstream-managed
copy with the bundled version.

更新跟随所记录的仓库分支，独立于 CLI 发版。技能改动合并到 `master` 后即可
通过上游更新，不需要 GitHub Release。首次内置副本可能比 master 旧，锁文件
摘要始终对应实际安装的文件。单独升级 CLI 不会改动工作区技能，再次运行
`one init skills` 也不会用内置版本覆盖已由上游管理的副本。

Upstream updates can overwrite hand-edited skill files. Keep durable team rules
in `AGENTS.md` or a separate skill. Existing fixed refs stay fixed; to explicitly
switch their update source to master, save custom edits before reinstalling:

上游更新可能覆盖手工修改的技能文件。长期团队规则建议放在 `AGENTS.md` 或独立
技能中。已有固定版本保持固定；要明确切换到 master，先保存自定义内容再安装：

```sh
one skills add 1cli-team/one-cli/packages/agent-skills#master --skill one-cli one-go --agent universal --copy
```

Global installation requires an explicit `--global`, followed by
`one skills update --global`. Business skills are installed only on request.
Optional skill `metadata.version` is descriptive; update tracking uses the
standard Git reference and content hash.

全局安装需显式添加 `--global`，之后使用 `one skills update --global` 更新。
业务技能仅按用户要求安装。可选的 `metadata.version` 仅用于说明，更新跟踪使用
标准的 Git 引用和内容摘要。

## Test locally / 本地测试

Build the repository version first; use its absolute path so an older globally
installed binary is not tested by accident:

先构建仓库版本，并使用绝对路径，避免误测全局安装的旧版：

```sh
mise run build
export ONE_TEST_BINARY="$PWD/packages/cli/bin/one"
ONE_TEST_ROOT="$(mktemp -d /tmp/one-skills-demo.XXXXXX)"
cd "$ONE_TEST_ROOT"
export ONE_HOME="$PWD/.one-test-home"
"$ONE_TEST_BINARY" create demo --yes --skip-skills
cd demo
"$ONE_TEST_BINARY" init skills --dry-run -o json
"$ONE_TEST_BINARY" init skills -o json
"$ONE_TEST_BINARY" skills list
```

The preview must not create `.agents/skills/one-cli` or `skills-lock.json`.
After initialization, the one-cli lock entry must contain `sourceType: github`,
`source: 1cli-team/one-cli`, `ref: master`, a `skillPath` ending in
`one-cli/SKILL.md`, and a 64-character `computedHash`. A second initialization
reports `managed` without changing files. With network access, run
`"$ONE_TEST_BINARY" skills update one-cli --project` or
`npx skills@1.8.1 update one-cli --project` and confirm it is not skipped as a
local source. Update commands fetch the repository's published master content,
which may differ from uncommitted changes in this checkout.

预览不应创建技能文件或锁文件。初始化后，one-cli 的记录应包含上述 GitHub 来源、
master 分支、技能路径和 64 位摘要；第二次初始化应显示 `managed`，不改文件。
联网后运行上述 update 命令，确认它不会因“本地来源”被跳过。更新取得的是仓库
已发布的 master 内容，可能与当前尚未提交的代码不同。

Repository checks include initialization, legacy migration, local edits, source
preservation, lock compatibility, bilingual help and command passthrough:

仓库检查覆盖初始化、旧来源迁移、本地修改保护、来源保留、锁文件兼容、中英文
帮助和命令透传：

```sh
mise run --jobs 1 check
```
