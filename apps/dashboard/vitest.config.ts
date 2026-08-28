import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config";

export default mergeConfig(
	viteConfig,
	defineConfig({
		test: {
			environment: "jsdom",
			environmentOptions: {
				jsdom: { url: "http://localhost/?token=test-token" },
			},
			setupFiles: ["./src/test/setup.ts"],
		},
	}),
);
