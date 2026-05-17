# Web SSR Next Template

基于 Next.js App Router 的 SSR 模板，适合需要服务端渲染、API Route、主题切换和基础交互组件的 Web 应用。模板当前已经统一到 `shadcn/ui + Tailwind CSS v4 + SWR + Zustand` 这一套前端基线。

## 技术栈

- Next.js 16
- React 19
- TypeScript 5
- shadcn/ui + Radix UI
- Tailwind CSS v4
- next-themes
- SWR
- Zustand
- Axios
- sonner
- oxlint + oxfmt

## 目录结构

```text
src/
├── api/                  # 请求 key 与纯函数 API
├── app/                  # App Router 页面与 route handlers
│   └── api/hello/route.ts
├── components/           # 页面组件与 shadcn/ui 组件
├── lib/
│   ├── http.ts           # Axios 客户端
│   ├── server-data.ts    # 服务端数据获取封装
│   └── utils.ts          # cn 等工具函数
└── stores/               # Zustand store
```

## 快速开始

```bash
pnpm install
pnpm dev
```

默认开发地址：`http://localhost:3000`

## 常用命令

| 命令              | 说明                       |
| ----------------- | -------------------------- |
| `pnpm dev`        | 启动开发服务器             |
| `pnpm build`      | 构建生产产物               |
| `pnpm start`      | 启动生产服务               |
| `pnpm lint`       | 执行 oxlint                |
| `pnpm lint:fix`   | 自动修复 lint 问题         |
| `pnpm format`     | 检查格式                   |
| `pnpm format:fix` | 自动格式化                 |
| `pnpm check`      | 执行 lint + format         |
| `pnpm check:fix`  | 执行 lint:fix + format:fix |

## 请求层约定

模板中的请求模块统一采用 “`key + pure function`” 形式，避免在页面和组件里内联 endpoint。

```ts
export const helloKey = "/api/hello";

export function getHello() {
  return http.get(helloKey);
}
```

当前示例位于：

- `src/api/hello.ts`
- `src/lib/server-data.ts`

## UI 与主题

- `src/components/ui/*` 提供 shadcn/ui 组件
- `src/components/theme-provider.tsx` 与 `theme-toggle.tsx` 管理主题切换
- `src/app/globals.css` 是样式入口，负责 `@theme inline` 映射与全局样式层
- `src/styles/tokens.css` 是 design token 源，维护 light / dark 与状态语义变量
- `src/components/ui/sonner.tsx` 在 `layout.tsx` 中挂载 `<Toaster />`，业务侧用 `import { toast } from "sonner"` 调用
- `src/app/global-error.tsx` 提供全局错误兜底

## 说明

这是模板仓库，不内置项目级 Changesets、Biome、额外 docs 目录或发布流水线。相关治理由 `one-cli` 在工作区根目录统一生成和维护。
