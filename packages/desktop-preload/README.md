# One Desktop preload

The context-isolated bridge exposes only the typed methods in `src/api.ts`. It never exposes raw IPC, Node.js or filesystem access to the renderer. Main validates the sender, main frame, trusted origin and arguments.

桥接保持最小权限；变更原生接口时同步修改 main、preload 类型与 renderer 调用，并运行桌面回归测试。
