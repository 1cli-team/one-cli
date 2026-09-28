import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { MemoryRouter } from "react-router-dom";
import { SWRConfig } from "swr";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { App } from "@/App";
import { ProjectTemplates } from "@/pages/ProjectTemplates";
import i18n from "@/lib/i18n";

const registry = JSON.parse(
	readFileSync(resolve(process.cwd(), "../../packages/templates/registry.json"), "utf8"),
);
const templates = registry.templates.map(
	(template: {
		id: string;
		name: string;
		description: string;
		category: "frontend" | "backend" | "library";
		toolchain: string;
	}) => ({
		...template,
		directory: { frontend: "apps", backend: "services", library: "packages" }[template.category],
	}),
);
const server = setupServer(
	http.get("http://localhost/api/project-templates", () => HttpResponse.json({ templates })),
	http.get("http://localhost/api/workspaces", () => HttpResponse.json({ workspaces: [] })),
	http.get("http://localhost/api/session", () =>
		HttpResponse.json({ session: { loggedIn: false } }),
	),
);
function show(withApp = false) {
	return render(
		<SWRConfig value={{ provider: () => new Map(), shouldRetryOnError: false }}>
			<MemoryRouter initialEntries={["/templates?env=preview"]}>
				{withApp ? <App /> : <ProjectTemplates />}
			</MemoryRouter>
		</SWRConfig>,
	);
}
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(async () => {
	await i18n.changeLanguage("en-US");
});
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe("project template catalog", () => {
	it.each(["en-US", "zh-CN"])("introduces every registered template in %s", async (locale) => {
		await i18n.changeLanguage(locale);
		show();
		expect(await screen.findAllByRole("article")).toHaveLength(templates.length);
		for (const template of templates) {
			const name = i18n.t(`projectCreate.templates.${template.id}.name`);
			const card = screen.getByRole("article", { name });
			for (const key of ["description", "stack", "feature1", "feature2", "feature3"]) {
				const value = i18n.getResource(
					locale,
					"translation",
					`templateCatalog.templates.${template.id}.${key}`,
				);
				expect(typeof value).toBe("string");
				expect(value.length).toBeGreaterThan(0);
				expect(within(card).getByText(value)).toBeDefined();
			}
			expect(within(card).getByText(`${template.directory}/`)).toBeDefined();
		}
		expect(
			screen
				.getByRole("link", { name: i18n.t("templateCatalog.chooseWorkspace") })
				.getAttribute("href"),
		).toBe("/?env=preview");
	});

	it("combines category and feature search, and resets an empty result", async () => {
		const user = userEvent.setup();
		show();
		await screen.findAllByRole("article");
		await user.click(screen.getByRole("button", { name: /Services/ }));
		expect(screen.getAllByRole("article")).toHaveLength(
			templates.filter((template: { category: string }) => template.category === "backend").length,
		);
		await user.type(
			screen.getByRole("textbox", { name: "Search templates, stacks, or features…" }),
			"  DRIZZLE  ",
		);
		expect(screen.getAllByRole("article")).toHaveLength(1);
		expect(screen.getByRole("article", { name: "NestJS API service" })).toBeDefined();
		await user.click(screen.getByRole("button", { name: /Libraries/ }));
		expect(screen.queryAllByRole("article")).toHaveLength(0);
		expect(screen.getByRole("heading", { name: "No matching templates" })).toBeDefined();
		await user.click(screen.getByRole("button", { name: "Clear filters" }));
		expect(screen.getAllByRole("article")).toHaveLength(templates.length);
	});

	it("updates introductions when the language changes", async () => {
		show();
		await screen.findByRole("article", { name: "React single-page application" });
		await act(async () => {
			await i18n.changeLanguage("zh-CN");
		});
		expect(await screen.findByRole("article", { name: "React 单页应用" })).toBeDefined();
		expect(screen.queryByText("Client-side routing and error boundaries")).toBeNull();
		expect(screen.getByText("客户端路由与错误边界")).toBeDefined();
	});

	it("recovers from a catalog error", async () => {
		server.use(
			http.get(
				"http://localhost/api/project-templates",
				() => new HttpResponse(null, { status: 500 }),
			),
		);
		const user = userEvent.setup();
		show();
		expect(await screen.findByRole("alert")).toBeDefined();
		server.use(
			http.get("http://localhost/api/project-templates", () => HttpResponse.json({ templates })),
		);
		await user.click(screen.getByRole("button", { name: "Retry" }));
		expect(await screen.findAllByRole("article")).toHaveLength(templates.length);
	});

	it("shows the catalog empty state", async () => {
		server.use(
			http.get("http://localhost/api/project-templates", () =>
				HttpResponse.json({ templates: [] }),
			),
		);
		show();
		expect(
			await screen.findByRole("heading", { name: i18n.t("projectCreate.noTemplates") }),
		).toBeDefined();
		expect(screen.queryByRole("textbox")).toBeNull();
	});

	it("exposes the page through the brand menu and breadcrumb", async () => {
		const user = userEvent.setup();
		show(true);
		await screen.findByRole("heading", { name: "Project templates", level: 1 });
		const breadcrumb = screen.getByRole("navigation", { name: "breadcrumb" });
		expect(within(breadcrumb).getByText("Project templates")).toBeDefined();
		await user.click(screen.getByRole("button", { name: "One CLI navigation menu" }));
		const item = screen.getByRole("menuitem", { name: "Project templates" });
		expect(item.getAttribute("href")).toBe("/templates?env=preview");
		expect(item.getAttribute("aria-current")).toBe("page");
		await user.click(item);
		await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
		expect(screen.getByRole("heading", { name: "Project templates", level: 1 })).toBeDefined();
	});
});
