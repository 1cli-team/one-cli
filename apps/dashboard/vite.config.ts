import path from "node:path";
import { fileURLToPath } from "node:url";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react-swc";
import { defineConfig } from "vite";

const projectDir = path.dirname(fileURLToPath(import.meta.url));

// VITE_DEV_API_TARGET lets contributors point `pnpm dev` at a local
// `one serve --no-ui --port <N>` instance. Default 5174 mirrors what the
// docs page suggests; nothing else in the repo cares about this port.
const devApiTarget = process.env.VITE_DEV_API_TARGET ?? "http://127.0.0.1:5174";

export default defineConfig({
	plugins: [tailwindcss(), react()],
	// Served from "/" by the embedded Go file server; no subpath rewriting.
	base: "/",
	resolve: {
		alias: {
			"@": path.resolve(projectDir, "src"),
		},
	},
	server: {
		port: 5173,
		// `pnpm dev` proxies /api/* to the local Go server so you can iterate
		// on the React side with HMR while the API stays in Go.
		proxy: {
			"/api": {
				target: devApiTarget,
				changeOrigin: false,
			},
		},
	},
	build: {
		outDir: "dist",
		// Sourcemaps would bloat the embedded bundle (Vite emits .js.map next
		// to each chunk). Drop for the binary; rebuild with `--sourcemap` if
		// you need them locally.
		sourcemap: false,
		rollupOptions: {
			output: {
				manualChunks(id) {
					if (id.includes("node_modules")) {
						if (
							id.includes("@radix-ui") ||
							id.includes("lucide-react") ||
							id.includes("sonner") ||
							id.includes("class-variance-authority")
						) {
							return "ui";
						}
						if (id.includes("react-router")) return "router";
						return "vendor";
					}
				},
			},
		},
	},
});
