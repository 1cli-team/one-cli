import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { SWRConfig } from "swr";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { CreateProjectDialog } from "@/features/project-creation/CreateProjectDialog";
import i18n from "@/lib/i18n";

const emptyTemplates = [
	{ id: "empty-app", category: "frontend", directory: "apps" },
	{ id: "empty-service", category: "backend", directory: "services" },
	{ id: "empty-library", category: "library", directory: "packages" },
].map((template) => ({
	...template,
	name: template.id,
	description: template.id,
	toolchain: "none",
}));

const templates = [
	...emptyTemplates,
	{
		id: "react-spa",
		name: "React",
		description: "React",
		category: "frontend",
		directory: "apps",
		toolchain: "node",
	},
	{
		id: "electron-app",
		name: "Electron",
		description: "Electron",
		category: "frontend",
		directory: "apps",
		toolchain: "node",
		projects: [
			{ directory: "apps", suffix: "-renderer" },
			{ directory: "services", suffix: "-main" },
			{ directory: "packages", suffix: "-preload" },
		],
	},
	{
		id: "go-api",
		name: "Go",
		description: "Go",
		category: "backend",
		directory: "services",
		toolchain: "go",
	},
];
const server = setupServer();
function show(projectNames: string[] = []) {
	const onClose = vi.fn();
	const onCreated = vi.fn();
	render(
		<SWRConfig value={{ provider: () => new Map(), shouldRetryOnError: false }}>
			<CreateProjectDialog
				entryId="selected-workspace"
				projectNames={projectNames}
				onClose={onClose}
				onCreated={onCreated}
			/>
		</SWRConfig>,
	);
	return { onClose, onCreated };
}

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(async () => {
	await i18n.changeLanguage("en-US");
	server.use(
		http.get("http://localhost/api/project-templates", () =>
			HttpResponse.json({ templates: [...templates].reverse() }),
		),
	);
});
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe("project creation", () => {
	it.each(
		["en-US", "zh-CN"].flatMap((locale) =>
			[...emptyTemplates, { id: "go-api", directory: "services" }].map((template) => ({
				locale,
				...template,
			})),
		),
	)("creates $id in $locale", async ({ locale, id, directory }) => {
		await i18n.changeLanguage(locale);
		const user = userEvent.setup();
		let payload: unknown;
		server.use(
			http.post(
				"http://localhost/api/workspaces/:entryId/projects",
				async ({ params, request }) => {
					expect(params.entryId).toBe("selected-workspace");
					payload = await request.json();
					return HttpResponse.json(
						{ name: "api", relativeDir: `${directory}/api`, templateId: id },
						{ status: 201 },
					);
				},
			),
		);
		const { onClose, onCreated } = show();
		await user.type(screen.getByLabelText(i18n.t("projectCreate.name")), "api");
		const select = await screen.findByRole("combobox", { name: i18n.t("projectCreate.template") });
		await waitFor(() => expect((select as HTMLButtonElement).disabled).toBe(false));
		await user.click(select);
		await user.click(
			await screen.findByRole("option", { name: i18n.t(`projectCreate.templates.${id}.name`) }),
		);
		expect(screen.getByText(`${directory}/api`)).toBeDefined();
		await user.click(screen.getByRole("button", { name: i18n.t("projectCreate.submit") }));
		await waitFor(() => expect(onCreated).toHaveBeenCalledWith("api"));
		expect(payload).toEqual({ name: "api", templateId: id });
		expect(onClose).toHaveBeenCalledOnce();
	});

	it.each(["en-US", "zh-CN"])("previews and creates all Electron members in %s", async (locale) => {
		await i18n.changeLanguage(locale);
		const user = userEvent.setup();
		server.use(
			http.post("http://localhost/api/workspaces/:entryId/projects", async ({ request }) => {
				expect(await request.json()).toEqual({ name: "desktop", templateId: "electron-app" });
				return HttpResponse.json(
					{
						name: "desktop",
						relativeDir: ".",
						templateId: "electron-app",
						projects: [
							{
								name: "desktop-renderer",
								relativeDir: "apps/desktop-renderer",
								templateId: "electron-app",
							},
							{
								name: "desktop-main",
								relativeDir: "services/desktop-main",
								templateId: "electron-app",
							},
							{
								name: "desktop-preload",
								relativeDir: "packages/desktop-preload",
								templateId: "electron-app",
							},
						],
					},
					{ status: 201 },
				);
			}),
		);
		const { onCreated } = show();
		await user.type(screen.getByLabelText(i18n.t("projectCreate.name")), "desktop");
		const select = await screen.findByRole("combobox", { name: i18n.t("projectCreate.template") });
		await waitFor(() => expect((select as HTMLButtonElement).disabled).toBe(false));
		await user.click(select);
		await user.click(
			await screen.findByRole("option", {
				name: i18n.t("projectCreate.templates.electron-app.name"),
			}),
		);
		for (const dir of [
			"apps/desktop-renderer",
			"services/desktop-main",
			"packages/desktop-preload",
		])
			expect(screen.getByText(dir)).toBeDefined();
		await user.click(screen.getByRole("button", { name: i18n.t("projectCreate.submit") }));
		await waitFor(() => expect(onCreated).toHaveBeenCalledWith("desktop-renderer"));
	});

	it("rejects a conflicting Electron member before creation", async () => {
		const user = userEvent.setup();
		const post = vi.fn();
		server.use(http.post("http://localhost/api/workspaces/:entryId/projects", post));
		show(["desktop-main"]);
		await user.type(screen.getByLabelText("Project name"), "desktop");
		const select = await screen.findByRole("combobox", { name: i18n.t("projectCreate.template") });
		await waitFor(() => expect((select as HTMLButtonElement).disabled).toBe(false));
		await user.click(select);
		await user.click(
			await screen.findByRole("option", {
				name: i18n.t("projectCreate.templates.electron-app.name"),
			}),
		);
		await user.click(screen.getByRole("button", { name: "Create project" }));
		expect(screen.getByRole("alert")).toBeDefined();
		expect(post).not.toHaveBeenCalled();
	});

	it("validates names and duplicates without sending requests", async () => {
		const user = userEvent.setup();
		const post = vi.fn();
		server.use(http.post("http://localhost/api/workspaces/:entryId/projects", post));
		show(["web"]);
		const name = screen.getByLabelText("Project name");
		const submit = screen.getByRole("button", { name: "Create project" });
		await waitFor(() => expect((submit as HTMLButtonElement).disabled).toBe(false));
		for (const value of ["../outside", "web"]) {
			await user.clear(name);
			await user.type(name, value);
			await user.click(submit);
			expect(name.getAttribute("aria-invalid")).toBe("true");
			expect(document.activeElement).toBe(name);
			expect(screen.getByRole("alert")).toBeDefined();
		}
		expect(post).not.toHaveBeenCalled();
	});

	it("retains input on server errors and allows retry", async () => {
		const user = userEvent.setup();
		let calls = 0;
		server.use(
			http.post("http://localhost/api/workspaces/:entryId/projects", () => {
				calls++;
				if (calls === 1)
					return HttpResponse.json(
						{ error: { code: "TARGET_EXISTS", message: "Directory already exists" } },
						{ status: 409 },
					);
				return HttpResponse.json(
					{ name: "web", relativeDir: "apps/web", templateId: "react-spa" },
					{ status: 201 },
				);
			}),
		);
		const { onCreated } = show();
		await user.type(screen.getByLabelText("Project name"), "web");
		await user.click(screen.getByRole("button", { name: "Create project" }));
		expect(await screen.findByText("Directory already exists")).toBeDefined();
		expect((screen.getByLabelText("Project name") as HTMLInputElement).value).toBe("web");
		await user.click(screen.getByRole("button", { name: "Create project" }));
		await waitFor(() => expect(onCreated).toHaveBeenCalledWith("web"));
	});

	it("blocks duplicate submissions and closing while creation is pending", async () => {
		const user = userEvent.setup();
		let release!: () => void;
		const blocked = new Promise<void>((resolve) => {
			release = resolve;
		});
		const post = vi.fn(async () => {
			await blocked;
			return HttpResponse.json(
				{ name: "web", relativeDir: "apps/web", templateId: "react-spa" },
				{ status: 201 },
			);
		});
		server.use(http.post("http://localhost/api/workspaces/:entryId/projects", post));
		const { onClose } = show();
		await user.type(screen.getByLabelText("Project name"), "web");
		await user.dblClick(screen.getByRole("button", { name: "Create project" }));
		expect(await screen.findByRole("status")).toBeDefined();
		expect((screen.getByRole("button", { name: "Cancel" }) as HTMLButtonElement).disabled).toBe(
			true,
		);
		await user.keyboard("{Escape}");
		expect(onClose).not.toHaveBeenCalled();
		expect(post).toHaveBeenCalledOnce();
		release();
		await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
	});

	it("recovers from template loading failure and updates translated template labels", async () => {
		const user = userEvent.setup();
		server.use(
			http.get(
				"http://localhost/api/project-templates",
				() => new HttpResponse(null, { status: 500 }),
			),
		);
		show();
		expect(await screen.findByText("Could not load templates")).toBeDefined();
		server.use(
			http.get("http://localhost/api/project-templates", () => HttpResponse.json({ templates })),
		);
		await user.click(screen.getByRole("button", { name: "Retry" }));
		await screen.findByRole("combobox", { name: i18n.t("projectCreate.template") });
		await i18n.changeLanguage("zh-CN");
		const dialog = await screen.findByRole("dialog", { name: "新建项目" });
		expect(await within(dialog).findByText("React 单页应用")).toBeDefined();
	});
});
