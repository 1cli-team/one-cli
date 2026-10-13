# Contributing / 开发约定

Read [AGENTS.md](AGENTS.md) before editing and [RELEASING.md](RELEASING.md) before releasing. Public copy must support Chinese and English; stable command names, IDs and wire fields remain unchanged.

修改前阅读开发约定，发布前阅读发布流程。用户可见文案同时支持中英文，稳定的命令名、ID 和协议字段保持不变。

## Local workflow / 本地工作流

This repository is a One workspace. Discover tasks from configuration rather than a separately maintained command guide.

本仓库也是 One 工作区，任务以配置和命令发现结果为准。

```sh
one run --list -o json
one install
one dev
one dev -p website
one build -p website
one mise run --jobs 1 --force check
```

`one install` builds and installs the local CLI launcher; `one dev` runs Desktop. The website exports static files to `apps/website/out` and can be previewed with `one run preview -p website`. Node dependencies and the pnpm lockfile belong to the repository root.

`one install` 构建并安装本地 CLI，`one dev` 启动桌面版。官网产物在 `apps/website/out`，通过 `one run preview -p website` 预览。Node 依赖与 pnpm 锁文件由仓库根统一管理。

After changing project task definitions, run `one init mise` and review the generated configuration. `one mise run sync-bundled` regenerates embedded templates and skills; edit their source under `packages/templates` and `packages/agent-skills`, not generated copies. Keep the Fumadocs template available even though the One website no longer uses it.

项目任务变更后执行 `one init mise` 并审阅生成配置。内嵌模板和技能通过 `one mise run sync-bundled` 更新，应修改源目录而不是生成副本。One 官网不再使用 Fumadocs，但该项目模板仍保留。

The root check covers Go, React, Rust, static website, CLI help, agent references and release rules. Also run the affected project build. Use temporary workspace/config directories for checks that write preferences or create projects. Never print real credentials.

根检查覆盖 Go、React、Rust、静态官网、CLI 帮助、AI 指引与发布规则；同时运行受影响项目的构建。涉及偏好保存或项目创建的验证使用临时工作区和配置目录，不打印真实凭据。

Preserve the public `/install.sh` and `/install.ps1` endpoints when changing the website. Inspect their syntax and served content without executing a real installation.

官网改动必须保留两个安装脚本地址；检查脚本语法与实际返回内容，不通过真实安装来验证。
