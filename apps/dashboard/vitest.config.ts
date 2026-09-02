import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config";

const nodeMajor = Number.parseInt(process.versions.node, 10);

export default mergeConfig(
	viteConfig,
	defineConfig({
		test: {
			// Node 25 exposes native Web Storage before Vitest installs jsdom.
			// Disable only the worker's Node implementation so tests use jsdom's
			// isolated localStorage without requiring a persistent storage file.
			execArgv: nodeMajor >= 25 ? ["--no-experimental-webstorage"] : [],
			environment: "jsdom",
			environmentOptions: {
				jsdom: { url: "http://localhost/" },
			},
			setupFiles: ["./src/test/setup.ts"],
		},
	}),
);
