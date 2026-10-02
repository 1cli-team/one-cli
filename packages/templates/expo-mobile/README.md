# Expo Mobile

Expo · Expo Router · React Native · TypeScript · Axios · SWR · Zustand · MMKV.

## 中文

- `pnpm android` / `pnpm ios`：构建原生开发客户端；`pnpm start`：启动开发服务器；`pnpm web`：浏览器开发。
- MMKV 依赖 Nitro 原生模块，需要开发客户端，不能用 Expo Go 验证。
- `pnpm check`、`pnpm test` 检查类型、格式与原生网络/前后台监听生命周期。
- 只有首页、404 和主题切换。使用 React Native 样式，系统主题为默认值；MMKV 只保存 UI 偏好。
- Axios 接收 `EXPO_PUBLIC_API_URL`。SWR 管理远程缓存，Zustand 管理共享 UI；没有 token、登录或自定义远程缓存。
- 通过 `pnpm exec expo install --check` 检查 SDK 依赖兼容性，按需手动安装业务 skills。

## English

- `pnpm android` / `pnpm ios`: build native development clients; `pnpm start`: start the development server; `pnpm web`: browser development.
- MMKV requires Nitro native modules and a development client. Expo Go cannot validate it.
- `pnpm check` and `pnpm test` validate types, formatting, and network/foreground listener lifecycles.
- Includes only home, 404, and theme switching. Uses React Native styles, defaults to the system theme, and stores UI preferences in MMKV.
- Axios reads `EXPO_PUBLIC_API_URL`. SWR owns remote caching; Zustand owns shared UI. No token, login, or custom remote cache is included.
- Check SDK compatibility with `pnpm exec expo install --check`. Install business skills manually as needed.
