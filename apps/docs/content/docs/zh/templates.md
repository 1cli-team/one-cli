---
title: 怎么选模板
description: 10 个基础模板按用途分组的决策树。30 秒判断到底该用哪个。
---

如果你正要给工作区加一个新项目，但不知道选哪个基础模板（API 选 Nest 还是 Go？前端选 CSR / SSR / SSG？），这一页给你一棵决策树和一张对比速查表。

如果你想直接从完整 workspace 组合开始，而不是逐个挑 `one add` 模板，去看 [模板示例](/zh/templates/)：那里是移动端、桌面端、Web、C 端、后台、文档站等完整 starter，可以直接复制 `one create --preset ...`。

**适合读这页的人**：刚跑完 `one templates` 看到一串 ID 但不知道差异的人；评估栈选型的 Tech Lead；要给下属 / agent 写决策约束的人。

**读完会**：用 30 秒挑出对的基础模板；如果想少做选择，也知道什么时候该直接去用完整示例。

## 30 秒判断口诀

```
要起一个后端 API ----------------→ nestjs-api / go-api
要起一个前端 Web 项目 -------------→ nextjs-app / react-spa / astro-site
要写一个跨项目复用的库 ----------→ ts-library / go-lib
要起一个文档站 -------------------→ starlight-docs
要起一个移动 app -----------------→ expo-mobile
要起一个桌面 app -----------------→ electron-app
```

不知道？问自己这一句：**用户怎么用你这个东西？** 浏览器打开 → Web；命令行调用 → API；npm install → Library；下载 .app/.dmg/.exe → Desktop；App Store → Mobile；阅读文字 → Docs。

## 完整对比表

| ID | 类别 | 关键词 | 一句话 | 详细 |
|---|---|---|---|---|
| `nestjs-api` | API | TypeScript, NestJS, REST | TS 团队默认 API 模板 | [移动 / 营销 / 后台示例](/zh/templates/) |
| `go-api` | API | Go, Gin, GORM | 高吞吐 / 低内存 / 团队混语言 | - |
| `nextjs-app` | Web | Next.js, SSR, React | 通用 Web 应用 / C 端内容站首选 | [C 端示例](/zh/templates/consumer-starter/) |
| `react-spa` | Web | Vite, React, SPA | 控制台 / 内部应用 / 无 SEO | [后台示例](/zh/templates/admin-starter/) |
| `astro-site` | Web | Astro, 静态优先 | 营销页 / 内容站 | [营销示例](/zh/templates/landing-starter/) |
| `starlight-docs` | Docs | Starlight, Astro | 文档站 / 知识库 | [文档站示例](/zh/templates/docs-starter/) |
| `expo-mobile` | Mobile | Expo, React Native | iOS + Android 跨平台 | [移动端示例](/zh/templates/mobile-starter/) |
| `electron-app` | Desktop | Electron, React, Vite | 桌面 app（macOS / Windows / Linux） | [桌面示例](/zh/templates/desktop-starter/) |
| `ts-library` | Library | TS, 严格 semver | 跨项目复用的 TS 库 | - |
| `go-lib` | Library | Go, module, package layout | 跨项目复用的 Go module | - |

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
one add astro-site --name marketing
one add react-spa --name console
```

为什么：Go API 顶住流量；Astro 静态化首页便于 SEO；React 控制台只给登录用户用，无 SEO 需求。

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

## 模板依赖和 Electron 工作区

Node 模板不复制预生成的锁文件。`one dev` 会按需生成或更新仓库根锁文件，
请将它提交 Git；`one build` 对已有锁文件执行严格校验。

`electron-app` 需要 pnpm 工作区。它仍是一个 One 项目，内部的主进程、UI 和 preload
包会自动加入根 `pnpm-workspace.yaml`，共享根锁文件。包名以项目名作为 scope，
例如 `@desktop/electron`、`@desktop/ui`、`@desktop/preload`，可在同一仓库中添加多个桌面应用。

模板沿用根目录包管理器版本、registry 和镜像配置。已有安装脚本策略会保留；未配置时
使用内置模板默认规则。若显式禁用了 Electron 安装脚本，需要在根目录调整策略。
多个桌面应用同时开发时，通过各项目环境中的 `ELECTRON_RENDERER_PORT` 配置不同端口。

这些规则适用于新生成的项目，已有 Electron 项目不会自动改写目录或依赖配置。
