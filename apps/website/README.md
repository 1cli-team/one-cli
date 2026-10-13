# One website / One 官网

Generated with One CLI 0.1.21: `one add nextjs-site --name website --yes --skip-skills`. The original homepage components and Three.js animation are retained; the site exports static HTML/CSS/JS without a documentation runtime.

通过 One CLI 的 Next.js 静态网站模板生成，沿用原首页组件和 Three.js 动画，不包含文档与教程站点。

From the workspace root / 在仓库根目录：

```sh
one dev -p website
one run check -p website
one build -p website
one run preview -p website
```

Output: `out/`. `/` and `/zh/` are Chinese; `/en/` is English. Keep `/install.sh`, `/install.ps1`, branding, and bilingual metadata available. Dependencies use the root pnpm workspace and lockfile.

产物为 `out/`，保留中文与英文首页、安装脚本、品牌资源与双语元数据。依赖和锁文件使用根工作区配置。

Vercel configuration is provided for either repository-root or application-root builds. The root directory and output path must match the selected Vercel project before deployment. No deployment is performed by local builds.

Vercel 配置分别支持仓库根与应用根构建；部署时需确认项目的 Root Directory 和产物路径匹配。本地构建不部署站点。
