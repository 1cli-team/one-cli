# Next.js 静态网站 / Next.js static website

Next.js 16 + React 19 + TypeScript 7 + Tailwind CSS 4 + shadcn Base UI。
A static website with English/Chinese pages, themes, and Base UI Toast.

## 创建与开发 / Create and develop

```sh
one add nextjs-site --name website --yes
one dev -p website
one build -p website
```

项目直接位于 `apps/<用户项目名>`，共用工作区根目录的 pnpm 配置。
The project lives directly in `apps/<project-name>` and uses the workspace root pnpm configuration.

## 内容与语言 / Content and languages

页面位于 `src/app/[lang]/`，包含首页与关于页面；译文集中在 `src/lib/i18n.ts`。
Pages in `src/app/[lang]/` include home and about; translations live in `src/lib/i18n.ts`.
`src/styles/tokens.css` 提供设计令牌，组件使用 shadcn `base-nova` 与 Base UI Toast。
Design tokens live in `src/styles/tokens.css`; components use shadcn `base-nova` and Base UI Toast.
部署前设置 `NEXT_PUBLIC_SITE_URL`，供 SEO 元数据、sitemap 与 robots 使用。
Set `NEXT_PUBLIC_SITE_URL` before building to configure metadata, sitemap, and robots.

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
