---
title: one skills
description: 透传上游 Skills CLI 管理 agent skills，并说明各模板的默认开发 skills。
---

`one skills` 是 `npx skills@1.7.0` 的薄封装。在当前目录原样传递所有参数，保留交互输入、标准输出、错误输出和退出码。缺少 npx 时通过 mise 准备 Node，不读取 One/Infisical 凭据，也不会为工作区添加 Node 项目。

```bash
one skills --help
one skills find react
one skills add owner/repo --skill skill-name
one skills list
one skills update
one skills remove skill-name
```

所有子命令和参数遵循 [Skills CLI](https://github.com/vercel-labs/skills)。One 不增加 `sync` 子命令，也不重新解释 `--agent`、`--global`、`--yes` 或未来新增的上游参数。`one skills --help` 显示上游帮助，`one help skills` 显示封装说明。原始输出不包装成 One JSON。npx 会接受 npm 的首次工具下载提示，Skills CLI 自身的确认仍保留，除非用户传入 `--yes`。

## 自动安装开发 skills

`one create` 默认安装 `one-cli` 和 `find-skills`，分别提供 One 用法与 skill 查找指导。`one add` 在项目文件和工作区配置生成完成后，为所选模板补齐缺少的基础和技术栈 skills。Electron 的三个成员全部就绪后，只安装一次合并后的 skills。自动安装都在工作区根目录的 `.agents/skills` 中，供 Codex、Cursor 等兼容 agent 读取，不安装到用户全局目录，并且只显示 One 的简洁进度。

两个命令都可通过 `--skip-skills` 跳过安装，Dashboard 新建项目弹窗也提供同样的选项。默认安装明确选择 skill 名称和 universal agent，复制文件并非交互执行，不会安装来源仓库中的所有 skills。

```bash
one create demo --skip-skills
one add react-spa --name web --skip-skills
```

所有模板都有基础 skills `one-cli` 和 `find-skills`，Node 模板另有 `pnpm`：

| 模板                                          | 额外 skills                                                                                                                   |
| --------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `nestjs-api`                                  | `one-nestjs`、`nestjs-best-practices`                                                                                         |
| `go-api`、`go-lib`                            | `one-go`、`golang-patterns`、`golang-testing`                                                                                 |
| `nextjs-app`、`nextjs-site`                   | `one-web`、React 最佳实践、组合模式、`shadcn`；查阅项目已安装的 Next 文档                                                     |
| `fumadocs-docs`                               | 上述 Next/React skills，以及 `one-fumadocs`                                                                                   |
| `react-spa`                                   | `one-web`、React 最佳实践、组合模式、`shadcn`、`vite`                                                                         |
| `expo-mobile`                                 | `one-expo`、`expo-overview`、`expo-router`、`expo-data-fetching`、`expo-dev-client`、`expo-upgrade`、`expo-project-structure` |
| `ts-library`                                  | `tsdown`                                                                                                            |
| `electron-app`                                | `one-web`、React 最佳实践、组合模式、`shadcn`、`vite`、`tsdown`、`one-electron`                                     |
| `empty-app`、`empty-service`、`empty-library` | 仅基础 skills                                                                                                                 |

空模板不预设语言、框架、UI、数据库或项目内部架构，技术栈由用户自由选择。基础 skills 只指导 One 工作区目录与操作流程，不要求采用其他模板的技术栈。确定技术栈后，再按需通过 `one skills add` 安装匹配的开发 skills。默认生成的 `AGENTS.md` 也遵循这一规则，复杂脚本使用项目选择的语言，不强制引入 Node 或 TypeScript。

`one-cli` skill 提供外层及项目内部目录规范，以及技术栈适配说明。项目依赖和团队约定优先：生成的 Web 模板使用 Base UI，已有应用可能使用 Radix；Expo 使用原生样式和已有 Axios/SWR；Electron 使用 Awilix 工厂函数。安装上游 skill 不代表授权替换这些选择。

One 自行维护七个开发 skills：

| Skill          | 职责                                                                |
| -------------- | ------------------------------------------------------------------- |
| `one-cli`      | One 命令、工作区目录和项目发现；空模板不限定技术栈                  |
| `one-nestjs`   | NestJS 模块、Drizzle、配置注入、验证、日志与健康检查                |
| `one-go`       | Go 服务和库的目录、显式依赖、Gin/Gorm/Viper/Zap 与资源生命周期      |
| `one-expo`     | 原生样式、Expo Router、MMKV、开发客户端与 SWR 原生监听              |
| `one-web`      | Base UI、Axios/SWR/Zustand，React SPA、Next.js SSR 与静态导出的区别 |
| `one-electron` | 三进程项目、Awilix、preload/IPC、Ubuntu 引导与打包                  |
| `one-fumadocs` | MDX、多语言文档、导航与静态搜索                                     |

这些技能提供 One 模板的适配规则，上游 skills 提供框架通用最佳实践。`one-web` 共用于 Web 应用、Electron renderer 和 Fumadocs 的 Web 基础栈；文档内容和原生进程分别由专属技能指导。Agent 只读取当前任务需要的技能与引用，不要求加载全部技能正文。TS 库沿用上游工具技能；空模板仅安装基础技能，不自动安装任何技术栈适配技能。已有项目可以在工作区根目录手动安装对应 One 技能，自动安装只在添加项目时补齐缺少的名称。

默认来源包括 [Vercel](https://github.com/vercel-labs/agent-skills)、[Skills CLI](https://github.com/vercel-labs/skills)、[shadcn](https://github.com/shadcn-ui/ui)、[Antfu](https://github.com/antfu/skills)、[tsdown](https://github.com/rolldown/tsdown)、[Expo](https://github.com/expo/skills)、[ECC Go skills](https://github.com/affaan-m/ecc) 和 [NestJS 社区 skills](https://github.com/kadajett/agent-nestjs-skills)。One 的默认目录固定已审阅的来源提交；用户手动命令仍遵循上游，不限制来源。

## 文件、冲突和恢复

将已安装的 `.agents/skills` 文件和上游 `skills-lock.json` 一并提交。新工作区默认忽略根目录 `.one/`，安装过程不再生成 `.one/skill-sources` 或 `.agents/skill-sources` 中的重复来源。One 自己的 skill 源码仍在本仓库的 `packages/agent-skills/skills` 中维护，与可运行模板和业务 skills 分开。

正式发布的 CLI 从 `1cli-team/one-cli/packages/agent-skills` 安装 One 自己的 skills，固定到该版本的完整 Git 提交。锁文件记录 GitHub 来源，恢复时不依赖本地缓存。开发版和快照版则只将所选的内置 skills 写入最终 `.agents/skills` 路径，再由 Skills CLI 将 `./.agents/skills` 登记为本地来源，避免未发布的修改指向不存在的远程提交，也不保留重复文件。这些本地副本同样需要提交；开发版 skill 被删除后，从 Git 恢复，或再次通过 One 添加对应项目补齐，本地来源无法下载自身缺失的文件。

已有安装及指向 `.one/skill-sources` 的锁文件会在添加项目时保留。明确迁移前继续保留它们引用的来源；迁移时使用 `one skills add` 选择已发布的 One 来源和提交。重新安装前审阅团队修改，确认锁文件不再引用旧目录后再删除它，或在旧工作区中忽略 `.one/`。One 不会自动删除旧目录或改写来源记录。

Skills 按模板安装、按任务读取。Agent 可以先根据名称与描述选择相关 skill，启用时读取其 `SKILL.md`，需要细节时再打开引用资料或脚本。生成的 `AGENTS.md` 会说明这一流程，不要求启动时阅读全部正文；具体加载行为由 agent 实现。

自动安装只补齐缺少的名称，保留已有 skill 目录、团队修改和不同来源。来源冲突、无效或不受支持的锁文件会产生提示，不覆盖已有内容。用户明确执行 `one skills` 时仍遵循上游的更新和覆盖行为。锁文件格式由上游管理，One 不维护第二份 skills 锁文件。

自动安装总超时为五分钟。网络、下载和工具运行失败不会撤销工作区或项目，创建操作仍成功，通过 `warnings` 返回诊断信息和在工作区根目录重试的命令。自动安装只显示 One 的本地化进度，上游 Logo、通用提示和安装界面会被收起；失败时保留具体原因，结构化创建结果仍可解析。手动执行 `one skills` 继续保留上游完整输出。默认来源提交保留在锁文件中，版本更新需要主动选择来源或 ref，添加项目不会自动升级已有 skills。

## 业务 skills 由用户手动安装

登录注册、支付、权限、上传等业务 skills 在独立 GitHub 仓库维护，不进入 One CLI 的源码、模板或发布资源。`one create` 和 `one add` 都不会自动安装。

业务仓库发布后，用户可以在工作区根目录选择需要的 skill，安装到项目中，不添加 `--global`：

```bash
one skills add owner/business-skills --skill one-auth
```

这里的仓库及名称是占位示例，不表示已经内置或发布。安装只是让 agent 获得指导，不会直接实现登录或创建后端服务；业务功能需要用户另外提出开发需求。

## 已有工作区

已有项目先检查实际依赖，再通过 `one skills add` 主动选择匹配的 skills，不根据旧模板名称猜测技术栈。例如 React/Vite 项目可以安装：

```bash
one skills add vercel-labs/agent-skills --skill vercel-react-best-practices vercel-composition-patterns
one skills add antfu/skills --skill vite
one skills add shadcn-ui/ui --skill shadcn
```

在 One 源码仓库中，可以从 `./packages/agent-skills` 安装自行维护的 skills。修改维护源码不会自动覆盖已安装副本，需要更新副本时主动重新安装。
