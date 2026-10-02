# React SPA

React · TypeScript · shadcn/ui (Base UI) · Tailwind CSS · Axios · SWR · Zustand.

## 中文

- `pnpm dev`、`pnpm build`、`pnpm check`：开发、构建、检查。
- 起始页面包含主题切换和 Base UI Toast，没有认证或业务 API。
- `src/lib/http.ts` 提供 Axios 实例和原始 JSON fetcher；通过 `.env.example` 中的公开 API 地址连接后端。
- SWR 管理远程数据与缓存；Zustand 只管理共享 UI 状态，组件局部状态使用 React。
- Next.js 的 store 必须按 Provider 创建；服务端数据使用服务端函数，私密环境变量不能传入客户端。
- 按需用 shadcn 添加 Base UI 组件，手动安装业务 skills。

## English

- `pnpm dev`, `pnpm build`, `pnpm check`: develop, build, validate.
- The starter includes theme switching and Base UI Toast, with no authentication or business API.
- `src/lib/http.ts` exports an Axios client and a raw JSON fetcher. Configure the public API URL from `.env.example`.
- SWR owns remote data and caching. Zustand owns shared UI state; use React for local state.
- In Next.js, create stores per Provider. Fetch server data with server functions and keep private environment variables off the client.
- Add Base UI components with shadcn as needed. Install business skills manually.
