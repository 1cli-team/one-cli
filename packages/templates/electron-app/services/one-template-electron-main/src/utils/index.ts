import { app } from "electron";
import { createRequire } from "node:module";
import path from "node:path";

const require = createRequire(import.meta.url);
const preloadPackage = "@one-template-electron/preload";
const rendererPackage = "@one-template-electron/renderer/package.json";
export const preloadPath = app.isPackaged
  ? path.resolve(import.meta.dirname, "../preload/index.cjs")
  : require.resolve(preloadPackage);
export const rendererPath = app.isPackaged
  ? path.resolve(import.meta.dirname, "../renderer/index.html")
  : path.join(
      path.dirname(require.resolve(rendererPackage)),
      "dist/index.html",
    );
