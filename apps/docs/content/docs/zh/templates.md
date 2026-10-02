---
title: 怎么选模板
description: 13 个内置模板按用途分组的决策树。30 秒判断到底该用哪个。
---

如果你正要给工作区加一个新项目，但不知道选哪个基础模板（API 选 Nest 还是 Go？前端选 CSR / SSR / SSG？），这一页给你一棵决策树和一张对比速查表。

运行 [`one templates`](/zh/docs/templates-cmd/) 查看当前安装版本的内置模板，下表说明各模板的用途与技术栈。

**适合读这页的人**：刚跑完 `one templates` 看到一串 ID 但不知道差异的人；评估栈选型的 Tech Lead；要给下属 / agent 写决策约束的人。

**读完会**：用 30 秒挑出对的基础模板，并使用 one add 添加项目。

## 30 秒判断口诀

```
要起一个后端 API ----------------→ nestjs-api / go-api
要起一个前端 Web 项目 -------------→ nextjs-app / react-spa / nextjs-site
要写一个跨项目复用的库 ----------→ ts-library / go-lib
要起一个文档站 -------------------→ fumadocs-docs
要起一个移动 app -----------------→ expo-mobile
要起一个桌面 app -----------------→ electron-app
```

不知道？问自己这一句：**用户怎么用你这个东西？** 浏览器打开 → Web；命令行调用 → API；npm install → Library；下载 .app/.dmg/.exe → Desktop；App Store → Mobile；阅读文字 → Docs。

## 完整对比表

| ID | 类别 | 关键词 | 一句话 | 详细 |
|---|---|---|---|---|
| `nestjs-api` | API | TypeScript, NestJS, REST | TS 团队默认 API 模板 | [源码](https://github.com/1cli-team/one-cli/tree/master/packages/templates/nestjs-api) |
| `go-api` | API | Go, Gin, GORM | 高吞吐 / 低内存 / 团队混语言 | - |
| `nextjs-app` | Web | Next.js, SSR, React | 通用 Web 应用 / C 端内容站首选 | [源码](https://github.com/1cli-team/one-cli/tree/master/packages/templates/nextjs-app) |
| `react-spa` | Web | Vite, React, SPA | 控制台 / 内部应用 / 无 SEO | [源码](https://github.com/1cli-team/one-cli/tree/master/packages/templates/react-spa) |
| `nextjs-site` | Web | Next.js, React, 静态导出 | 营销页 / 内容站 | [源码](https://github.com/1cli-team/one-cli/tree/master/packages/templates/nextjs-site) |
| `fumadocs-docs` | Docs | Fumadocs, Next.js, Base UI | 文档站 / 知识库 | [源码](https://github.com/1cli-team/one-cli/tree/master/packages/templates/fumadocs-docs) |
| `expo-mobile` | Mobile | Expo, React Native | iOS + Android 跨平台 | [源码](https://github.com/1cli-team/one-cli/tree/master/packages/templates/expo-mobile) |
| `electron-app` | Desktop | Electron, React, Vite | 桌面 app（macOS / Windows / Linux） | [源码](https://github.com/1cli-team/one-cli/tree/master/packages/templates/electron-app) |
| `ts-library` | Library | TS, 严格 semver | 跨项目复用的 TS 库 | - |
| `go-lib` | Library | Go, module, package layout | 跨项目复用的 Go module | - |
| `empty-app` | App | 无技术栈 | 从空应用目录开始 | - |
| `empty-service` | API | 无技术栈 | 从空服务目录开始 | - |
| `empty-library` | Library | 无技术栈 | 从空共享库目录开始 | - |

## 选好之后怎么加

这张表里的 `ID` 就是 `one add` 后面的第一个参数。

第一次不确定时，直接跑交互式：

```bash
one add
```

已经选好模板时：

```bash
one templates
one add nestjs-api --name api
```

`nestjs-api` 来自模板 ID，`api` 是你给这个项目起的名字。

## 推荐组合

### 全栈 SaaS（默认推荐）

```bash
one create my-saas
cd my-saas
one add nestjs-api     --name api
one add nextjs-app --name web
one add ts-library   --name shared
```

为什么：TS 全栈复用类型，`shared` 同时被 api 和 web 引用；Next.js SSR 走 SEO 也能跑后台。

### 高性能后端 + 静态营销页

```bash
one add go-api     --name api
one add nextjs-site --name marketing
one add react-spa --name console
```

为什么：Go API 顶住流量；Next.js 静态导出首页便于 SEO；React 控制台只给登录用户用，无 SEO 需求。

### 移动 + API

```bash
one add nestjs-api       --name api
one add expo-mobile --name app
one add ts-library     --name shared
```

`shared` 在 RN 端可以复用 API 的 DTO 类型。

## 还是不确定？

跑 `one templates -o json` 看每个模板的完整描述，或者直接 `one add` 进入交互式选择 —— 选择器里会带上类别和一句话提示。

或者直接选 [推荐组合](#推荐组合) 里的栈，先跑起来，跑不通再换。

## 静态网站与文档模板

`nextjs-site` 使用 Next.js、React、Tailwind CSS 和 shadcn Base UI，默认导出 `out/`。
`fumadocs-docs` 使用 Next.js + Fumadocs 的 Base UI 实现，内置中英文 MDX、侧边栏、
本页目录、代码高亮和浏览器静态搜索；同样部署 `out/`，无需 Node 服务。

旧 ID `astro-site` 和 `starlight-docs` 已下架。新建项目分别使用 `nextjs-site` 和
`fumadocs-docs`；旧 ID 会显示替代指引。已有项目的目录、依赖和任务保持原状。

## 模板依赖和 Electron 工作区

Node 模板不复制预生成的锁文件。`one dev`、`one build` 等任务共用依赖准备流程：
复用匹配的依赖，需要安装时允许生成或更新根锁文件。请审阅并提交锁文件变更。
需要严格校验锁文件的 CI，可显式执行 `one mise exec -- pnpm install --frozen-lockfile`。

`electron-app` 会创建三个顶层 One 项目。输入 `--name desktop` 时，目录分别为
`apps/desktop-renderer`、`services/desktop-main` 和 `packages/desktop-preload`，
npm 包名分别为 `desktop-renderer`、`desktop-main` 和 `desktop-preload`。
目录保留用户输入的项目名，npm 包名转换为 kebab-case。三个项目共用根目录的
pnpm workspace 和 lockfile，不再创建嵌套工作区。

清单通过 `[groups.desktop]` 记录三个成员。使用 `one run desktop:dev`、
`one run desktop:build`、`one run desktop:test` 和 `one run desktop:pack`
操作整组项目；重新运行 `one init mise` 后这些任务仍会保留。
每个成员仍可独立操作，例如 `one build -p desktop-main`。
主进程使用 Awilix 函数工厂，renderer 使用 React、Vite 和基于 Base UI 的 shadcn/ui。
打包时会将 renderer 和 preload 的构建产物复制到 main 的构建目录。

模板沿用根目录包管理器版本、registry 和镜像配置。已有安装脚本策略会保留；未配置时
使用内置模板默认规则。若显式禁用了 Electron 安装脚本，需要在根目录调整策略。
多个桌面应用同时开发时，为每组配置不同的 `ELECTRON_RENDERER_PORT`；
同组 main 和 renderer 使用相同端口值。

Electron 开发启动仅在系统标识为 Ubuntu（`ID=ubuntu`）时检查沙箱权限。
首次遇到 AppArmor 用户命名空间限制，会生成当前 Electron 专用的规则文件，
显示供管理员审阅、安装与加载的命令；配置后重新运行即可，脚本不会自行提权。
Electron 可执行文件路径变化后会重新检查。Linux 会保留已有显示变量（包括
SSH X11 转发和 Xvfb），缺失时自动识别当前用户可用的 Wayland / X11 桌面。
多个候选无法确定时提示显式选择；无桌面时提示使用图形终端、X11 转发或 Xvfb。
macOS 和 Windows 跳过这些检查。详细流程见生成项目的 README。

这些规则适用于新生成的项目，已有 Electron 项目不会自动改写目录或依赖配置。


## 在模板目录直接开发

内置模板使用正常源码文件，可以在 `packages/templates/<id>` 内直接运行和调试。
Go 使用本地 `go.work` 隔离仓库根模块；Electron 使用仅用于模板开发的
`pnpm-workspace.yaml`。这些开发配置不会复制到生成项目，生成后的项目仍由
One 工作区根目录管理。

在 One CLI 源码仓库中，例如：

```sh
cd packages/templates/go-api
go test ./...
go run ./cmd/server
```

Go API 默认使用内存 SQLite。需要 PostgreSQL 或其他运行配置时，按模板说明设置环境变量。

```sh
cd packages/templates/electron-app
pnpm install
pnpm run dev
```

Electron 会先构建 preload，再启动主进程和 Vite UI。运行环境仍需要满足 Electron
的图形界面和系统沙箱要求。仅构建可使用 `pnpm run build`。

单包 Node 模板（例如 React）开发时要隔离父工作区：

```sh
cd packages/templates/react-spa
pnpm --ignore-workspace install
pnpm --ignore-workspace run dev
```

模板开发使用其 `package.json` 声明的 pnpm 版本；生成后沿用目标工作区的版本。
本地依赖、锁文件、构建产物不会打包进 CLI。

## 修改模板的生成规则

默认按原始字节复制文件。只有需要参数化的模板才包含 `template.json`，目前支持：

| 配置 | 用途 |
| --- | --- |
| `schemaVersion: 1` | 声明描述文件版本 |
| `go.modulePrefix` | 生成 module 路径，并同步改写对应 Go import |
| `node.scope`、`node.sourceFiles` | 修改内部 Node 包名、依赖键、scripts 和指定源码中的 scope |
| `text` | 在明确列出的文件中替换示例文字 |
| `exclude` | 排除模板开发专用的文件或目录 |
| `projects` | 组合模板的成员，每个成员声明 `source`、名称 `suffix` 和 `category`（`frontend`、`backend` 或 `library`） |
| `sharedFiles` | 复制到组合模板每个成员的模板相对路径文件 |

`text` 的每条规则使用 `files`、`from`、`value`；`value` 仅支持
`projectName` 和 `projectNameKebabCase`。可选 `minMatches` 默认为 1，
每个指定文件都必须达到命中次数。路径是模板内的精确相对路径，不执行脚本或表达式。
未知字段、缺失文件、替换不足或重叠会在写入目标目录前报错。

Go 模板只维护一份正常的 `go.mod` 和必要的 `go.sum`。资源打包层临时把
`go.mod` 改名为 `_go.mod`，避开 Go 嵌入的子模块限制，生成时自动还原。
不要手动编辑 `packages/cli/internal/resources/bundled/` 中的生成资源。

修改后从仓库根目录运行：

```sh
mise run sync-bundled
mise run check
mise run check:templates
```

`check:templates` 会安装模板依赖，构建 Go/Electron 源码与生成的 Next.js 静态站点，检查全部
Node 模板的生成格式，并在临时工作区构建两个不同名称的 Electron 项目、两个 Go 项目与两种
Next.js 静态站点，同时验证中英文页面和文档静态搜索。
该检查需要网络及对应工具链；它不会修改开发者的全局语言偏好。
