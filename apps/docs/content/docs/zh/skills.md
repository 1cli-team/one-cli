---
title: one skills
description: "One CLI skills（one-cli、one-migrate）—— 它们是什么、什么时候被触发、one skills install 怎么把它们装到 coding agent 里。"
---

`one` 提供两个 agent skill，跟着 binary 一起发布。装到本机的 coding agent 之后，agent 看到匹配的用户意图时会自动加载对应 skill，按里面的 playbook 执行，避免每次都得现教它怎么用 One CLI。

## 什么是 Skill

Skill 是 [agentskills.io](https://agentskills.io/specification) 提出的格式：一个目录，里面一份 `SKILL.md`（包含 frontmatter description 和 routing），可选 `references/`、`scripts/`、`assets/`。Agent 启动时读 frontmatter 决定是否激活；激活后才按需读 references 子文件 —— 上下文窗口省着用。

One CLI 自己就是这两个 skill 的提供方，所以**装 skill ≠ 装外部插件**，而是让 agent 拿到一份"如何用 One CLI"的官方说明书。

## 两个内置 skill

### `one-cli`

覆盖**已经接入 / 想接入 One CLI 工作区**的全部日常动作：创建工作区、加项目、修 manifest、查命令 / JSON schema / 错误码。

触发关键词（来自 [SKILL.md](https://github.com/torchstellar-team/one-cli/blob/master/packages/skills/one-cli/SKILL.md) frontmatter description）：

| 类别 | 中文 | 英文 |
|---|---|---|
| 创建工作区 | 建一个新项目 / 新建 workspace / 搭脚手架 | create a new project / scaffold a workspace |
| 添加项目 | 新增一个服务 / 加一个前端应用 | add a backend / add a frontend |
| 修复 / 诊断 | 检查项目 / 修一下 / manifest 不同步 | fix the workspace / manifest is out of sync |
| 查命令 / schema / 错误码 | — | look up exact command / JSON schema / error code |

### `one-migrate`

覆盖**没有命令**的那条路径：把用户**已经有的项目**（一个 Next.js / Go 服务 / 小型 pnpm / turbo / nx monorepo）改造成 One CLI 工作区。输出形态与 `one create` 产物完全一致。

触发关键词：

| 中文 | 英文 |
|---|---|
| 迁移到 one cli | migrate to one cli |
| 把项目改造成 one workspace | adopt my existing project |
| 现有项目接入 one cli | convert this repo to a one workspace |
| 把 monorepo 改成 one cli | — |

> 注意：跨技术栈代码重写（如 CRA → Next.js）以及 manifest schema 迁移**不**走这个 skill。

## 安装：`one skills install`

```bash
one skills install               # 交互式多选；默认只勾 Claude Code
one skills install -a cursor -a claude-code   # 只装到指定 agent
one skills install --yes         # 装到所有检测到的 agent（CI 用）
```

行为：

1. **自动检测**本机已安装的 coding agent（Claude Code / Cursor / Codex / Gemini CLI / GitHub Copilot / OpenCode / Cline 等 50+；完整列表见 `one skills install --help`）
2. **交互模式**：弹出多选列表，空格切换、回车确认；默认只勾 Claude Code
3. **非 TTY 或 `--yes`**：装到所有检测到的；都没检测到时 fallback 到 Claude Code 默认路径
4. **物化**到 `~/.one/skills-store/one-bundled/<skill-name>/`，每个目标 agent 在其 global skills 目录建一个 **symlink** 指向 store
5. **Windows fallback**：symlink 失败时（无 dev mode 权限）自动改为整目录 copy
6. **幂等**：跑多次都安全；升级 binary 后再跑一次会把 skill 内容同步到所有目标

完成之后 `~/.one/skills-store/one-bundled/one-cli/SKILL.md` 与 `~/.one/skills-store/one-bundled/one-migrate/SKILL.md` 就在那里，agent 走自己的 global skills 目录就能加载。

输出 schema：`one-cli/skills-install/v1`，包含 `targets`、`installed_to`、`skill_count`。

## 交互模式

直接运行 `one skills install` 会进入 agent 多选列表；适合本机人工安装。只想装到某几个 agent 时，也可以不用交互，显式传 `-a`：

```bash
one skills install -a cursor -a claude-code
```

CI 或一次性装到全部检测到的 agent 时，用：

```bash
one skills install --yes
```

## 想看 Skill 全文

Skill 全文不在本站 inline（避免与源同步漂移），直接到仓库读：

- [`packages/skills/one-cli/`](https://github.com/torchstellar-team/one-cli/tree/master/packages/skills/one-cli) —— SKILL.md + references/INDEX.md + 各 playbook
- [`packages/skills/one-migrate/`](https://github.com/torchstellar-team/one-cli/tree/master/packages/skills/one-migrate) —— 同上

或者装完之后直接打开本地 store 目录：`~/.one/skills-store/one-bundled/`。

## 想装外部 / 社区 skill

One CLI **不做**外部 skill 分发 —— 这是 [vercel-labs/skills](https://github.com/vercel-labs/skills) 的领地。装外部 skill 用：

```bash
npx skills add <github:org/repo | https://… | …>
```

格式与 One CLI skills 兼容，agent 看到的目录结构一致。

## 进一步阅读

- [`one skills install` 参考](/zh/docs/cli-overview/) —— 看 top-level 命令位置
- [agentskills.io 规范](https://agentskills.io/specification) —— skill 标准本身
- [`one create`](/zh/docs/create/) —— 装完 skill 后第一个会触发它的命令
