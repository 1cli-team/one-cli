---
title: 持续集成
description: 查看、启用、更新或移除可选的持续集成工作流。
---

持续集成是可选能力。`one create` 和 `one add` 都不会生成工作流文件。当前版本
支持 GitHub Actions。

## 用法

```bash
one ci
one ci enable [project]
one ci sync [project]
one ci disable [project]
```

`one ci` 是只读命令，会显示每个项目的当前状态和最合适的下一步命令。

## 命令

| 命令 | 行为 |
|---|---|
| `one ci` | 查看全部项目的持续集成状态 |
| `one ci enable web` | 为 `web` 生成标准工作流 |
| `one ci enable` | 为全部项目启用持续集成 |
| `one ci sync web` | 在已经启用时重新生成 `web` 的工作流 |
| `one ci sync` | 更新所有已经存在的工作流 |
| `one ci disable web` | 确认后删除 `web` 的生成工作流 |
| `one ci disable --yes` | 不询问，删除全部生成工作流 |

拒绝停用确认或按 Ctrl-C 都属于正常取消：命令以 0 退出，文件保持不变。

## 文件与工作区状态

GitHub Actions 的标准文件按项目写入 `.github/workflows/`。持续集成选择不会写入
`one.manifest.json`，生成文件本身就是状态。因此 `enable`、`sync`、`disable` 都
不会修改 manifest。

## 自动化

```bash
one ci -o json
one ci enable web --provider ci/github-actions -o json
one ci sync --project web -o json
one ci disable web --yes -o json
```

稳定 schema 是 `one-cli/ci-status/v1`、`one-cli/ci-enable/v1`、
`one-cli/ci-sync/v1` 和 `one-cli/ci-disable/v1`。Provider ID 和 JSON 字段不会翻译。
`--project` 为旧脚本保留，日常帮助优先展示位置参数。

## 错误

| 错误码 | 处理 |
|---|---|
| `CI_NOT_ENABLED` | 先运行 `one ci enable <project>`，再重试 sync |
| `CI_PROVIDER_UNKNOWN` | 使用 `error.context.available_providers` 中的 ID |
| `CI_RENDER_FAILED` | 检查错误 context 中的项目和工作流路径 |

稳定错误信封详见 [错误码](/zh/docs/error-codes/)。
