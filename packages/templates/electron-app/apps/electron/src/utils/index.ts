import { app } from "electron";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { DEFAULT_SCHEME } from "../constants";

// ESM 下 __dirname 不存在，基于 import.meta.url 重建
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// ESM 主进程要加载 CJS preload，需要 createRequire 才能 resolve workspace 包
const requireFromHere = createRequire(import.meta.url);

export const appName = app.getName();

// 工作目录：所有持久化数据（日志、store、缓存）的根
export const workspace = path.resolve(app.getPath("userData"));

// 自定义协议名
export const defaultScheme = DEFAULT_SCHEME;

// 空函数占位
export const noop = () => {};

// preload 脚本绝对路径 —— 主窗口 webPreferences.preload 要指向这里
// @app/preload 包的 main 指向 dist/index.cjs，这里 resolve 出真实路径
export const preloadPath = requireFromHere.resolve("@app/preload");

export { __dirname, __filename };
