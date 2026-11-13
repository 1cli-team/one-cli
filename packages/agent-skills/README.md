# One agent skills / One AI 技能

One's official skills use the standard [Agent Skills](https://agentskills.io/specification)
format. Sources live in `skills/`; workspace copies live in `.agents/skills/`.
`one skills` forwards to `npx skills@1.7.0`. Both use the same `skills-lock.json`
and manage the same installed files.

One 官方技能使用标准 Agent Skills 格式。源码在 `skills/`，工作区副本在
`.agents/skills/`。`one skills` 透传给 `npx skills@1.7.0`，双方使用同一份
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

## Updates / 后续更新

```sh
one skills list
one skills update --project
# Equivalent / 等价：
npx skills@1.7.0 update --project
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
`npx skills@1.7.0 update one-cli --project` and confirm it is not skipped as a
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
