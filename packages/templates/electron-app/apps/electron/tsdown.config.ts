import { defineConfig } from "tsdown";

export default defineConfig({
  entry: { index: "src/index.ts" },
  format: "esm",
  outDir: "build/main",
  platform: "node",
  target: "node24",
  fixedExtension: false,
  deps: {
    neverBundle: [
      "electron",
      "electron-updater",
      "electron-devtools-installer",
    ],
  },
  dts: false,
  clean: true,
  sourcemap: true,
});
