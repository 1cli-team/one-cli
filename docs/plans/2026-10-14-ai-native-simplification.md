# One CLI AI Native 精简方案

**Goal:** 删除官网文档和教程，以 One CLI 的 Next.js 静态网站模板承接现有首页，删除无实际用途的 kernel 模块，并下线内置 `one serve` 命令。

**Architecture:** 保留 CLI、One Core、Desktop API 和 Desktop 的现有边界。官网独立为 `apps/website`，构建后只部署静态文件；AI 操作入口继续使用真实 CLI 帮助、结构化输出和随项目提供的 skills / AGENTS。

**Tech Stack:** One CLI `nextjs-site` 模板、Next.js App Router 静态导出、React、TypeScript、Tailwind CSS、现有 Three.js 首页动画、Go 标准库 context。

**状态:** 2026-10-14 已在本地实施并验证。基于 `06aaacd`，所有变更留在工作区；未提交、未推送、未部署。下文保留批准时的范围与验收要求，末尾记录实际结果。

## 已确认范围

- 删除 One 官网的中英文文档和教程，以及对应路由、布局、内容处理和依赖。
- 官网不再使用 Fumadocs；CLI 对外提供的 `fumadocs-docs` 模板及 `one-fumadocs` skill 保留。
- 首页布局、样式、动画、响应式行为保持一致，只替换指向已删除页面或功能的入口和文案。
- “删除文档”以公开产品文档、教程为范围。保留运行所需的 CLI help、协议定义、AGENTS、skills，以及维护仓库必需的发布约定和最小开发说明；不把教程整套搬进 README。
- 本方案不新增 AI 命令、MCP 服务或另一套手工维护的命令参考。

## 实施前证据

| 项目 | 当前情况 | 结论 |
| --- | --- | --- |
| 官网 | `apps/docs` 使用 Next.js、Fumadocs、MDX；当前配置不是静态导出模式 | 需要移除文档依赖并调整输出和部署配置 |
| 静态模板 | `packages/templates/nextjs-site/next.config.mjs` 已配置 `output: "export"`、目录式 URL、关闭运行时图片优化 | 直接通过 One 创建新官网，不采用 `nextjs-app` 的 standalone 输出 |
| 首页 | `src/app/(home)` 的主体是独立 React 组件；Three.js 在浏览器 effect 中加载 | 可迁移现有实现，无需重新设计；须剥离全局 Fumadocs 样式和 Provider 的影响 |
| kernel | 业务代码只有 `one-core/pkg/application/execution/scope.go` 直接引用 `kernel.Context`；未找到业务清理注册 | 用标准 context 替代后可删除整个模块，无需搬迁清理栈 |
| backend | `one-core/pkg/core/backend/builtin.go` 仍注册 Infisical，bootstrap 仍使用对应 loader | 不把 kernel 精简扩大为删除当前环境变量和凭据能力 |
| serve | Cobra 命令只返回迁移提示，已不启动 HTTP 服务 | 删除命令注册和残留引用；Desktop API 独立保留 |
| AI 指引 | 生成的双语 AGENTS 仍提示 `one serve`；`public/llms.txt` 仍列 configure、deploy、container 等旧命令 | 必须一起清理，避免 AI 持续调用已下线能力 |

## 目标结构

```text
apps/
  website/                 # Next.js 静态官网，产物 out/
  desktop/                 # React + Tauri
services/
  desktop-api/             # one-api HTTP 服务
packages/
  cli/                     # one 命令行
  one-core/                # 共享业务与平台能力
  templates/               # 保留所有现有模板，包括 fumadocs-docs
  agent-skills/            # 保留 AI 工作指引
```

删除 `apps/docs` 和 `packages/kernel`，同步更新工作区项目登记。`docs/plans` 属于仓库设计记录，不属于待下线的官网路由。

## 任务 1：固定首页迁移基准

**涉及文件：** `apps/docs/src/app/(home)/*`、`apps/docs/src/app/global.css`、`apps/docs/src/components/*`、`apps/docs/public/*`。

1. 实施前核对当前线上首页与仓库版本是否一致。线上如果与当前代码不同，先记录差异，不能宣称直接迁移代码就与线上完全相同。
2. 留存 `/`、`/zh/`、`/en/` 的桌面和窄屏截图，记录布局、字体、颜色、间距、按钮、语言菜单、滚动定位和安装命令复制行为。
3. 记录 Three.js 初始场景、交互选中状态、窗口缩放、减少动画偏好和 WebGL 不可用时的降级表现。截图比较使用同一浏览器、视口和动画阶段。
4. 保存安装脚本和品牌资源的内容校验值，用于迁移后确认未误改文件。

**验收：** 有可以对比的视觉和交互基准；这一步不重新设计首页。

## 任务 2：用 One 的 Next.js 模板创建静态官网

**新增：** `apps/website`，通过 One CLI 生成，不手写仿制模板目录。

**修改：** `one.manifest.toml`、根 `package.json`、`mise.toml`、`pnpm-lock.yaml`；必要时同步根工作区配置。

1. 实施时重新执行 `one templates -o json` 核对 `nextjs-site`，记录所用 CLI / 模板来源版本。
2. 根目录执行 `one add nextjs-site --name website --yes --skip-skills`。仓库已有相关技能，避免重复安装；此命令仅在实施阶段执行。
3. 沿用模板的静态导出和检查工具配置，删除模板示例页面、默认导航和默认主题控件，把现有首页组件及资源迁入，避免混入模板演示 UI。
4. 保留当前首页所需的 Three.js、Base UI 和页面切换实现；不为了减依赖改变视觉和动画行为。
5. 保留 `/` 的中文首页、`/zh/` 和 `/en/`；显式生成语言路由，禁止未枚举语言的动态生成。HTML lang、canonical、alternate、robots、sitemap 一起调整。
6. 从当前 global.css 提取首页实际使用的设计变量、语言菜单和动画样式，检查移除 Fumadocs preset 后的字体、字号、边框与基础元素样式。
7. 保留 `/install.sh`、`/install.ps1`、品牌图标和现有域名。迁移脚本文件不改安装逻辑，避免影响现有安装和升级入口。
8. 同步根任务，使 `one dev -p website`、`one build -p website`、`one run check -p website` 可用；保持根 `one dev` 的 Desktop 行为。
9. 工作区依赖和锁文件仍由仓库根管理，生成项目后检查没有新增子项目锁文件或违反仓库规则的 packageManager 声明。通过 `one init mise` 同步受管任务，再审阅变更。

**验收：** `apps/website/out` 包含三种首页入口和静态资源，使用普通静态服务器即可运行，不需要 `next start`、Node 运行时接口或 Go 服务。

静态导出由 Next.js 官方支持，构建产物包含 HTML/CSS/JS；原有客户端动画仍可运行。`next.config` 的运行时 redirects、默认图片优化、请求时 cookies 等不能直接带入静态版本。[官方静态导出说明](https://nextjs.org/docs/app/guides/static-exports)

## 任务 3：删除官网文档体系并修正入口

**删除：** `apps/docs/content/docs`、`apps/docs/content/tutorials`、`src/app/[lang]/docs`、`src/app/[lang]/tutorials`、旧 `src/app/docs`、`source.config.ts`、`src/lib/source.ts`、MDX 专用组件及生成配置；迁移完成后删除整个 `apps/docs`。

**清理：** Fumadocs/MDX 的官网依赖、postinstall、文档专用 CSS、侧栏、目录、文档 SEO 和导航；只移除确认不再被首页使用的依赖。历史设计源只保留仍有价值的首页资产，不继续维护文档页面设计。

入口调整采用以下映射，保留原组件位置与样式：

| 原内容 | 替换方案 |
| --- | --- |
| 顶部“教程 / Tutorials” | “工作流 / Workflow”，定位现有工作流区域 |
| 顶部“文档 / Docs” | “下载 / Downloads”，进入统一 GitHub Release |
| 安装按钮 | 定位现有安装命令区；Desktop 下载连接到统一 Release |
| 首页工作流中的 `one serve` | 在同一位置介绍 One Desktop，操作指向下载，不编造新 CLI 命令 |
| 各功能的文档详情链接 | 对应现有页面锚点、GitHub 模板源或 AI skill 源；不指向已删除页面 |
| Footer 文档 / 教程栏目 | 复用现有网格，改为产品入口与 AI 资源入口 |
| “基于 Fumadocs 构建” | 改为实际使用的 Next.js 与 One CLI |

三维场景中的 Fumadocs 模板模块继续保留，因为该模板本身没有下线。两种语言同时完成调整。

`llms.txt` 改为很短的 AI 发现入口，链接权威的 skills / AGENTS 源，并提示使用当前 CLI 的 help、templates 和任务发现命令。删除手写的长命令清单及全部旧文档链接，避免重新形成第二套教程。

**维护工具调整：**

- 删除只生成官网页面的 `packages/cli/tools/gen-error-codes` 及根 / CLI Taskfile 的对应任务。保留运行时错误码、JSON 错误结构和修复建议。
- 删除 `packages/cli/tools/repo-checks/main.go` 的官网 frontmatter 分支。
- 将 `verify-docs` 拆成实际仍需要的 CLI help / AI 指引引用校验；保留 `tools/verify-help`。
- 调整 `tools/verify-cli-references` 的扫描目标，去掉已删除内容目录，覆盖实际维护的 README、模板指引、双语 AGENTS 源及 agent skills。任务简称应从真实任务定义确认，不再依赖已删除教程来声明有效命令。
- 精简 README / CONTRIBUTING 的产品教程和失效链接，保留必要的仓库说明；更新根 AGENTS 中的项目路径和检查约定。
- `packages/templates/fumadocs-docs`、模板 registry、嵌入模板、`one-fumadocs` skill 均保留。不能按 `fumadocs` 关键字全仓删除。

**部署：** 沿用当前仓库配置所指向的 Vercel。实施前读取线上 Project Root Directory 和构建配置，确定根 `vercel.json` 与项目级配置的实际生效位置；随后调整项目目录与静态输出路径，避免保留互相矛盾的配置。

旧 `/docs/*`、`/tutorials/*` 及中英版本路径，在托管层重定向到对应语言首页；保留裸域名到 www 的规则，核对不会产生循环。不要为已下线内容保留 Next 动态路由。安装脚本保持正确的正文、Content-Type 与缓存策略。部署配置修改先预览验证，正式部署另行授权。

**验收：** 站点产物没有教程、文档页面或 MDX 搜索资源；首页链接可用；模板目录仍能发现并生成 Fumadocs 项目。

## 任务 4：移除 kernel，收拢到标准 context

**修改：**

- `packages/one-core/pkg/application/execution/scope.go`
- `packages/cli/internal/bootstrap/cli/root.go`
- `packages/cli/go.mod`、`packages/one-core/go.mod`、`services/desktop-api/go.mod` 及必要的 sum 文件
- `go.work`、`one.manifest.toml`、`mise.toml`
- `packages/cli/tools/repo-checks/main.go`、`scripts/release.mts`

**删除：** `packages/kernel` 整个项目，包括其 Taskfile 和仅用于该模块的说明。

1. 再次核对仓库中没有新增 `Lifecycle.Defer` 调用或 kernel 的其他使用者。
2. `execution.Scope` 直接持有 `context.Context`；构造函数和零值读取仍兼容 nil，必要时返回 `context.Background()`。
3. 保留 `Scope.Derive` 的值拷贝行为及工作区、项目、环境等状态；保留来自请求的取消信号、deadline 与 context values。
4. 删除无实际清理工作的 `Scope.Close` 和 CLI 根入口的对应 defer。
5. 删除 kernel 的模块依赖、replace、工作区登记、构建/格式化/tidy 任务、发布脚本中的模块下载项。
6. 整理三个剩余 Go 模块的依赖并检查生成差异，确认 CLI 和 one-api 都可以独立构建。

**边界：** 不新建等价 runtime/lifecycle 包，不把无人使用的清理栈搬到 one-core；Desktop 的进程退出逻辑和现有子进程管理继续保留。Infisical、backend 描述与环境变量流程仍有消费者，本次不扩大到重构这些能力。

**验收：** 活跃 Go 代码、模块和任务配置不再依赖 `packages/kernel`；CLI / Desktop API 的 context 传播和退出行为保持有效。

## 任务 5：下线内置 one serve

**删除：** `packages/cli/internal/transport/cobra/serve/cmd.go` 及空目录。

**修改：** CLI 根命令注册、One Core 双语 locale、`modules/creation/templates/AGENTS.*.md`、官网和仓库当前说明、相关错误提示与注释。

1. 删除 Cobra 的 serve 注册、迁移提示和专属翻译键，不继续保留过渡命令。
2. 将 `create.tip`、`create.registry_warning` 和新项目 AGENTS 中的操作指引改为 One Desktop 的实际入口；不要继续承诺运行 serve 会重新登记工作区。
3. 修正 Desktop API 中“one serve 在工作区外启动”的过时错误，沿用中英文机制。
4. 区分命令与协议：`SERVE_BIND_FORBIDDEN`、`SERVE_PAYLOAD_INVALID`、`SERVE_MANIFEST_CONFLICT` 仍被 Desktop API / 前端使用，保留这些稳定值并修正过时恢复建议；仅清理确认没有消费者的旧服务专用项。
5. 保留 `one run serve` 执行用户自定义任务的能力。现有通用任务简称机制会把未知的 `one <task>` 转交给 `one run <task>`，因此删除内置命令后 `one serve` 也可能成为用户自定义任务简称；它不会再启动旧 Dashboard。不为 serve 单独增加黑名单。
6. 检查 AI skill 源、双语 AGENTS 模板及所有生成/嵌入流程，防止下一次构建把旧指引带回。

**验收：** `one help --all` 和命令树不再出现内置 serve；没有当前产品指引要求运行旧 Dashboard；Desktop 仍可通过 one-api 使用 HTTP，用户自己的 serve 任务仍可运行。

## 实施顺序与验证

顺序：**记录首页基准 → 生成并迁移静态官网 → 删除旧文档体系 → 精简 kernel → 移除内置 serve → 全量检查 → 交付审阅。** 首页旧命令替换与 serve 下线应在同一批交付中完成。

任务入口在修改后重新通过 One 发现，按以下范围验证，不在规划阶段运行构建或改变用户环境：

```sh
one run --list -o json
one run check -p website
one build -p website
one run preview -p website
one mise run --jobs 1 --force check
one mise run build-all
one build -p desktop-api
git diff --check
```

- 将 website 的质量检查接入根 check，保留 CLI 帮助、Go、Desktop UI、Rust 和已有发布规则检查。
- 对静态产物检查三种首页入口、404、语言切换、品牌资源、安装脚本和 sitemap；在与基准相同条件下比较完整页面和交互。文案替换区域单独审阅，其余部分不接受无解释的视觉变化。
- 校验安装脚本迁移前后内容一致；不要为了验证下载安装脚本而执行真实安装。
- 使用构建出的候选 CLI 和临时工作区/配置核对 help、templates、任务发现与 serve 任务行为，不能拿旧版全局 `one` 的结果证明命令已删除。
- 验证两种 CLI 语言、两种官网语言；保留 `fumadocs-docs` 的模板生成能力。凭据验证不使用或打印真实密钥。
- 确认 Go 模块和发布脚本没有 kernel 路径；现有统一 Release 的版本、附件名、签名和公证规则保持不变。
- 本地静态预览不能证明 Vercel 的重定向已生效；部署预览和正式线上验证分别记录，未运行的项目标记为未验证。

## 交付边界

本轮已获准完成四项实现；提交、推送、生产部署和正式发布均须按后续明确指令执行。本轮未进行这些操作。


## 实施结果与验证记录（2026-10-14）

- 用 One CLI 0.1.21 执行 `one add nextjs-site --name website --yes --skip-skills`，生成 `apps/website` 后迁移现有首页。保留模板静态导出及检查工具，依赖仍由根 pnpm workspace 管理。
- 删除 `apps/docs` 的文档、教程、MDX、Fumadocs 运行时与相关生成工具；设计源迁至 `design/website.pen` 归档。README、CONTRIBUTING 和 ARCHITECTURE 精简为维护者与 AI 所需的事实。
- 保留首页布局、配色、字体及三维动画；双语替换失效入口。四个动画/工作流组件去除格式位置和 JSX 空白差异后语法树一致。
- 语言根布局在切换时会重建原 React 过渡组件，实际复现过渡超时。因此语言链接改用普通同源导航，并通过原生 `@view-transition` 保留原 280ms/380ms 淡入淡出样式；移除 `next-view-transitions` 依赖。支持该 API 的浏览器运行过渡，不支持时正常导航；减少动画的 CSS 规则保留。[MDN：跨文档页面过渡](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/@view-transition)。
- 删除 kernel 模块，Scope 使用标准 context；删除内置 serve 注册及迁移提示。Desktop API、Infisical 和仍被消费的协议错误码保留。
- CLI 的 Fumadocs 模板与品牌、安装脚本合计 45 个文件内容哈希与迁移前一致。

已通过：

| 验证 | 结果 |
| --- | --- |
| `one mise run --jobs 1 --force check` | 最终通过：三个 Go 模块、Go 格式、27 份 AI/维护指引、15 个公开命令、Desktop React/Rust、Website TypeScript/lint/format、现有 21 项发布规则测试 |
| `one build -p website` | 静态导出成功；含 `/`、`/zh/`、`/en/`、404、robots、sitemap，无文档教程路由 |
| `one mise run build-all` | CLI 五个平台交叉构建通过；随后补充文案后再次通过本机 CLI 与 Desktop API 构建 |
| 候选 CLI + 临时 HOME/XDG | 中英文 help 均不含内置 serve；create 帮助和生成 AGENTS 使用 One Desktop；发现并实际生成 fumadocs-docs 项目成功 |
| 候选 CLI + 临时工作区 | 任务发现、`one run serve`、`one serve` 都成功执行自定义 serve 任务 |
| Scope 临时冒烟 | 零值/nil 安全、取消、deadline、context value、派生字段、父 Scope 不被修改均通过 |
| 静态产物 | 三个入口的 HTML lang、内部锚点及资源有效；安装脚本与源文件一致，install.sh 语法检查通过 |
| 浏览器 | 中英双向语言切换、平台切换、复制成功提示、工作流/桌面锚点验证通过；修复后新页面无控制台警告或错误 |
| 视觉 | 对比中英文 1440px 桌面与 390px 窄屏。字体、色彩及主要布局一致；窄屏测量一致，桌面局部高度差来自已批准的替换文案换行 |
| `git diff --check` | 通过 |

验证边界：

- 视觉基准来自迁移前线上页面与保留的本地源码；不是逐帧动画像素相等证明。
- 保留了原三维动画的减少动画偏好和 WebGL 失败处理；未分别模拟所有浏览器/硬件降级组合。复制验证覆盖页面成功反馈，未独立读回系统剪贴板内容。
- 未执行真实 CLI 安装、未重新打包签名/公证 Desktop、未发布版本。
- Vercel 根目录远端设置未读取或更改。仓库根与应用根配置已同步，旧文档/教程重定向与安装脚本响应头已配置；上线前须核对实际 Project Root Directory，线上行为需获准部署后验证。
- 临时构建日志、截图和隔离工作区用于本地验证，不加入仓库。未新增专门测试文件。
