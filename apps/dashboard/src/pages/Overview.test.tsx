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
import { EnvironmentSelector } from "@/features/environment-context/EnvironmentSelector";
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
	{
		id: "container/docker",
		domain: "container",
		name: "docker",
		capabilities: ["container-build"],
		profile: { configurable: true, fields: [] },
		project: { configurable: false },
	},
	{
		id: "deploy/kustomize",
		domain: "deploy",
		name: "kustomize",
		capabilities: ["deploy"],
		requirements: [
			{ kind: "capability", name: "container/build" },
			{ kind: "capability", name: "container/push" },
		],
		profile: { configurable: true, fields: [] },
		project: { configurable: true, fields: [] },
	},
	{
		id: "deploy/vercel",
		domain: "deploy",
		name: "vercel",
		capabilities: ["deploy"],
		profile: { configurable: true, fields: [] },
		project: {
			configurable: true,
			fields: [
				{
					path: "projectName",
					input_name: "project-name",
					type: "string",
					label_key: "project.fields.projectName",
				},
				{
					path: "env",
					input_name: "environment",
					type: "environment",
					label_key: "project.fields.environment",
				},
			],
		},
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
			compatibleDeployTargets: ["vercel"],
			domains: { env: "dotenv", container: "docker", deploy: "vercel" },
		},
		{
			name: "api",
			relativeDir: "services/api",
			kind: "service",
			templateId: "go-api",
			toolchain: "go",
			compatibleDeployTargets: ["kustomize"],
			domains: { env: "dotenv", container: "docker", deploy: "kustomize" },
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
		container: {
			enabled: true,
			backend: "docker",
			image: "ghcr.io/one/web:latest",
			namespace: "one",
			selectedProfile: "registry-main",
			profile: { name: "registry-main", source: "workspace-project-environment" },
		},
		deploy: {
			backend: "vercel",
			compatibleTargets: ["vercel"],
			config: { projectName: "old-web", env: "dev" },
			selectedProfile: "production",
			profile: { name: "production", source: "workspace-project-environment" },
		},
	},
};

const apiSettings: ProjectSettingsResponse = {
	...webSettings,
	project: {
		...webSettings.project,
		name: "api",
		relativeDir: "services/api",
		kind: "service",
		templateId: "go-api",
		toolchain: "go",
		packageManager: undefined,
		deploy: {
			backend: "kustomize",
			compatibleTargets: ["kustomize"],
			config: { environment: "dev" },
			selectedProfile: "cluster-main",
			profile: { name: "cluster-main", source: "workspace-project-environment" },
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
			<EnvironmentSelector />
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

async function openProjectSettings(user: ReturnType<typeof userEvent.setup>) {
	await user.click(screen.getByRole("tab", { name: /^Projects/ }));
	return screen.findByRole("region", { name: "Project settings" });
}

async function openProjectSettingsTab(
	user: ReturnType<typeof userEvent.setup>,
	tabName: "Environment" | "Deploy",
) {
	const settings = await openProjectSettings(user);
	await user.click(within(settings).getByRole("tab", { name: tabName }));
	return settings;
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

	it("uses three Workspace tabs and shows projects in a left-hand list", async () => {
		const user = userEvent.setup();
		renderOverview();

		expect(screen.getByRole("tab", { name: "Workspace environment" })).toBeDefined();
		expect(screen.getByRole("tab", { name: /^Projects/ })).toBeDefined();
		expect(screen.getByRole("tab", { name: "Infisical secrets" })).toBeDefined();
		expect(screen.getByRole("region", { name: "Workspace environment" })).toBeDefined();

		const settings = await openProjectSettings(user);
		expect(within(settings).getByRole("button", { name: "web apps/web" })).toBeDefined();
		expect(within(settings).getByRole("button", { name: "api services/api" })).toBeDefined();
		expect(within(settings).getByRole("button", { name: "shared packages/shared" })).toBeDefined();
		expect(
			within(settings).getByRole("button", { name: "web apps/web" }).getAttribute("aria-current"),
		).toBe("page");
		expect(await within(settings).findByText("Manifest draft")).toBeDefined();
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
		await user.click(screen.getByRole("tab", { name: "Infisical secrets" }));
		expect(screen.getByRole("tabpanel", { name: "Infisical secrets" })).toBeDefined();
	});

	it("starts with the Workspace tabs without the summary header or command card", () => {
		renderOverview();
		expect(screen.queryByRole("heading", { level: 1, name: "demo" })).toBeNull();
		expect(screen.queryByText("/workspace/demo")).toBeNull();
		expect(screen.queryByText("one dev")).toBeNull();
		expect(screen.getByRole("tab", { name: "Workspace environment" })).toBeDefined();
	});

	it("uses environment-specific SWR keys for every workspace projection", () => {
		expect(overviewKeyFor("demo-entry", "dev")).toBe("/workspaces/demo-entry/overview?env=dev");
		expect(workspaceProfileBindingKey("demo-entry", "preview")).toBe(
			"/workspaces/demo-entry/profile-bindings/env?env=preview",
		);
		expect(projectSettingsKey("web app", "demo-entry", "prod")).toBe(
			"/workspaces/demo-entry/projects/web%20app?env=prod",
		);
		expect(projectProfileBindingKey("web", "deploy", undefined, "dev")).toBe(
			"/workspace/projects/web/profile-bindings/deploy?env=dev",
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

		const region = await screen.findByRole("region", { name: "Workspace environment" });
		const backendSettings = within(region).getByTestId("workspace-backend-settings");
		expect(within(backendSettings).getByRole("combobox", { name: "Backend" })).toBeDefined();
		const profile = await within(backendSettings).findByRole("combobox", { name: "Profile" });
		await chooseSelect(user, profile, "personal");
		await user.click(within(region).getByRole("button", { name: "Save local binding" }));

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

		const region = await screen.findByRole("region", { name: "Workspace environment" });
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

		const region = await screen.findByRole("region", { name: "Workspace environment" });
		const profile = await within(region).findByRole("combobox", { name: "Profile" });
		await waitFor(() => expectSelectText(profile, "personal"));
		await chooseSelect(user, profile, "Resolve automatically (machine default)");
		await user.click(within(region).getByRole("button", { name: "Save local binding" }));
		await waitFor(() => expect(requestBody).toEqual({ profile: "" }));
	});

	it("guards an unsaved Workspace Profile selection before changing environment", async () => {
		const requestedEnvironments: string[] = [];
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
		);
		const user = userEvent.setup();
		renderOverview(configurableOverview);
		const region = await screen.findByRole("region", { name: "Workspace environment" });
		const profile = await within(region).findByRole("combobox", { name: "Profile" });
		await chooseSelect(user, profile, "personal");

		await user.click(screen.getByRole("radio", { name: "Preview" }));
		let discardDialog = await screen.findByRole("alertdialog");
		expect(screen.getByTestId("environment-search").textContent).toBe("?env=dev");
		expectSelectText(profile, "personal");

		await user.click(within(discardDialog).getByRole("button", { name: "Cancel" }));
		await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
		expect(screen.getByTestId("environment-search").textContent).toBe("?env=dev");
		expectSelectText(within(region).getByRole("combobox", { name: "Profile" }), "personal");

		await user.click(screen.getByRole("radio", { name: "Preview" }));
		discardDialog = await screen.findByRole("alertdialog");
		await user.click(within(discardDialog).getByRole("button", { name: "Discard and continue" }));

		await waitFor(() =>
			expect(screen.getByTestId("environment-search").textContent).toBe("?env=preview"),
		);
		await waitFor(() => expect(requestedEnvironments).toContain("preview"));
		await waitFor(() =>
			expectSelectText(screen.getByRole("combobox", { name: "Profile" }), "preview-base"),
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
		renderOverview(configurableOverview, "demo-entry", true);

		const region = await screen.findByRole("region", { name: "Workspace environment" });
		expect(
			(within(region).getByRole("combobox", { name: "Profile" }) as HTMLButtonElement).disabled,
		).toBe(true);
		expect(
			(
				within(region).getByRole("button", {
					name: "Save local binding",
				}) as HTMLButtonElement
			).disabled,
		).toBe(true);
	});

	it("explains when the workspace backend does not use Profiles", async () => {
		renderOverview();
		const region = await screen.findByRole("region", { name: "Workspace environment" });
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
		const user = userEvent.setup();
		renderOverview();
		const inspector = await openProjectSettings(user);

		expect(await within(inspector).findByText("Manifest draft")).toBeDefined();
		expect((within(inspector).getByLabelText("Build version") as HTMLInputElement).value).toBe(
			"1.0.0",
		);
		expect(within(inspector).getByText("pnpm")).toBeDefined();
		expect(
			(within(inspector).getByLabelText("Development command") as HTMLInputElement).value,
		).toBe("pnpm dev");
		expect(within(inspector).queryByLabelText("Package manager")).toBeNull();
		expect(within(inspector).queryByRole("button", { name: "Save local binding" })).toBeNull();
		expect(receivedEnvironment).toBe("dev");
	});

	it.each([
		{
			domain: "env" as const,
			tabName: "Environment" as const,
			backend: "infisical",
			initial: "work",
			next: "personal",
			legacyPath: "/api/workspace/projects/web/environment",
		},
		{
			domain: "deploy" as const,
			tabName: "Deploy" as const,
			backend: "vercel",
			initial: "production",
			next: "preview-team",
			legacyPath: "/api/workspace/projects/web/settings/deploy",
		},
	])("saves only {profile} through the $domain binding endpoint", async (testCase) => {
		let requestBody: unknown;
		let receivedEnvironment = "";
		let legacyWrites = 0;
		server.use(
			http.get("http://localhost/api/workspace/projects/web", () => HttpResponse.json(webSettings)),
			http.get(`http://localhost/api/configure/${testCase.domain}/${testCase.backend}`, () =>
				HttpResponse.json(
					sectionResponse(testCase.domain, testCase.backend, [testCase.initial, testCase.next]),
				),
			),
			http.put(
				`http://localhost/api/workspace/projects/web/profile-bindings/${testCase.domain}`,
				async ({ request }) => {
					requestBody = await request.json();
					const url = new URL(request.url);
					receivedEnvironment = url.searchParams.get("env") ?? "";
					return HttpResponse.json({
						...webSettings,
						project: {
							...webSettings.project,
							[testCase.domain === "env" ? "environment" : testCase.domain]: {
								...webSettings.project[testCase.domain === "env" ? "environment" : testCase.domain],
								selectedProfile: testCase.next,
								profile: {
									name: testCase.next,
									source: "workspace-project-environment",
								},
							},
						},
					});
				},
			),
			http.put(`http://localhost${testCase.legacyPath}`, () => {
				legacyWrites += 1;
				return HttpResponse.json(webSettings);
			}),
		);
		const user = userEvent.setup();
		renderOverview();
		const inspector = await openProjectSettingsTab(user, testCase.tabName);
		const backendConfig = within(inspector).getByTestId(
			testCase.domain === "env" ? "environment-settings-grid" : "deployment-settings-grid",
		);
		expect(
			within(inspector).queryByText(`Inherited ${testCase.domain} backend`, { exact: false }),
		).toBeNull();
		const profile = await within(backendConfig).findByLabelText("Project profile");
		expectSelectText(profile, testCase.initial);
		await chooseSelect(user, profile, testCase.next);
		await user.click(within(inspector).getByRole("button", { name: "Save local binding" }));

		await waitFor(() => expect(requestBody).toEqual({ profile: testCase.next }));
		expect(Object.keys(requestBody as Record<string, unknown>)).toEqual(["profile"]);
		expect(receivedEnvironment).toBe("dev");
		expect(legacyWrites).toBe(0);
		if (testCase.domain === "deploy") {
			expect((within(inspector).getByLabelText("Project name") as HTMLInputElement).value).toBe(
				"old-web",
			);
		}
	});

	it("hides image configuration when the deploy backend does not require an image", async () => {
		server.use(
			http.get("http://localhost/api/workspace/projects/web", () => HttpResponse.json(webSettings)),
			http.get("http://localhost/api/configure/deploy/vercel", () =>
				HttpResponse.json(sectionResponse("deploy", "vercel", ["production"])),
			),
		);
		const user = userEvent.setup();
		renderOverview();
		const inspector = await openProjectSettingsTab(user, "Deploy");

		expect(
			await within(inspector).findByRole("form", { name: "Deployment configuration" }),
		).toBeDefined();
		expect(within(inspector).queryByRole("form", { name: "Image configuration" })).toBeNull();
	});

	it("shows image configuration for image-based deployment and saves its registry Profile", async () => {
		let requestBody: unknown;
		let receivedEnvironment = "";
		server.use(
			http.get("http://localhost/api/workspace/projects/api", () => HttpResponse.json(apiSettings)),
			http.get("http://localhost/api/configure/deploy/kustomize", () =>
				HttpResponse.json(sectionResponse("deploy", "kustomize", ["cluster-main"])),
			),
			http.get("http://localhost/api/configure/container/docker", () =>
				HttpResponse.json(
					sectionResponse("container", "docker", ["registry-main", "registry-backup"]),
				),
			),
			http.put(
				"http://localhost/api/workspace/projects/api/profile-bindings/container",
				async ({ request }) => {
					requestBody = await request.json();
					receivedEnvironment = new URL(request.url).searchParams.get("env") ?? "";
					return HttpResponse.json({
						...apiSettings,
						project: {
							...apiSettings.project,
							container: {
								...apiSettings.project.container,
								selectedProfile: "registry-backup",
								profile: {
									name: "registry-backup",
									source: "workspace-project-environment",
								},
							},
						},
					});
				},
			),
		);
		const user = userEvent.setup();
		renderOverview();
		const inspector = await openProjectSettings(user);
		await user.click(within(inspector).getByRole("button", { name: "api services/api" }));
		await user.click(within(inspector).getByRole("tab", { name: "Deploy" }));

		const imageForm = await within(inspector).findByRole("form", {
			name: "Image configuration",
		});
		expect(
			within(imageForm).queryByText("Inherited container backend", { exact: false }),
		).toBeNull();
		const backendConfig = within(imageForm).getByTestId("image-settings-grid");
		const profile = await within(backendConfig).findByLabelText("Project profile");
		expectSelectText(profile, "registry-main");
		await chooseSelect(user, profile, "registry-backup");
		await user.click(within(imageForm).getByRole("button", { name: "Save local binding" }));

		await waitFor(() => expect(requestBody).toEqual({ profile: "registry-backup" }));
		expect(receivedEnvironment).toBe("dev");
	});

	it("reveals image configuration when the staged deploy backend starts requiring it", async () => {
		const switchableSettings: ProjectSettingsResponse = {
			...webSettings,
			project: {
				...webSettings.project,
				deploy: {
					...webSettings.project.deploy,
					compatibleTargets: ["vercel", "kustomize"],
				},
			},
		};
		server.use(
			http.get("http://localhost/api/workspace/projects/web", () =>
				HttpResponse.json(switchableSettings),
			),
			http.get("http://localhost/api/configure/deploy/vercel", () =>
				HttpResponse.json(sectionResponse("deploy", "vercel", ["production"])),
			),
			http.get("http://localhost/api/configure/deploy/kustomize", () =>
				HttpResponse.json(sectionResponse("deploy", "kustomize", ["cluster-main"])),
			),
			http.get("http://localhost/api/configure/container/docker", () =>
				HttpResponse.json(sectionResponse("container", "docker", ["registry-main"])),
			),
		);
		const user = userEvent.setup();
		renderOverview();
		const inspector = await openProjectSettingsTab(user, "Deploy");
		const deploymentForm = await within(inspector).findByRole("form", {
			name: "Deployment configuration",
		});
		const backendConfig = within(deploymentForm).getByTestId("deployment-settings-grid");
		expect(within(inspector).queryByRole("form", { name: "Image configuration" })).toBeNull();
		expectSelectText(within(backendConfig).getByLabelText("Project profile"), "production");

		await chooseSelect(user, within(deploymentForm).getByLabelText("Backend"), "Kustomize");

		expect(
			await within(inspector).findByRole("form", { name: "Image configuration" }),
		).toBeDefined();
		const profile = within(backendConfig).getByLabelText("Project profile");
		expectSelectText(profile, "Resolve automatically (workspace / default)");
		await chooseSelect(user, profile, "cluster-main");
		expect(
			within(deploymentForm).getByText(
				"Save the pending Manifest change before binding a Profile to the selected Backend.",
			),
		).toBeDefined();
		expect(
			(
				within(deploymentForm).getByRole("button", {
					name: "Save local binding",
				}) as HTMLButtonElement
			).disabled,
		).toBe(true);
	});

	it("shows a stale Profile selection and can return it to Automatic", async () => {
		let requestBody: unknown;
		const staleSettings: ProjectSettingsResponse = {
			...webSettings,
			project: {
				...webSettings.project,
				deploy: {
					...webSettings.project.deploy,
					selectedProfile: "deleted-profile",
					profile: undefined,
				},
			},
		};
		server.use(
			http.get("http://localhost/api/workspace/projects/web", () =>
				HttpResponse.json(staleSettings),
			),
			http.get("http://localhost/api/configure/deploy/vercel", () =>
				HttpResponse.json(sectionResponse("deploy", "vercel", ["production"])),
			),
			http.put(
				"http://localhost/api/workspace/projects/web/profile-bindings/deploy",
				async ({ request }) => {
					requestBody = await request.json();
					return HttpResponse.json({
						...staleSettings,
						project: {
							...staleSettings.project,
							deploy: {
								...staleSettings.project.deploy,
								selectedProfile: "",
								profile: { name: "production", source: "default" },
							},
						},
					});
				},
			),
		);
		const user = userEvent.setup();
		renderOverview();
		const inspector = await openProjectSettingsTab(user, "Deploy");
		const profile = await within(inspector).findByLabelText("Project profile");
		expectSelectText(profile, "deleted-profile");

		await chooseSelect(user, profile, "Resolve automatically (workspace / default)");
		await user.click(within(inspector).getByRole("button", { name: "Save local binding" }));

		await waitFor(() => expect(requestBody).toEqual({ profile: "" }));
		await waitFor(() =>
			expectSelectText(within(inspector).getByLabelText("Project profile"), "production"),
		);
	});

	it("preserves the dirty guard for an unsaved Profile selection", async () => {
		server.use(
			http.get("http://localhost/api/workspace/projects/web", () => HttpResponse.json(webSettings)),
			http.get("http://localhost/api/configure/deploy/vercel", () =>
				HttpResponse.json(sectionResponse("deploy", "vercel", ["production", "preview-team"])),
			),
		);
		const user = userEvent.setup();
		renderOverview();
		const inspector = await openProjectSettingsTab(user, "Deploy");
		const profile = await within(inspector).findByLabelText("Project profile");
		await chooseSelect(user, profile, "preview-team");

		await user.click(within(inspector).getByRole("tab", { name: "Overview" }));
		let discardDialog = await screen.findByRole("alertdialog");
		expect(within(inspector).getByLabelText("Project profile")).toBeDefined();
		await user.click(within(discardDialog).getByRole("button", { name: "Cancel" }));
		await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());

		await user.click(within(inspector).getByRole("button", { name: "api services/api" }));
		discardDialog = await screen.findByRole("alertdialog");
		expect(inspector.isConnected).toBe(true);
		await user.click(within(discardDialog).getByRole("button", { name: "Discard and continue" }));
		await waitFor(() =>
			expect(
				within(inspector)
					.getByRole("button", { name: "api services/api" })
					.getAttribute("aria-current"),
			).toBe("page"),
		);
	});

	it("does not change environment until an unsaved Profile selection is discarded", async () => {
		const requestedEnvironments: string[] = [];
		server.use(
			http.get("http://localhost/api/workspace/projects/web", ({ request }) => {
				requestedEnvironments.push(new URL(request.url).searchParams.get("env") ?? "");
				return HttpResponse.json(webSettings);
			}),
			http.get("http://localhost/api/configure/deploy/vercel", () =>
				HttpResponse.json(sectionResponse("deploy", "vercel", ["production", "preview-team"])),
			),
		);
		const user = userEvent.setup();
		renderOverview();
		const inspector = await openProjectSettingsTab(user, "Deploy");
		const profile = await within(inspector).findByLabelText("Project profile");
		await chooseSelect(user, profile, "preview-team");

		await user.click(screen.getByRole("radio", { name: "Preview" }));
		let discardDialog = await screen.findByRole("alertdialog");
		expect(screen.getByTestId("environment-search").textContent).toBe("?env=dev");
		expectSelectText(within(inspector).getByLabelText("Project profile"), "preview-team");

		await user.click(within(discardDialog).getByRole("button", { name: "Cancel" }));
		await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
		expect(screen.getByTestId("environment-search").textContent).toBe("?env=dev");

		await user.click(screen.getByRole("radio", { name: "Preview" }));
		discardDialog = await screen.findByRole("alertdialog");
		await user.click(within(discardDialog).getByRole("button", { name: "Discard and continue" }));

		await waitFor(() =>
			expect(screen.getByTestId("environment-search").textContent).toBe("?env=preview"),
		);
		await waitFor(() => expect(requestedEnvironments).toContain("preview"));
	});

	it("scopes project reads and empty Profile writes to workspace and preview environment", async () => {
		let requestBody: unknown;
		let readEnvironment = "";
		let writeEnvironment = "";
		server.use(
			http.get("http://localhost/api/workspaces/demo-entry/projects/web", ({ request }) => {
				readEnvironment = new URL(request.url).searchParams.get("env") ?? "";
				return HttpResponse.json({ ...webSettings, environment: "preview" });
			}),
			http.get("http://localhost/api/configure/deploy/vercel", () =>
				HttpResponse.json(sectionResponse("deploy", "vercel", ["production"])),
			),
			http.put(
				"http://localhost/api/workspaces/demo-entry/projects/web/profile-bindings/deploy",
				async ({ request }) => {
					requestBody = await request.json();
					writeEnvironment = new URL(request.url).searchParams.get("env") ?? "";
					return HttpResponse.json({
						...webSettings,
						environment: "preview",
						project: {
							...webSettings.project,
							deploy: {
								...webSettings.project.deploy,
								selectedProfile: "",
								profile: { name: "production", source: "default" },
							},
						},
					});
				},
			),
		);
		const user = userEvent.setup();
		renderOverview(overview, "demo-entry", false, false, "preview");
		const inspector = await openProjectSettingsTab(user, "Deploy");
		const profile = await within(inspector).findByLabelText("Project profile");
		await chooseSelect(user, profile, "Resolve automatically (workspace / default)");
		await user.click(within(inspector).getByRole("button", { name: "Save local binding" }));

		await waitFor(() => expect(requestBody).toEqual({ profile: "" }));
		expect(readEnvironment).toBe("preview");
		expect(writeEnvironment).toBe("preview");
	});
});
