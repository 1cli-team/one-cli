import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { MemoryRouter, useLocation } from "react-router-dom";
import useSWR, { SWRConfig } from "swr";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import {
	getOverview,
	overviewKeyFor,
	projectProfileBindingKey,
	projectSettingsKey,
	workspaceProfileBindingKey,
} from "@/api/workspace";
import { environmentFromSearch } from "@/features/environment-context/environment";
import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import i18n from "@/lib/i18n";
import { Overview } from "@/pages/Overview";
import type {
	BackendDomain,
	BackendSpec,
	Overview as OverviewPayload,
	ProjectSettingsResponse,
} from "@/types/api";

const server = setupServer();

const catalogBackends: BackendSpec[] = [
	{
		id: "env/dotenv",
		domain: "env",
		name: "dotenv",
		capabilities: ["env-load"],
		profile: { configurable: false },
		project: { configurable: false },
	},
	{
		id: "env/infisical",
		domain: "env",
		name: "infisical",
		capabilities: ["env-load"],
		profile: { configurable: true, fields: [] },
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
		environments: ["dev", "preview", "prod"],
		domains: { env: "dotenv" },
	},
	projects: [
		{
			name: "web",
			relativeDir: "apps/web",
			kind: "app",
			templateId: "react-spa",
			toolchain: "node",
			domains: { env: "dotenv" },
		},
		{
			name: "api",
			relativeDir: "services/api",
			kind: "service",
			templateId: "go-api",
			toolchain: "go",
			domains: { env: "dotenv" },
		},
		{
			name: "shared",
			relativeDir: "packages/shared",
			kind: "package",
			templateId: "typescript-package",
			toolchain: "node",
			domains: { env: "dotenv" },
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
		templateId: "react-spa",
		toolchain: "node",
		packageManager: "pnpm",
		buildVersion: "1.0.0",
		devCommand: "pnpm dev",
		build: {
			command: "pnpm run build",
			source: "package.json#scripts.build",
			status: "ready",
		},
		availableEnvironments: ["dev", "preview", "prod"],
		environment: {
			backend: "infisical",
			path: ".env",
			inherits: true,
			disabled: false,
			keys: ["API_URL"],
			selectedProfile: "work",
			profile: { name: "work", source: "workspace-project-environment" },
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

async function chooseSelect(
	user: ReturnType<typeof userEvent.setup>,
	trigger: HTMLElement,
	optionName: string,
) {
	await user.click(trigger);
	await user.click(await screen.findByRole("option", { name: optionName }));
}

function expectSelectText(trigger: HTMLElement, value: string) {
	expect(trigger.textContent).toContain(value);
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

async function selectEnvironment(
	user: ReturnType<typeof userEvent.setup>,
	currentName: string,
	nextName: string,
) {
	const selector = screen.getByRole("combobox", { name: `Environment: ${currentName}` });
	await user.click(selector);
	await user.click(await screen.findByRole("option", { name: nextName }));
}

function sectionResponse(domain: BackendDomain, backend: string, profiles: string[]) {
	return {
		schema: "one-cli/serve-configure-section/v1",
		domain,
		backend,
		reveal: false,
		section: {
			default: profiles[0],
			profiles: Object.fromEntries(profiles.map((name) => [name, {}])),
		},
	};
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
			http.get("http://localhost/api/workspace/profile-bindings/env", ({ request }) =>
				HttpResponse.json({
					schema: "one-cli/workspace-profile/v1",
					root: "/workspace/demo",
					environment: new URL(request.url).searchParams.get("env") ?? "",
					domain: "env",
					backend: "dotenv",
					configurable: false,
					selectedProfile: "",
				}),
			),
			http.get("http://localhost/api/workspaces/:entryId/profile-bindings/env", ({ request }) =>
				HttpResponse.json({
					schema: "one-cli/workspace-profile/v1",
					root: "/workspace/demo",
					environment: new URL(request.url).searchParams.get("env") ?? "",
					domain: "env",
					backend: "dotenv",
					configurable: false,
					selectedProfile: "",
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

	it("shows projects in a persistent left-hand list with project settings tabs", async () => {
		renderOverview();

		const settings = await openProjectSettings();
		expect(within(settings).getByRole("button", { name: "web apps/web" })).toBeDefined();
		expect(within(settings).getByRole("button", { name: "api services/api" })).toBeDefined();
		expect(within(settings).getByRole("button", { name: "shared packages/shared" })).toBeDefined();
		expect(
			within(settings).getByRole("button", { name: "web apps/web" }).getAttribute("aria-current"),
		).toBe("page");
		expect(await within(settings).findByText("Manifest draft")).toBeDefined();
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
		expect(workspaceProfileBindingKey("demo-entry", "preview")).toBe(
			"/workspaces/demo-entry/profile-bindings/env?env=preview",
		);
		expect(projectSettingsKey("web app", "demo-entry", "prod")).toBe(
			"/workspaces/demo-entry/projects/web%20app?env=prod",
		);
		expect(projectProfileBindingKey("web", "env", undefined, "dev")).toBe(
			"/workspace/projects/web/profile-bindings/env?env=dev",
		);
		expect(projectSettingsKey("web", undefined, "dev")).not.toBe(
			projectSettingsKey("web", undefined, "prod"),
		);
	});

	it("exposes the workspace backend selector and saves Profile bindings separately", async () => {
		let requestBody: unknown;
		let receivedEnvironment = "";
		let legacyWrites = 0;
		let overviewRequests = 0;
		const configurableOverview: OverviewPayload = {
			...overview,
			workspace: {
				...overview.workspace!,
				domains: { ...overview.workspace?.domains, env: "infisical" },
			},
			issues: [
				{
					domain: "env",
					severity: "missing",
					reason: "profile",
					backend: "infisical",
					section: "env/infisical",
					message: "Infisical credentials are missing",
				},
			],
		};
		server.use(
			http.get("http://localhost/api/workspaces/demo-entry/overview", ({ request }) => {
				overviewRequests += 1;
				expect(new URL(request.url).searchParams.get("env")).toBe("dev");
				return HttpResponse.json({ ...configurableOverview, issues: [] });
			}),
			http.get("http://localhost/api/workspaces/demo-entry/profile-bindings/env", () =>
				HttpResponse.json({
					schema: "one-cli/workspace-profile/v1",
					root: "/workspace/demo",
					environment: "dev",
					domain: "env",
					backend: "infisical",
					configurable: true,
					selectedProfile: "",
					profile: { name: "work", source: "default" },
				}),
			),
			http.get("http://localhost/api/configure/env/infisical", () =>
				HttpResponse.json(sectionResponse("env", "infisical", ["work", "personal"])),
			),
			http.put(
				"http://localhost/api/workspaces/demo-entry/profile-bindings/env",
				async ({ request }) => {
					requestBody = await request.json();
					const url = new URL(request.url);
					receivedEnvironment = url.searchParams.get("env") ?? "";
					return HttpResponse.json({
						schema: "one-cli/workspace-profile/v1",
						root: "/workspace/demo",
						environment: "dev",
						domain: "env",
						backend: "infisical",
						configurable: true,
						selectedProfile: "personal",
						profile: { name: "personal", source: "workspace-environment" },
					});
				},
			),
			http.put("http://localhost/api/workspaces/demo-entry/domains/env", () => {
				legacyWrites += 1;
				return HttpResponse.json(configurableOverview);
			}),
		);
		const user = userEvent.setup();
		renderOverview(configurableOverview, "demo-entry", false, true);

		const region = await openWorkspaceEnvironmentSettings(user);
		const backendSettings = within(region).getByTestId("workspace-backend-settings");
		expect(within(backendSettings).getByRole("combobox", { name: "Backend" })).toBeDefined();
		const profile = await within(backendSettings).findByRole("combobox", { name: "Profile" });
		await chooseSelect(user, profile, "personal");

		await waitFor(() => expect(requestBody).toEqual({ profile: "personal" }));
		expect(receivedEnvironment).toBe("dev");
		expect(legacyWrites).toBe(0);
		await waitFor(() => expect(overviewRequests).toBe(1));
	});

	it("stages a Workspace env backend change for Manifest review", async () => {
		let backendWrites = 0;
		const configurableOverview: OverviewPayload = {
			...overview,
			workspace: {
				...overview.workspace!,
				domains: { ...overview.workspace?.domains, env: "infisical" },
			},
		};
		server.use(
			http.get("http://localhost/api/workspaces/demo-entry/profile-bindings/env", () =>
				HttpResponse.json({
					schema: "one-cli/workspace-profile/v1",
					root: "/workspace/demo",
					environment: "dev",
					revision: "sha256:test-revision",
					domain: "env",
					backend: "infisical",
					configurable: true,
					selectedProfile: "",
					profile: { name: "work", source: "default" },
				}),
			),
			http.get("http://localhost/api/configure/env/infisical", () =>
				HttpResponse.json(sectionResponse("env", "infisical", ["work"])),
			),
			http.put("http://localhost/api/workspaces/demo-entry/manifest", () => {
				backendWrites += 1;
				return HttpResponse.json({
					schema: "one-cli/workspace-manifest-apply/v1",
					revision: "sha256:next",
					applied: 1,
				});
			}),
		);
		const user = userEvent.setup();
		renderOverview(configurableOverview, "demo-entry");

		const region = await openWorkspaceEnvironmentSettings(user);
		await chooseSelect(user, within(region).getByRole("combobox", { name: "Backend" }), "Dotenv");

		expect(useManifestDraftStore.getState().drafts[manifestDraftKey("demo-entry")]).toMatchObject({
			revision: "sha256:test-revision",
			workspace: { environment: { backend: "dotenv" } },
		});
		expect(within(region).getByText("Pending review")).toBeDefined();
		expect(backendWrites).toBe(0);
	});

	it("unbinds a direct workspace Profile with an explicit empty value", async () => {
		let requestBody: unknown;
		const configurableOverview: OverviewPayload = {
			...overview,
			workspace: {
				...overview.workspace!,
				domains: { ...overview.workspace?.domains, env: "infisical" },
			},
		};
		server.use(
			http.get("http://localhost/api/workspace/profile-bindings/env", () =>
				HttpResponse.json({
					schema: "one-cli/workspace-profile/v1",
					root: "/workspace/demo",
					environment: "dev",
					domain: "env",
					backend: "infisical",
					configurable: true,
					selectedProfile: "personal",
					profile: { name: "personal", source: "workspace-environment" },
				}),
			),
			http.get("http://localhost/api/configure/env/infisical", () =>
				HttpResponse.json(sectionResponse("env", "infisical", ["work", "personal"])),
			),
			http.put("http://localhost/api/workspace/profile-bindings/env", async ({ request }) => {
				requestBody = await request.json();
				return HttpResponse.json({
					schema: "one-cli/workspace-profile/v1",
					root: "/workspace/demo",
					environment: "dev",
					domain: "env",
					backend: "infisical",
					configurable: true,
					selectedProfile: "",
					profile: { name: "work", source: "default" },
				});
			}),
		);
		const user = userEvent.setup();
		renderOverview(configurableOverview);

		const region = await openWorkspaceEnvironmentSettings(user);
		const profile = await within(region).findByRole("combobox", { name: "Profile" });
		await waitFor(() => expectSelectText(profile, "personal"));
		await chooseSelect(user, profile, "Resolve automatically (machine default)");
		await waitFor(() => expect(requestBody).toEqual({ profile: "" }));
	});

	it("auto-saves a Workspace Profile before changing environment", async () => {
		const requestedEnvironments: string[] = [];
		let requestBody: unknown;
		const configurableOverview: OverviewPayload = {
			...overview,
			workspace: {
				...overview.workspace!,
				domains: { ...overview.workspace?.domains, env: "infisical" },
			},
		};
		server.use(
			http.get("http://localhost/api/workspace/profile-bindings/env", ({ request }) => {
				const selectedEnvironment = new URL(request.url).searchParams.get("env") ?? "";
				requestedEnvironments.push(selectedEnvironment);
				const selectedProfile = selectedEnvironment === "preview" ? "preview-base" : "work";
				return HttpResponse.json({
					schema: "one-cli/workspace-profile/v1",
					root: "/workspace/demo",
					environment: selectedEnvironment,
					domain: "env",
					backend: "infisical",
					configurable: true,
					selectedProfile,
					profile: { name: selectedProfile, source: "workspace-environment" },
				});
			}),
			http.get("http://localhost/api/configure/env/infisical", () =>
				HttpResponse.json(
					sectionResponse("env", "infisical", ["work", "personal", "preview-base"]),
				),
			),
			http.put("http://localhost/api/workspace/profile-bindings/env", async ({ request }) => {
				requestBody = await request.json();
				return HttpResponse.json({
					schema: "one-cli/workspace-profile/v1",
					root: "/workspace/demo",
					environment: "dev",
					domain: "env",
					backend: "infisical",
					configurable: true,
					selectedProfile: "personal",
					profile: { name: "personal", source: "workspace-environment" },
				});
			}),
		);
		const user = userEvent.setup();
		renderOverview(configurableOverview);
		const region = await openWorkspaceEnvironmentSettings(user);
		const profile = await within(region).findByRole("combobox", { name: "Profile" });
		await chooseSelect(user, profile, "personal");
		await waitFor(() => expect(requestBody).toEqual({ profile: "personal" }));
		expectSelectText(profile, "personal");

		const dialog = screen.getByRole("dialog", { name: "Workspace settings" });
		await user.click(within(dialog).getByRole("button", { name: "Close" }));
		await selectEnvironment(user, "Development", "Preview");

		await waitFor(() =>
			expect(screen.getByTestId("environment-search").textContent).toBe("?env=preview"),
		);
		const previewRegion = await openWorkspaceEnvironmentSettings(user);
		await waitFor(() => expect(requestedEnvironments).toContain("preview"));
		await waitFor(() =>
			expectSelectText(
				within(previewRegion).getByRole("combobox", { name: "Profile" }),
				"preview-base",
			),
		);
	});

	it("keeps workspace Profile selection disabled for an identity conflict", async () => {
		const configurableOverview: OverviewPayload = {
			...overview,
			workspace: {
				...overview.workspace!,
				domains: { ...overview.workspace?.domains, env: "infisical" },
			},
		};
		server.use(
			http.get("http://localhost/api/workspaces/demo-entry/profile-bindings/env", () =>
				HttpResponse.json({
					schema: "one-cli/workspace-profile/v1",
					root: "/workspace/demo",
					environment: "dev",
					domain: "env",
					backend: "infisical",
					configurable: true,
					selectedProfile: "",
					profile: { name: "work", source: "default" },
				}),
			),
			http.get("http://localhost/api/configure/env/infisical", () =>
				HttpResponse.json(sectionResponse("env", "infisical", ["work"])),
			),
		);
		const user = userEvent.setup();
		renderOverview(configurableOverview, "demo-entry", true);

		const region = await openWorkspaceEnvironmentSettings(user);
		expect(
			(within(region).getByRole("combobox", { name: "Profile" }) as HTMLButtonElement).disabled,
		).toBe(true);
		expect(
			(within(region).getByRole("combobox", { name: "Backend" }) as HTMLButtonElement).disabled,
		).toBe(true);
		expect(within(region).queryByRole("button", { name: "Save local binding" })).toBeNull();
	});

	it("explains when the workspace backend does not use Profiles", async () => {
		const user = userEvent.setup();
		renderOverview();
		const region = await openWorkspaceEnvironmentSettings(user);
		expect(
			within(region).getByText("This backend does not require a credential profile."),
		).toBeDefined();
		expect(within(region).queryByRole("combobox", { name: "Profile" })).toBeNull();
	});

	it("keeps identity fields read-only and stages editable General manifest fields", async () => {
		let receivedEnvironment = "";
		server.use(
			http.get("http://localhost/api/workspace/projects/web", ({ request }) => {
				receivedEnvironment = new URL(request.url).searchParams.get("env") ?? "";
				return HttpResponse.json(webSettings);
			}),
		);
		renderOverview();
		const inspector = await openProjectSettings();

		expect(await within(inspector).findByText("Manifest draft")).toBeDefined();
		expect((within(inspector).getByLabelText("Build version") as HTMLInputElement).value).toBe(
			"1.0.0",
		);
		expect(within(inspector).getByText("pnpm")).toBeDefined();
		expect(
			(within(inspector).getByLabelText("Development command") as HTMLInputElement).value,
		).toBe("pnpm dev");
		expect(within(inspector).queryByLabelText("Package manager")).toBeNull();
		const buildCommand = within(inspector).getByLabelText("Build command") as HTMLInputElement;
		expect(buildCommand.value).toBe("pnpm run build");
		expect(buildCommand.readOnly).toBe(true);
		expect(within(inspector).getByText(/one build reads package.json#scripts.build/)).toBeDefined();
		const user = userEvent.setup();
		await user.clear(within(inspector).getByLabelText("Build version"));
		await user.type(within(inspector).getByLabelText("Build version"), "2.0.0");
		expect(
			useManifestDraftStore.getState().drafts[manifestDraftKey()]?.changes.web?.general,
		).toEqual({ buildVersion: "2.0.0", devCommand: "pnpm dev" });
		expect(within(inspector).queryByRole("button", { name: "Save local binding" })).toBeNull();
		expect(receivedEnvironment).toBe("dev");
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
			expect(
				(within(inspector).getByLabelText("Development command") as HTMLInputElement).disabled,
			).toBe(false);
		},
	);

	it("keeps project Environment settings manifest-only", async () => {
		const user = userEvent.setup();
		renderOverview();
		const inspector = await openProjectSettingsTab(user, "Environment");
		const backendConfig = within(inspector).getByTestId("environment-settings-grid");

		expect(within(backendConfig).queryByLabelText("Project profile")).toBeNull();
		expect(within(inspector).queryByRole("button", { name: "Save local binding" })).toBeNull();
	});
});
