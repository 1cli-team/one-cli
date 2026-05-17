import path from "node:path";
import { fileURLToPath } from "node:url";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react-swc";
import { defineConfig } from "vite";

const templateDir = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
	plugins: [tailwindcss(), react()],
	resolve: {
		alias: {
			"@": path.resolve(templateDir, "src"),
			"@components": path.resolve(templateDir, "src/components"),
			"@lib": path.resolve(templateDir, "src/lib"),
			"@pages": path.resolve(templateDir, "src/pages"),
			"@hooks": path.resolve(templateDir, "src/hooks"),
			"@types": path.resolve(templateDir, "src/types"),
		},
	},
	build: {
		sourcemap: true,
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
