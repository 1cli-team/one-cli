import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  server: {
    port: Number(process.env.ELECTRON_RENDERER_PORT || 5173),
    strictPort: true,
  },
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  envDir: "../../",
  envPrefix: "APP",
  build: {
    outDir: path.resolve(__dirname, "../electron/build/renderer"),
    emptyOutDir: true,
  },
});
