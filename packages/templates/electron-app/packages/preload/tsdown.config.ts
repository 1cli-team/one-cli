import { defineConfig } from "tsdown";

export default defineConfig([
  {
    entry: { index: "src/index.ts" },
    format: "cjs",
    outDir: "dist",
    outExtensions: () => ({ js: ".cjs" }),
    external: ["electron"],
    dts: true,
    clean: true,
  },
  {
    entry: { channels: "src/channels.ts" },
    format: "esm",
    outDir: "dist",
    dts: true,
    clean: false,
  },
]);
