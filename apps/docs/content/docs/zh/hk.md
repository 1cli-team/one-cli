---
title: one hk
description: 使用 hk 统一工作区检查、显式修复和 Git 提交检查。
---

`one hk` 通过 One 内置的 mise 运行固定版本的 hk。Go、JS/TS 和混合工作区使用同一套入口，不需要为了 Git hooks 安装 Husky 或 commitlint。

## 日常使用

```bash
one hk check                 # 检查变更文件
one hk check --all           # 检查整个工作区，也可用于 CI
one hk check --plan --json   # 查看检查计划，不执行检查
one hk fix                   # 显式修复，默认不暂存修改
one hk validate              # 校验 hk 配置
```

One 从工作区根目录运行 hk；指定文件时使用相对工作区根目录的路径，例如 `one hk check apps/web/src/App.tsx`。hk 参数、标准输出和退出码直接透传。

新工作区自动安装本地 `pre-commit` 和 `commit-msg` 启动器，继续正常使用 `git commit`。克隆已有工作区后，运行一次：

```bash
one configure hooks
```

Git 启动器记录本次 One 可执行文件的位置，并通过它运行内置 mise；不依赖终端的 mise 激活状态。移动或更换 One 安装位置后，可以重新执行 `one configure hooks`。

## 默认检查

| 阶段 | 行为 |
|---|---|
| 提交前 | 检查暂存内容；临时保存并恢复未暂存修改，不自动修复或暂存文件 |
| 提交信息 | hk 内置的 Conventional Commits 基础检查，支持中文描述 |
| Go 项目 | 使用该项目的 Go 版本检查 `gofmt` 格式；修复时调用 `gofmt -w` |
| JS/TS 项目 | 使用项目已经声明的 oxlint / oxfmt，按文件筛选并读取项目配置 |
| 其他 Node 工具 | 存在时复用 `lint`、`format:check` 和相应的 `:fix` 脚本；脚本可能检查整个项目 |

提交检查不调用 Go 模板中包含 `go mod tidy` 的 `task check`。构建、类型检查和测试仍可通过项目原有命令执行，也可以自行加入 hk 配置。

工具版本由 mise 提供，首次需要时下载并缓存。mise 本身已经内置在 One 中；hk 是单独安装的工作区工具。JS 检查使用项目依赖，需要先安装依赖；`one dev <project>` 会准备开发依赖，也可运行 `one mise exec -- pnpm install`。

## 配置与项目增量更新

- `hk.pkl`：用户维护的根配置，默认继承 `.config/one/hk.pkl`。
- `.config/one/hk.pkl`：One 生成的默认检查，随 `one add` 更新。
- `.mise/conf.d/one.toml`：声明固定的 hk 版本。
- Git hooks：安装到当前仓库的 Git hooks 目录，不写用户全局 Git 配置。

在 `hk.pkl` 中增加检查或覆盖设置；One 不会覆盖这个文件。不要直接修改生成文件，它带有内容校验，修改后再次生成会报告冲突。新增自定义 hook 事件时，需要自行接入对应 Git 启动器；One 默认安装两个提交检查事件。

## 迁移旧工作区

```bash
one configure hooks --dry-run -o json
one configure hooks
one mise exec -- pnpm install
```

预览会列出配置、Git 启动器以及将移除的默认文件。只有 One 以前生成的默认 Husky / commitlint 配置会自动迁移；自定义规则、额外的 Husky hook、已有的其他 hook 目录会报告 `HOOKS_CONFIG_CONFLICT`，保留原文件供手动整合。

迁移移除根 `package.json` 中的 Husky / commitlint 依赖以及默认 `prepare: husky`，保留已有的 Changesets 等其他配置。最后使用实际包管理器执行安装，以更新锁文件；命令本身不会猜测或重写包管理器锁文件。

hk 的基础提交检查不等价于 commitlint 的全部规则。例如自定义的长度、大小写和插件规则，需要显式补充到 `hk.pkl` 中。

## CI

准备 One 和项目依赖后运行 `one hk check --all`。该命令复用本地检查配置，不安装 Git hooks，也不自动修复代码；现有构建和测试步骤继续保留。
