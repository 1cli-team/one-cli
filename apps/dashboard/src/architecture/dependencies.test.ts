import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { basename, join, relative, sep } from "node:path";
import { describe, expect, it } from "vitest";

const SOURCE_ROOT = join(process.cwd(), "src");
const SOURCE_EXTENSIONS = new Set([".ts", ".tsx"]);

interface DependencyRule {
	from: string;
	forbidden: readonly string[];
}

const DEPENDENCY_RULES: readonly DependencyRule[] = [
	{
		from: "api",
		forbidden: ["components", "features", "hooks", "pages", "providers", "router"],
	},
	{
		from: "components/ui",
		forbidden: ["api", "features", "hooks", "pages", "providers", "router"],
	},
	{ from: "features", forbidden: ["pages", "router"] },
	{ from: "pages", forbidden: ["pages", "router"] },
];

describe("Dashboard dependency boundaries", () => {
	it("keeps imports pointing inward", () => {
		const violations: string[] = [];
		for (const file of productionSourceFiles()) {
			const source = sourcePath(file);
			const imports = aliasImports(readFileSync(file, "utf8"));
			const rule = DEPENDENCY_RULES.find(({ from }) => inArea(source, from));

			for (const imported of imports) {
				const target = imported.slice(2);
				if (target.startsWith("pages/") && !inArea(source, "router")) {
					violations.push(`${source} imports routed page ${imported}`);
					continue;
				}
				const blocked = rule?.forbidden.find((area) => inArea(target, area));
				if (blocked) {
					violations.push(`${source} imports outer area ${imported}`);
				}
			}
		}

		expect(violations, violations.join("\n")).toEqual([]);
	});

	it("uses direct source imports instead of local barrel entrypoints", () => {
		const violations: string[] = [];
		for (const file of productionSourceFiles()) {
			const source = sourcePath(file);
			const contents = readFileSync(file, "utf8");
			if (/^index\.tsx?$/.test(basename(file)) && /\bexport\s+(?:\*|\{)/.test(contents)) {
				violations.push(`${source} is a local barrel file`);
			}

			for (const imported of aliasImports(contents)) {
				const target = join(SOURCE_ROOT, imported.slice(2));
				if (existsSync(target) && statSync(target).isDirectory()) {
					violations.push(`${source} imports directory barrel ${imported}`);
				}
			}
		}

		expect(violations, violations.join("\n")).toEqual([]);
	});

	it("keeps interactive primitives inside components/ui", () => {
		const violations: string[] = [];
		const forbidden = [
			{ pattern: /<button\b/g, label: "native <button>" },
			{ pattern: /<input\b/g, label: "native <input>" },
			{ pattern: /<select\b/g, label: "native <select>" },
			{ pattern: /<textarea\b/g, label: "native <textarea>" },
			{ pattern: /\bwindow\.confirm\s*\(/g, label: "window.confirm" },
		] as const;

		for (const file of productionSourceFiles()) {
			const source = sourcePath(file);
			if (inArea(source, "components/ui")) continue;
			const contents = readFileSync(file, "utf8");
			for (const { pattern, label } of forbidden) {
				if (pattern.test(contents)) violations.push(`${source} uses ${label}`);
				pattern.lastIndex = 0;
			}
		}

		expect(violations, violations.join("\n")).toEqual([]);
	});
});

function productionSourceFiles(root = SOURCE_ROOT): string[] {
	const files: string[] = [];
	for (const entry of readdirSync(root, { withFileTypes: true })) {
		const path = join(root, entry.name);
		if (entry.isDirectory()) {
			if (entry.name !== "test") files.push(...productionSourceFiles(path));
			continue;
		}
		const extension = entry.name.endsWith(".tsx")
			? ".tsx"
			: entry.name.endsWith(".ts")
				? ".ts"
				: "";
		if (
			SOURCE_EXTENSIONS.has(extension) &&
			!entry.name.includes(".test.") &&
			!entry.name.endsWith(".d.ts")
		) {
			files.push(path);
		}
	}
	return files.sort();
}

function aliasImports(source: string): string[] {
	const imports: string[] = [];
	const patterns = [
		/\bfrom\s+["'](@\/[^"']+)["']/g,
		/\bimport\s+["'](@\/[^"']+)["']/g,
		/\bimport\s*\(\s*["'](@\/[^"']+)["']\s*\)/g,
	];
	for (const pattern of patterns) {
		for (const match of source.matchAll(pattern)) {
			if (match[1]) imports.push(match[1]);
		}
	}
	return imports;
}

function sourcePath(file: string): string {
	return relative(SOURCE_ROOT, file).split(sep).join("/");
}

function inArea(path: string, area: string): boolean {
	return path === area || path.startsWith(`${area}/`);
}
