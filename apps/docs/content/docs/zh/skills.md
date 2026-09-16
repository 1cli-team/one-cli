---
title: one skills install
description: 为 coding agent 安装精简的 One CLI skill。
---

内置 `one-cli` skill 只有两个职责：遵循 [One Workspace Convention](https://github.com/1cli-team/one-workspace-convention)，以及通过本机 CLI 的 help 发现工作区操作。命令列表、参数、模板和 manifest schema 由 CLI 维护，避免在 skill 中复制一份。

```bash
one skills install
one skills install --agent claude-code --agent codex
one skills install --yes -o json
```

## 选择安装目标

- `--agent <id>` / `-a <id>` 显式选择 Agent，即使该 Agent 尚未创建配置目录也可安装。支持重复指定。
- 未指定目标时，交互终端列出检测到的 Agent，默认全部勾选。
- 使用 `--yes` / `-y` 或处于非交互环境时，直接安装到所有检测到的 Agent。
- 未检测到目标时，命令返回错误并提示显式使用 `--agent`。
- `one skills` 和 `one skills install --help` 只显示帮助，后者列出所有支持的 Agent ID。

检测依据是用户配置目录，兼容旧安装器的 Agent ID。`CODEX_HOME`、`CLAUDE_CONFIG_DIR`、`XDG_CONFIG_HOME` 和 `VIBE_HOME` 可用绝对目录覆盖对应的默认位置。

## 安装行为

安装器将二进制内置的 `one-cli/SKILL.md` 复制到所选 Agent 的用户级 skills 目录，可离线运行，无需进入工作区。多个目标共用同一目录时只写入一次。成功输出列出目标目录，其下的 `one-cli/SKILL.md` 就是已安装的 skill。

重复安装会替换 `one-cli` 目录，包括旧参考资料。其他 skills 保持原样。旧的 `one-cli` 符号链接会替换为直接副本，链接指向的共享 store 及其他 Agent 的链接不受影响。如果部分目标成功后发生错误，错误的 `context.installed_to` 会列出已完成的目录；修复问题后可重新执行。

安装后加载本地 skill；Agent 若无法自动发现新 skill，请重新启动会话。

## 生成项目的指引

`one create`（包括 preset 创建）会写入简短的根 `AGENTS.md`：

```markdown
# Development

Use the `one-cli` skill when developing this workspace.
If it is not installed, run `one skills install` first.
```

创建项目不向用户目录安装 skill；后续 `one add` 保留已有指引。已有工作区可单独安装 skill，再将相同要求加入自己的 `AGENTS.md`。

## 更新与输出

日常 CLI 升级无需重新安装 skill：Agent 会查看 `one --help`、`one help --all` 和对应的命令帮助。仅在 skill 的指导原则变化或配置新的 Agent 时重新安装。

结构化输出沿用 `one-cli/skills-install/v1`，包含 `status`、`targets`、`installed_to` 和 `skill_count`（值为 1）。支持通用的 `-o json`、`-o yaml`、`-o text`。安装失败使用 `SKILLS_INSTALL_FAILED`。
