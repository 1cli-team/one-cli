# Web CSR React Template

面向浏览器端单页应用的 React 模板，当前基线已经切到 `shadcn/ui (Base UI) + Tailwind CSS v4 + CSS variables`，并保留了 SWR、Zustand、Axios、主题切换和错误兜底这些常用能力。

## 技术栈

- React 19
- TypeScript 7
- Vite 8
- shadcn/ui (Base UI, base-nova)
- Tailwind CSS v4
- SWR
- Zustand
- Axios
- Base UI Toast
- oxlint + oxfmt

## 目录结构

```text
src/
├── api/                 # 请求 key 与纯函数 API
├── components/
│   ├── ErrorBoundary.tsx
│   └── ui/              # shadcn/ui 基础组件
├── hooks/               # 组合 hooks（如 toast）
├── lib/
│   ├── http.ts          # Axios 客户端
│   ├── toast.ts         # Base UI Toast 封装
│   ├── utils.ts         # cn 等工具函数
│   └── stores/          # Zustand stores
├── pages/               # 页面组件
├── providers/           # Theme / SWR Provider
├── router/              # 路由配置
├── styles/              # reset、Tailwind、设计令牌
├── types/               # 类型定义
├── App.tsx
└── main.tsx
```

## 快速开始

```bash
pnpm install
pnpm dev
```

默认开发地址：`http://localhost:5173`

## 常用命令

| 命令              | 说明                       |
| ----------------- | -------------------------- |
| `pnpm dev`        | 启动开发服务器             |
| `pnpm build`      | 构建生产产物               |
| `pnpm preview`    | 预览构建结果               |
| `pnpm lint`       | 执行 oxlint                |
| `pnpm lint:fix`   | 自动修复 lint 问题         |
| `pnpm format`     | 检查格式                   |
| `pnpm format:fix` | 自动格式化                 |
| `pnpm check`      | 执行 lint + format         |
| `pnpm check:fix`  | 执行 lint:fix + format:fix |

## 请求层约定

模板里的请求模块统一采用 “`key + pure function`” 形式，便于 SWR 复用和测试。当前示例接口是本地 mock，但组织方式和真实请求保持一致。

```ts
export const demoKey = "/api/demo";

export async function getDemo() {
  return {
    message: "SWR cache 已同步完成",
    timestamp: new Date().toLocaleString("zh-CN"),
    count: Math.floor(Math.random() * 100),
    status: "success",
  };
}
```

页面消费时直接复用导出的 key 和请求函数：

```tsx
const { data, isLoading } = useSWR(demoKey, getDemo);
```

当前示例可参考：

- `src/api/demo.ts`
- `src/api/auth.ts`
- `src/api/users.ts`

## UI 与主题

- `src/components/ui/*` 提供模板内置的 shadcn/ui 组件
- `src/providers/ThemeProvider.tsx` 负责同步 light / dark 主题
- `src/styles/index.css` 是样式入口，负责串联 reset、tokens 与 Tailwind
- `src/styles/tokens.css` 是唯一 design token 源
- `src/styles/tailwind.css` 负责 Tailwind v4 的 `@theme inline` 映射与全局样式层
- `src/lib/toast.ts` 封装 Base UI Toast，保留提示创建、更新和关闭接口 / Wraps Base UI Toast with create, update, and close helpers

### shadcn/ui Base UI

模板使用 shadcn/ui 的 `base-nova` 风格，保留 `components.json` 和 Tailwind CSS 设计令牌。交互组件与 Toast 均基于 `@base-ui/react`，提示沿用模板的主题颜色。
The template uses shadcn/ui's `base-nova` style with `components.json` and Tailwind CSS design tokens. Interactive components and toasts use `@base-ui/react`; notifications retain the template's theme colors.

在生成的项目目录添加组件 / Add components from the generated project directory:

```sh
pnpm dlx shadcn@latest add dialog
```

组合组件使用 `render` / Compose components using `render`:

```tsx
<Button render={<button type="button" />}>Submit</Button>
```

Toast 示例 / Toast example:

```tsx
import { toast } from "@/components/ui/toast";

toast.add({
  title: "保存成功 / Saved",
  type: "success",
});
```

## 说明

这是模板仓库，不内置项目级 CI、Git hooks、Docker Compose 或发布流程。工作区治理由 `one-cli` 提供，提交检查使用工作区的 hk 配置；版本管理和发布流程由项目按需配置。
