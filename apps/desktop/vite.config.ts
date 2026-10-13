import path from "node:path";
import { fileURLToPath } from "node:url";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const projectDir = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
	plugins: [tailwindcss(), react()],
	// Static assets are packaged by Tauri; browser requests go directly to Go.
	base: "/",
	resolve: {
		alias: {
			"@": path.resolve(projectDir, "src"),
		},
	},
	server: { host: "127.0.0.1", port: 5173, strictPort: true },
	build: {
		outDir: "dist",
		// Sourcemaps would bloat the embedded bundle (Vite emits .js.map next
		// to each chunk). Drop for the binary; rebuild with `--sourcemap` if
		// you need them locally.
		sourcemap: false,
		rolldownOptions: {
			output: {
				codeSplitting: {
					groups: [
						{
							debugName: "vendor-groups",
							name(id) {
								if (!id.includes("node_modules")) return null;
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
							},
						},
					],
				},
			},
		},
	},
});
