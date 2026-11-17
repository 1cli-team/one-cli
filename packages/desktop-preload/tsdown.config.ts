import { defineConfig } from "tsdown";

export default defineConfig({
  entry: { index: "src/index.ts" },
  format: "cjs",
  outDir: "dist",
  outExtensions: () => ({ js: ".cjs" }),
  deps: { neverBundle: ["electron"] },
  dts: false,
  clean: true,
});
