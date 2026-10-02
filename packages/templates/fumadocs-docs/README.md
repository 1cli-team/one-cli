# Fumadocs 文档站 / Fumadocs documentation

Next.js 16 + React 19 + TypeScript 7 + Tailwind CSS 4 + Fumadocs Base UI。
English/Chinese documentation with local Markdown/MDX and static browser search.

## 创建与开发 / Create and develop

```sh
one add fumadocs-docs --name docs --yes
one dev -p docs
one build -p docs
```

项目直接位于 `apps/<用户项目名>`，共用工作区根目录的 pnpm 配置。
The project lives directly in `apps/<project-name>` and uses the workspace root pnpm configuration.

## 内容与语言 / Content and languages

在 `content/docs/en/` 和 `content/docs/zh/` 编写文档，使用相同文件名让语言切换打开对应页面。
Edit matching files in `content/docs/en/` and `content/docs/zh/`.
`meta.json` 控制侧边栏顺序，正文标题生成目录，`source.config.ts` 配置内容与代码高亮。
Use `meta.json` for navigation order and `source.config.ts` for content and syntax highlighting.

文档 UI 使用 `fumadocs-ui` 到 `@fumadocs/base-ui` 的 npm alias。
The `fumadocs-ui` npm alias installs `@fumadocs/base-ui`; custom components use shadcn `base-nova`.
搜索通过静态 `GET /api/search` 导出索引，并在浏览器中按当前语言搜索。
The static `GET /api/search` exports the index; search runs in the browser and filters by locale.

## 构建与部署 / Build and deploy

```sh
pnpm run check
pnpm run build
pnpm run preview
```

`next build` 将站点导出到 `out/`，`preview` / `start` 使用静态文件服务器。
`next build` exports the site to `out/`; `preview` / `start` serve those static files.
可部署到 Nginx、对象存储或其他静态托管平台。
Deploy to Nginx, object storage, or another static hosting provider.
需要运行时服务端功能时，选择 `nextjs-app` 或调整 Next.js 输出模式。
Choose `nextjs-app` or change the output mode when your site needs a runtime server.

## 模板源码开发 / Template source development

```sh
pnpm install
pnpm run dev
```

源码目录的 `pnpm-workspace.yaml` 隔离父工作区，生成项目时会排除它。
The source-only `pnpm-workspace.yaml` isolates the parent workspace and is excluded when generating projects.
生成的内容、依赖、锁文件和构建产物不包含在模板中。
Generated content, dependencies, lockfiles, and build outputs are excluded from the template.
