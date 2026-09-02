import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import type React from "react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { SWRConfig } from "swr";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import i18n from "@/lib/i18n";
import { AppRoutes } from "@/router/routes";
import type {
	BackendSpec,
	Overview,
	WorkspaceRegistryEntry,
	WorkspacesResponse,
} from "@/types/api";

const server = setupServer();

const alpha: WorkspaceRegistryEntry = {
	entryId: "alpha-entry",
	id: "alpha-a1b2c3",
	name: "Alpha",
	root: "/workspaces/alpha",
	status: "ready",
	projectCount: 1,
	lastSeenAt: "2026-08-29T09:00:00Z",
};

const beta: WorkspaceRegistryEntry = {
	entryId: "beta-entry",
	id: "beta-d4e5f6",
	name: "Beta",
	root: "/workspaces/beta",
	status: "ready",
	projectCount: 2,
	lastSeenAt: "2026-08-30T09:00:00Z",
};

const broken: WorkspaceRegistryEntry = {
	entryId: "broken-entry",
	id: "broken-aabbcc",
	name: "Broken",
	root: "/workspaces/missing",
	status: "missing",
	projectCount: 0,
	lastSeenAt: "2026-08-28T09:00:00Z",
};

const infisicalBackend: BackendSpec = {
	id: "env/infisical",
	domain: "env",
	name: "infisical",
	capabilities: ["env-load"],
	profile: { configurable: true, fields: [] },
	project: { configurable: false },
};

function workspaceOverview(workspace: WorkspaceRegistryEntry): Overview {
	return {
		schema: "one-cli/workspace-overview/v1",
		present: true,
		root: workspace.root,
		workspace: {
			id: workspace.id,
			name: workspace.name,
			manifestVersion: 1,
			environments: ["dev"],
			defaultEnvironment: "dev",
			domains: { env: "dotenv" },
		},
		projects: [
			{
				name: `${workspace.name.toLowerCase()}-web`,
				relativeDir: "apps/web",
				kind: "app",
				templateId: "react-spa",
				toolchain: "node",
				domains: { env: "dotenv" },
			},
		],
	};
}

const LocationProbe: React.FC = () => {
	const location = useLocation();
	return (
		<>
			<output data-testid="location">{location.pathname}</output>
			<output data-testid="location-search">{location.search}</output>
		</>
	);
};

function renderDashboard(path = "/") {
	return render(
		<SWRConfig value={{ provider: () => new Map(), dedupingInterval: 10_000 }}>
			<MemoryRouter initialEntries={[path]}>
				<main data-testid="route-content">
					<AppRoutes />
				</main>
				<LocationProbe />
			</MemoryRouter>
		</SWRConfig>,
	);
}

function registerCatalogHandler() {
	server.use(
		http.get("http://localhost/api/catalog", () =>
			HttpResponse.json({ schema: "one-cli/catalog/v1", backends: [] }),
		),
		http.get(
			"http://localhost/api/workspaces/:entryId/profile-bindings/env",
			({ params, request }) => {
				const entry = params.entryId === beta.entryId ? beta : alpha;
				return HttpResponse.json({
					schema: "one-cli/workspace-profile/v1",
					root: entry.root,
					environment: new URL(request.url).searchParams.get("env") ?? "dev",
					domain: "env",
					backend: "dotenv",
					configurable: false,
					selectedProfile: "",
				});
			},
		),
		http.get(
			"http://localhost/api/workspaces/:entryId/projects/:projectName",
			({ params, request }) => {
				const entry = params.entryId === beta.entryId ? beta : alpha;
				const projectName = String(params.projectName);
				return HttpResponse.json({
					schema: "one-cli/workspace-project/v1",
					root: entry.root,
					environment: new URL(request.url).searchParams.get("env") ?? "dev",
					revision: "sha256:route-test",
					project: {
						name: projectName,
						relativeDir: "apps/web",
						kind: "app",
						templateId: "react-spa",
						toolchain: "node",
						packageManager: "pnpm",
						buildVersion: "1.0.0",
						devCommand: "pnpm dev",
						availableEnvironments: ["dev", "preview", "prod"],
						environment: {
							backend: "dotenv",
							path: ".env",
							inherits: true,
							disabled: false,
							keys: [],
						},
						container: { enabled: false },
						deploy: { compatibleTargets: [] },
					},
				});
			},
		),
	);
}

function registerSettingsHandlers() {
	server.use(
		http.get("http://localhost/api/workspaces", () =>
			HttpResponse.json({
				schema: "one-cli/workspaces/v1",
				workspaces: [],
			} satisfies WorkspacesResponse),
		),
		http.get("http://localhost/api/catalog", () =>
			HttpResponse.json({
				schema: "one-cli/catalog/v1",
				backends: [infisicalBackend],
			}),
		),
		http.get("http://localhost/api/configure", () =>
			HttpResponse.json({
				schema: "one-cli/serve-configure-config/v1",
				config_path: "/machine/config.json",
				credentials_path: "/machine/credentials.json",
				reveal: false,
				config: { version: 1 },
			}),
		),
		http.get("http://localhost/api/configure/env/infisical", () =>
			HttpResponse.json({
				schema: "one-cli/serve-configure-section/v1",
				domain: "env",
				backend: "infisical",
				reveal: false,
				section: { profiles: {} },
			}),
		),
	);
}

describe("multi-workspace routing", () => {
	beforeAll(async () => {
		server.listen({ onUnhandledRequest: "error" });
		await i18n.changeLanguage("en-US");
	});
	afterEach(() => {
		server.resetHandlers();
		vi.restoreAllMocks();
	});
	afterAll(() => server.close());

	it("lists every workspace at the root and opens a selected card without losing env", async () => {
		let registryRequests = 0;
		let alphaOverviewRequests = 0;
		const response: WorkspacesResponse = {
			schema: "one-cli/workspaces/v1",
			currentEntryId: beta.entryId,
			workspaces: [alpha, beta],
		};
		server.use(
			http.get("http://localhost/api/workspaces", () => {
				registryRequests += 1;
				return HttpResponse.json(response);
			}),
			http.get("http://localhost/api/workspaces/alpha-entry/overview", () => {
				alphaOverviewRequests += 1;
				return HttpResponse.json(workspaceOverview(alpha));
			}),
		);
		registerCatalogHandler();
		const user = userEvent.setup();

		renderDashboard("/?env=prod");

		const content = within(screen.getByTestId("route-content"));
		expect(await content.findByRole("heading", { name: "Workspaces" })).toBeDefined();
		expect(content.getByRole("link", { name: /Alpha/ })).toBeDefined();
		expect(content.getByRole("link", { name: /Beta/ })).toBeDefined();
		expect(screen.getByTestId("location").textContent).toBe("/");
		expect(screen.getByTestId("location-search").textContent).toBe("?env=prod");
		expect(alphaOverviewRequests).toBe(0);

		await user.click(content.getByRole("link", { name: /Alpha/ }));

		expect(await content.findByRole("region", { name: "Project settings" })).toBeDefined();
		expect(content.getByRole("button", { name: "alpha-web apps/web" })).toBeDefined();
		await waitFor(() =>
			expect(screen.getByTestId("location").textContent).toBe("/workspace/alpha-entry"),
		);
		expect(screen.getByTestId("location-search").textContent).toBe("?env=prod");
		expect(registryRequests).toBe(1);
		expect(alphaOverviewRequests).toBe(1);
	});

	it("keeps an empty registry on the Workspace home instead of showing Settings", async () => {
		server.use(
			http.get("http://localhost/api/workspaces", () =>
				HttpResponse.json({
					schema: "one-cli/workspaces/v1",
					workspaces: [],
				} satisfies WorkspacesResponse),
			),
		);

		renderDashboard("/?env=preview");

		const content = within(screen.getByTestId("route-content"));
		expect(await content.findByRole("heading", { name: "Workspaces" })).toBeDefined();
		expect(content.getByRole("heading", { name: "No Workspaces yet" })).toBeDefined();
		expect(content.queryByRole("heading", { name: "Settings" })).toBeNull();
		expect(screen.getByTestId("location").textContent).toBe("/");
		expect(screen.getByTestId("location-search").textContent).toBe("?env=preview");
	});

	it("switches the scoped overview and forgets only an unavailable registry entry", async () => {
		let deletedEntry = "";
		server.use(
			http.get("http://localhost/api/workspaces", () =>
				HttpResponse.json({
					schema: "one-cli/workspaces/v1",
					currentEntryId: alpha.entryId,
					workspaces: [alpha, beta, broken].filter(
						(workspace) => workspace.entryId !== deletedEntry,
					),
				} satisfies WorkspacesResponse),
			),
			http.get("http://localhost/api/workspaces/alpha-entry/overview", () =>
				HttpResponse.json(workspaceOverview(alpha)),
			),
			http.get("http://localhost/api/workspaces/beta-entry/overview", () =>
				HttpResponse.json(workspaceOverview(beta)),
			),
			http.delete("http://localhost/api/workspaces/:entryId", ({ params }) => {
				deletedEntry = String(params.entryId);
				return new HttpResponse(null, { status: 204 });
			}),
		);
		registerCatalogHandler();
		const user = userEvent.setup();

		renderDashboard("/workspace/alpha-entry");
		expect(await screen.findByRole("region", { name: "Project settings" })).toBeDefined();

		await user.click(screen.getByRole("link", { name: "Home" }));
		await user.click(await screen.findByRole("link", { name: /Beta/ }));
		await waitFor(() =>
			expect(screen.getByTestId("location").textContent).toBe("/workspace/beta-entry"),
		);
		expect(await screen.findByRole("button", { name: "beta-web apps/web" })).toBeDefined();

		await user.click(screen.getByRole("link", { name: "Home" }));
		await user.click(await screen.findByRole("button", { name: "Remove Broken" }));
		const confirmation = await screen.findByRole("alertdialog");
		expect(
			within(confirmation).getByText(
				'Remove Workspace "Broken"? This only removes the local registry entry; no project files or Profiles will be deleted.',
			),
		).toBeDefined();
		await user.click(within(confirmation).getByRole("button", { name: "Remove Broken" }));
		await waitFor(() => expect(deletedEntry).toBe("broken-entry"));
		expect(screen.queryByRole("link", { name: /Broken/ })).toBeNull();
	});

	it("keeps duplicated workspace identities visible but read-only", async () => {
		const duplicate = { ...alpha, status: "identity-conflict" as const };
		server.use(
			http.get("http://localhost/api/workspaces", () =>
				HttpResponse.json({
					schema: "one-cli/workspaces/v1",
					currentEntryId: duplicate.entryId,
					workspaces: [duplicate],
				} satisfies WorkspacesResponse),
			),
			http.get("http://localhost/api/workspaces/alpha-entry/overview", () =>
				HttpResponse.json(workspaceOverview(duplicate)),
			),
		);
		registerCatalogHandler();
		const user = userEvent.setup();

		renderDashboard("/workspace/alpha-entry");

		expect(
			await screen.findByText("Read-only: this Workspace ID is used by another path"),
		).toBeDefined();
		const inspector = screen.getByRole("region", { name: "Project settings" });
		await user.click(within(inspector).getByRole("button", { name: "Workspace settings" }));
		const dialog = await screen.findByRole("dialog", { name: "Workspace settings" });
		expect(
			(within(dialog).getByRole("combobox", { name: "Backend" }) as HTMLButtonElement).disabled,
		).toBe(true);
	});

	it("opens machine profile management at the Settings route", async () => {
		registerSettingsHandlers();

		renderDashboard("/settings");

		expect(await screen.findByRole("heading", { name: "Settings" })).toBeDefined();
		expect(screen.getByTestId("location").textContent).toBe("/settings");
		expect(screen.queryByText("env/infisical")).toBeNull();
	});

	it("redirects the legacy Profile route to Settings", async () => {
		registerSettingsHandlers();

		renderDashboard("/profile?env=prod");

		await waitFor(() => expect(screen.getByTestId("location").textContent).toBe("/settings"));
		expect(screen.getByTestId("location-search").textContent).toBe("?env=prod");
		expect(await screen.findByRole("heading", { name: "Settings" })).toBeDefined();
	});

	it("redirects legacy section URLs to the corresponding Settings backend", async () => {
		registerSettingsHandlers();

		renderDashboard("/section/env/infisical?env=preview");

		await waitFor(() =>
			expect(screen.getByTestId("location").textContent).toBe("/settings/env/infisical"),
		);
		expect(screen.getByTestId("location-search").textContent).toBe("?env=preview");
		expect(await screen.findByRole("heading", { name: "Infisical" })).toBeDefined();
	});

	it("confirms a destructive Profile removal before calling the API", async () => {
		let deletedProfile = "";
		registerSettingsHandlers();
		server.use(
			http.get("http://localhost/api/configure/env/infisical", () =>
				HttpResponse.json({
					schema: "one-cli/serve-configure-section/v1",
					domain: "env",
					backend: "infisical",
					reveal: false,
					section: { default: "work", profiles: { work: {} } },
				}),
			),
			http.delete("http://localhost/api/configure/env/infisical/:name", ({ params }) => {
				deletedProfile = String(params.name);
				return HttpResponse.json({
					schema: "one-cli/serve-configure-remove/v1",
					status: "removed",
					name: deletedProfile,
				});
			}),
		);
		const user = userEvent.setup();

		renderDashboard("/settings/env/infisical");
		expect(await screen.findByText("work")).toBeDefined();

		await user.click(screen.getByRole("button", { name: "Delete" }));
		const confirmation = await screen.findByRole("alertdialog");
		expect(
			within(confirmation).getByText('Delete profile "work"? This cannot be undone.'),
		).toBeDefined();
		expect(deletedProfile).toBe("");

		await user.click(within(confirmation).getByRole("button", { name: "Delete" }));
		await waitFor(() => expect(deletedProfile).toBe("work"));
	});
});
