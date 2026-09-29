import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { MemoryRouter, useLocation } from "react-router-dom";
import useSWR, { SWRConfig } from "swr";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import {
	getOverview,
	overviewKeyFor,
	projectSettingsKey,
	workspaceEnvironmentKey,
} from "@/api/workspace";
import { environmentFromSearch } from "@/features/environment-context/environment";
import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import i18n from "@/lib/i18n";
import { Overview } from "@/pages/Overview";
import type {
	BackendSpec,
	Overview as OverviewPayload,
	ProjectSettingsResponse,
} from "@/types/api";

const server = setupServer(
	http.get(/\/api\/workspaces?(?:\/[^/]+)?\/services$/, () => HttpResponse.json({ services: [] })),
);

const catalogBackends: BackendSpec[] = [
	{
		id: "env/infisical",
		domain: "env",
		name: "infisical",
		capabilities: ["env-load"],
		project: { configurable: false },
	},
];

const overview: OverviewPayload = {
	schema: "one-cli/workspace-overview/v1",
	present: true,
	root: "/workspace/demo",
	workspace: {
		id: "demo",
		name: "demo",
		manifestVersion: 1,
		defaultEnvironment: "dev",
		environments: ["dev", "staging", "prod"],
		domains: { env: "infisical" },
	},
	projects: [
		{
			name: "web",
			relativeDir: "apps/web",
			kind: "app",
			toolchain: "node",
			domains: { env: "infisical" },
		},
		{
			name: "api",
			relativeDir: "services/api",
			kind: "service",
			toolchain: "go",
			domains: { env: "infisical" },
		},
		{
			name: "shared",
			relativeDir: "packages/shared",
			kind: "package",
			toolchain: "node",
			domains: { env: "infisical" },
		},
	],
};

const webSettings: ProjectSettingsResponse = {
	schema: "one-cli/workspace-project/v1",
	root: "/workspace/demo",
	environment: "dev",
	revision: "sha256:test-revision",
	project: {
		name: "web",
		relativeDir: "apps/web",
		kind: "app",
		toolchain: "node",
		packageManager: "pnpm",
		devAvailable: true,
		build: {
			command: "pnpm run build",
			source: "package.json#scripts.build",
			status: "ready",
		},
		availableEnvironments: ["dev", "staging", "prod"],
		environment: {
			backend: "infisical",
			path: "/apps/web",
			inherits: true,
		},
	},
};

const OverviewHarness: React.FC<{
	data: OverviewPayload;
	workspaceEntryId?: string;
	readOnly?: boolean;
	revalidateOverview?: boolean;
}> = ({ data, workspaceEntryId, readOnly, revalidateOverview }) => {
	const { search } = useLocation();
	const environment = environmentFromSearch(search);
	const current = useSWR<OverviewPayload>(
		overviewKeyFor(workspaceEntryId, environment),
		revalidateOverview ? () => getOverview(workspaceEntryId, environment) : null,
		{
			fallbackData: data,
			revalidateOnMount: false,
		},
	);

	return (
		<>
			<output data-testid="environment-search">{search}</output>
			<Overview
				data={current.data ?? data}
				workspaceEntryId={workspaceEntryId}
				readOnly={readOnly}
			/>
		</>
	);
};

function renderOverview(
	data: OverviewPayload = overview,
	workspaceEntryId?: string,
	readOnly?: boolean,
	revalidateOverview?: boolean,
	environment = "dev",
) {
	return render(
		<SWRConfig value={{ provider: () => new Map() }}>
			<MemoryRouter initialEntries={[`/?env=${environment}`]}>
				<OverviewHarness
					data={data}
					workspaceEntryId={workspaceEntryId}
					readOnly={readOnly}
					revalidateOverview={revalidateOverview}
				/>
			</MemoryRouter>
		</SWRConfig>,
	);
}

async function openProjectSettings() {
	return screen.findByRole("region", { name: "Project settings" });
}

async function openProjectSettingsTab(
	user: ReturnType<typeof userEvent.setup>,
	tabName: "Environment",
) {
	const settings = await openProjectSettings();
	await user.click(within(settings).getByRole("tab", { name: tabName }));
	return settings;
}

async function openWorkspaceSettingsDialog(user: ReturnType<typeof userEvent.setup>) {
	const inspector = await openProjectSettings();
	await user.click(within(inspector).getByRole("button", { name: "Workspace settings" }));
	return screen.findByRole("dialog", { name: "Workspace settings" });
}

async function openWorkspaceEnvironmentSettings(user: ReturnType<typeof userEvent.setup>) {
	const dialog = await openWorkspaceSettingsDialog(user);
	return within(dialog).findByRole("region", { name: "Workspace environment" });
}

describe("workspace overview Profile-only configuration", () => {
	beforeAll(async () => {
		server.listen({ onUnhandledRequest: "error" });
		await i18n.changeLanguage("en-US");
	});
	beforeEach(() => {
		server.use(
			http.get("http://localhost/api/catalog", () =>
				HttpResponse.json({ schema: "one-cli/catalog/v1", backends: catalogBackends }),
			),
			http.get("http://localhost/api/workspace/environment", ({ request }) =>
				HttpResponse.json({
					schema: "one-cli/workspace-profile/v1",
					root: "/workspace/demo",
					environment: new URL(request.url).searchParams.get("env") ?? "",
					domain: "env",
					backend: "infisical",
					configurable: false,
				}),
			),
			http.get("http://localhost/api/workspaces/:entryId/environment", ({ request }) =>
				HttpResponse.json({
					schema: "one-cli/workspace-profile/v1",
					root: "/workspace/demo",
					environment: new URL(request.url).searchParams.get("env") ?? "",
					domain: "env",
					backend: "infisical",
					configurable: false,
				}),
			),
			http.get("http://localhost/api/workspace/secrets", () =>
				HttpResponse.json({
					schema: "one-cli/env-list/v1",
					env: "dev",
					path: "/",
					keys: [],
					total: 0,
				}),
			),
			http.get("http://localhost/api/workspaces/:entryId/secrets", () =>
				HttpResponse.json({
					schema: "one-cli/env-list/v1",
					env: "dev",
					path: "/",
					keys: [],
					total: 0,
				}),
			),
			http.get("http://localhost/api/workspace/projects/:projectName", ({ params }) =>
				HttpResponse.json({
					...webSettings,
					project: {
						...webSettings.project,
						name: String(params.projectName),
					},
				}),
			),
			http.get("http://localhost/api/workspaces/:entryId/projects/:projectName", ({ params }) =>
				HttpResponse.json({
					...webSettings,
					project: {
						...webSettings.project,
						name: String(params.projectName),
					},
				}),
			),
		);
	});
	afterEach(() => {
		useManifestDraftStore.getState().clearWorkspace();
		useManifestDraftStore.getState().clearWorkspace("demo-entry");
		server.resetHandlers();
		vi.restoreAllMocks();
	});
	afterAll(() => server.close());

	it("creates a project from the empty workspace and selects it after refreshing", async () => {
		const user = userEvent.setup();
		let created = false;
		server.use(
			http.get("http://localhost/api/project-templates", () =>
				HttpResponse.json({
					templates: [
						{
							id: "react-spa",
							name: "React",
							description: "React",
							category: "frontend",
							directory: "apps",
							toolchain: "node",
						},
					],
				}),
			),
			http.post("http://localhost/api/workspaces/demo-entry/projects", async ({ request }) => {
				expect(await request.json()).toEqual({ name: "web", templateId: "react-spa" });
				created = true;
				return HttpResponse.json(
					{ name: "web", relativeDir: "apps/web", templateId: "react-spa" },
					{ status: 201 },
				);
			}),
			http.get("http://localhost/api/workspaces/demo-entry/overview", () =>
				HttpResponse.json({ ...overview, projects: created ? [overview.projects![0]] : [] }),
			),
			http.get("http://localhost/api/workspaces", () => HttpResponse.json({ workspaces: [] })),
		);
		renderOverview({ ...overview, projects: [] }, "demo-entry", false, true);
		const navigation = screen.getByRole("navigation", { name: "Workspace projects" });
		await user.click(within(navigation).getByRole("button", { name: "New project" }));
		const dialog = await screen.findByRole("dialog", { name: "New project" });
		await user.type(within(dialog).getByLabelText("Project name"), "web");
		await user.click(within(dialog).getByRole("button", { name: "Create project" }));
		const project = await within(navigation).findByRole("button", { name: "web apps/web" });
		expect(project.getAttribute("aria-current")).toBe("page");
		expect(screen.queryByRole("dialog", { name: "New project" })).toBeNull();
	});

	it("hides project creation for an identity-conflicted workspace", () => {
		renderOverview({ ...overview, projects: [] }, "demo-entry", true);
		expect(screen.queryByRole("button", { name: "New project" })).toBeNull();
	});

	it("shows projects in a persistent left-hand list with project settings tabs", async () => {
		renderOverview();

		const settings = await openProjectSettings();
		expect(within(settings).getByRole("button", { name: "web apps/web" })).toBeDefined();
		expect(within(settings).getByRole("button", { name: "api services/api" })).toBeDefined();
		expect(within(settings).getByRole("button", { name: "shared packages/shared" })).toBeDefined();
		expect(
			within(settings).getByRole("button", { name: "web apps/web" }).getAttribute("aria-current"),
		).toBe("page");
		expect(within(settings).queryByText("Manifest draft")).toBeNull();
		expect(within(settings).getByRole("tab", { name: "Overview" })).toBeDefined();
		expect(within(settings).getByRole("tab", { name: "Environment" })).toBeDefined();
		expect(within(settings).queryByRole("tab", { name: "Deploy" })).toBeNull();
		expect(within(settings).queryByRole("tab", { name: "Container" })).toBeNull();
	});

	it("removes the priority queue and local Profile notice from the Workspace page", async () => {
		const user = userEvent.setup();
		renderOverview({
			...overview,
			workspace: { ...overview.workspace!, domains: { env: "infisical" } },
		});

		expect(screen.queryByText("Resolve first")).toBeNull();
		expect(
			screen.queryByText(
				"Profiles stay in machine-local configuration; credentials never enter the manifest.",
			),
		).toBeNull();
		const dialog = await openWorkspaceSettingsDialog(user);
		await user.click(within(dialog).getByRole("tab", { name: "Infisical secrets" }));
		expect(within(dialog).getByRole("tabpanel", { name: "Infisical secrets" })).toBeDefined();
	});

	it("starts with the project inspector without the summary header or command card", async () => {
		renderOverview();
		expect(screen.queryByRole("heading", { level: 1, name: "demo" })).toBeNull();
		expect(screen.queryByText("/workspace/demo")).toBeNull();
		expect(screen.queryByText("one dev")).toBeNull();
		expect(await screen.findByRole("region", { name: "Project settings" })).toBeDefined();
	});

	it("uses environment-specific SWR keys for every workspace projection", () => {
		expect(overviewKeyFor("demo-entry", "dev")).toBe("/workspaces/demo-entry/overview?env=dev");
		expect(workspaceEnvironmentKey("demo-entry", "staging")).toBe(
			"/workspaces/demo-entry/environment?env=staging",
		);
		expect(projectSettingsKey("web app", "demo-entry", "prod")).toBe(
			"/workspaces/demo-entry/projects/web%20app?env=prod",
		);
		expect(projectSettingsKey("web", undefined, "dev")).not.toBe(
			projectSettingsKey("web", undefined, "prod"),
		);
	});

	it("shows Infisical without a backend switch", async () => {
		const user = userEvent.setup();
		renderOverview(overview, "demo-entry");
		const region = await openWorkspaceEnvironmentSettings(user);
		expect(within(region).getByText("Infisical")).toBeDefined();
		expect(within(region).queryByRole("combobox", { name: "Backend" })).toBeNull();
		expect(useManifestDraftStore.getState().drafts[manifestDraftKey("demo-entry")]).toBeUndefined();
	});

	it("shows project metadata without retired manifest editing fields", async () => {
		renderOverview();
		const inspector = await openProjectSettings();
		const command = (await within(inspector).findByLabelText("Build command")) as HTMLInputElement;
		expect(command.value).toBe("pnpm run build");
		expect(command.readOnly).toBe(true);
		expect(within(inspector).getByText("pnpm")).toBeDefined();
		expect(within(inspector).queryByLabelText("Build version")).toBeNull();
		expect(within(inspector).queryByLabelText("Development URL")).toBeNull();
		expect(useManifestDraftStore.getState().drafts[manifestDraftKey()]).toBeUndefined();
	});

	it.each([
		{ status: "missing" as const, placeholder: "No build task configured" },
		{ status: "invalid" as const, placeholder: "Unable to read build configuration" },
	])(
		"keeps project settings usable when the build task is $status",
		async ({ status, placeholder }) => {
			server.use(
				http.get("http://localhost/api/workspace/projects/web", () =>
					HttpResponse.json({
						...webSettings,
						project: {
							...webSettings.project,
							build: { source: "Taskfile.yml#tasks.build", status },
						},
					}),
				),
			);
			renderOverview();
			const inspector = await openProjectSettings();
			const command = (await within(inspector).findByLabelText(
				"Build command",
			)) as HTMLInputElement;
			expect(command.value).toBe("");
			expect(command.placeholder).toBe(placeholder);
			expect(command.readOnly).toBe(true);
			expect(within(inspector).getByText(/one build reads Taskfile.yml#tasks.build/)).toBeDefined();
			expect(within(inspector).queryByLabelText("Build version")).toBeNull();
		},
	);

	it("shows configured task dependencies and cache settings without claiming cache hits", async () => {
		server.use(
			http.get("http://localhost/api/workspace/projects/web", () =>
				HttpResponse.json({
					...webSettings,
					project: {
						...webSettings.project,
						tasks: {
							status: "ready",
							entries: [
								{
									name: "//apps/web:build",
									source: "apps/web/mise.toml",
									depends: ["//packages/lib:build"],
									outputs: ["dist"],
									cacheEnabled: true,
								},
							],
						},
					},
				}),
			),
		);
		renderOverview();
		const inspector = await openProjectSettings();
		const tasks = await within(inspector).findByRole("region", { name: "Configured tasks" });
		expect(within(tasks).getByText("//apps/web:build")).toBeDefined();
		expect(within(tasks).getByText("Dependencies: //packages/lib:build")).toBeDefined();
		expect(within(tasks).getByText("Artifact caching is configured.")).toBeDefined();
		expect(within(tasks).queryByRole("button")).toBeNull();
	});

	it("keeps project environment configuration separate from remote secret operations", async () => {
		const user = userEvent.setup();
		renderOverview();
		const inspector = await openProjectSettingsTab(user, "Environment");
		const backendConfig = within(inspector).getByTestId("environment-settings-grid");

		expect(within(backendConfig).queryByLabelText("Project profile")).toBeNull();
		expect(within(inspector).queryByRole("button", { name: "Save local binding" })).toBeNull();
	});
});
