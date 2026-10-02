---
title: one.manifest.toml
description: Manifest v2 的完整结构：工作区身份、项目与 Infisical 绑定。
---

`one.manifest.toml` 是工作区配置文件。Manifest v2 使用 TOML，以表名登记项目。One CLI 只读取这个文件，不读取原来的 JSON 格式，也不提供迁移命令。

## 完整结构

```toml
version = 2

[workspace]
id = "my-workspace"
name = "My Workspace"

[env.infisical]
siteUrl = "https://app.infisical.com"
projectId = "your-project-id"
environments = ["dev", "staging", "prod"]

[projects.web]
path = "apps/web"
toolchain = "node"

[projects.api]
path = "services/api"
toolchain = "go"
```

新工作区只生成版本和身份信息。添加项目时增加对应的项目表，绑定 Infisical 后增加 `[env.infisical]`。实际生成的文件不带注释。你可以自行添加 TOML 注释；后续修改会保留注释、表顺序和未改动的内容。

模板只在创建项目时使用，不记录在 manifest 中。项目区块只包含 `path` 和 `toolchain`。

## 字段

| 字段 | 含义 |
|---|---|
| `version` | 必填，固定为 `2` |
| `workspace.id` | 工作区的稳定标识 |
| `workspace.name` | 工作区展示名称，也是创建远程存储时的初始名称 |
| `env.infisical.siteUrl` | 可选，Infisical 实例地址；默认 `https://app.infisical.com` |
| `env.infisical.projectId` | 配置绑定时必填，远程项目 ID |
| `env.infisical.environments` | 配置绑定时必填，远程环境 slug 列表；名称唯一且必须包含 `dev` |
| `projects.<name>` | 表名中的项目名称，工作区内唯一 |
| `projects.<name>.path` | 必填，规范化的工作区相对目录，不能与其他项目重复 |
| `projects.<name>.toolchain` | 必填，可选 `node`、`go` 或 `none` |

项目目录不能是绝对路径，也不能越出工作区。未知字段、不支持的版本或无效字段返回 `MANIFEST_INVALID`。TOML 语法错误会标明文件和位置。

## 项目组

组合模板可以将已注册的项目归为一组：

```toml
[groups.desktop]
projects = ["desktop-renderer", "desktop-main", "desktop-preload"]
```

`groups.<name>.projects` 必须是非空列表，成员必须是已注册的项目名且不能重复。
项目组名称不能与项目名称冲突。项目组生成 `desktop:dev`、`desktop:build`
等任务别名，自身没有目录或环境绑定；成员任务各自使用所属项目的环境变量。
删除或重命名成员时需同步更新项目组。`one init mise` 根据此记录重新生成别名。

## 环境变量约定

未配置 `[env.infisical]` 时，任务继承现有进程环境。Manifest 不保存变量值，也不登记变量名。

默认环境固定为 `dev`，不受 `environments` 中的排列顺序影响。通过 `--env staging` 或其他已声明 slug 指定环境。这里填写 Infisical 的环境标识，例如 `dev`、`staging`、`prod`；界面展示名可以是 Development、Staging、Production。把 slug 写入列表不会自动创建远程环境。

共享变量目录固定为 `/`。`path = "services/api"` 的项目依次合并 `/`、`/services`、`/services/api`，更具体的目录优先。并行任务分别接收所属项目的变量。目录和继承规则按约定执行，不提供项目级覆盖配置。

## 配置各归其位

- `mise.toml` 定义任务、依赖、工具版本和缓存；存在对应 mise 任务时，才可以使用 `one run dev` 或 `one dev` 任务快捷入口。
- `package.json` 保存包管理器信息；项目版本保留在各自原生文件中。
- Dashboard 从进程输出发现服务访问地址。
- stream 或 TUI 输出模式由 One CLI 个人偏好控制。

`one env bind` 显式保存 Infisical 绑定。`one env set` 要求已有绑定，将变量值保存到 Infisical；成功写入后可登记新的环境名，但不会把变量名写入 Manifest。Dashboard 发布绑定修改前展示实际 TOML，并通过文件 revision 拒绝过期草稿。项目设置展示约定，不再提供项目级环境覆盖开关。

具体命令参阅[环境变量](/zh/docs/env-vars/)，项目登记参阅[添加项目](/zh/docs/add/)。
