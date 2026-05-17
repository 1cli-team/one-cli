# Web SSG Astro Template

基于 Astro 的静态站点模板，适合官网、营销页和纯静态内容站点。模板保留了设计令牌、统一主题样式和轻量请求工具，适合作为 `one-cli` 的前端 SSG 起点。

## 技术栈

- Astro 6
- TypeScript 5
- Tailwind CSS v4
- `@astrojs/check`
- oxlint + oxfmt

## 目录结构

```text
public/                   # 静态资源
src/
├── api/                  # 请求 key 与纯函数 API
├── assets/               # 模板图片资源
├── components/           # Astro 组件
├── layouts/              # 页面布局
├── pages/                # 文件路由
├── styles/               # 样式入口与 design token
└── utils/                # API / toast 等工具
```

## 快速开始

```bash
pnpm install
pnpm dev
```

默认开发地址：`http://localhost:4321`

## 常用命令

| 命令              | 说明                                 |
| ----------------- | ------------------------------------ |
| `pnpm dev`        | 启动开发服务器                       |
| `pnpm build`      | 先执行 `astro check`，再构建生产产物 |
| `pnpm preview`    | 预览构建结果                         |
| `pnpm astro`      | 直接调用 Astro CLI                   |
| `pnpm lint`       | 执行 oxlint                          |
| `pnpm lint:fix`   | 自动修复 lint 问题                   |
| `pnpm format`     | 检查格式                             |
| `pnpm format:fix` | 自动格式化                           |
| `pnpm check`      | 执行 lint + format                   |
| `pnpm check:fix`  | 执行 lint:fix + format:fix           |

## 请求层约定

模板示例里的网络请求也统一采用 “`key + pure function`” 结构：

```ts
export const demoPostKey = "https://jsonplaceholder.typicode.com/posts/1";

export function getDemoPost() {
  return api.get(demoPostKey);
}
```

当前示例位于：

- `src/api/demo.ts`
- `src/utils/api.ts`
- `src/utils/toast.ts`

## 设计系统

- `src/styles/global.css` 是样式入口，负责引入 Tailwind 与全局基础样式
- `src/styles/tokens.css` 是唯一 design token 源，维护 Tailwind CSS v4 变量与动画
- `src/layouts/Layout.astro` 和 `src/components/Welcome.astro` 展示了模板的默认页面组织方式

## 说明

这是模板仓库，不内置项目级 Changesets、commitlint、Biome 配置或发布工作流。相关治理由 `one-cli` 在工作区根目录统一生成和维护。
