import { render, screen, within } from "@testing-library/react";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { MemoryRouter } from "react-router-dom";
import { SWRConfig } from "swr";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";
import { WorkspaceHome } from "@/pages/WorkspaceHome";
import type { WorkspaceRegistryEntry, WorkspacesResponse } from "@/types/api";

const server = setupServer();

const workspaces: WorkspaceRegistryEntry[] = [
	{
		entryId: "alpha-entry",
		id: "alpha-a1b2c3",
		name: "Alpha",
		root: "/workspaces/alpha",
		status: "ready",
		projectCount: 3,
		lastSeenAt: "2026-08-30T09:00:00Z",
	},
	{
		entryId: "missing-entry",
		id: "missing-a1b2c3",
		name: "Missing",
		root: "/workspaces/missing",
		status: "missing",
		projectCount: 0,
		lastSeenAt: "2026-08-29T09:00:00Z",
	},
	{
		entryId: "invalid-entry",
		id: "invalid-a1b2c3",
		name: "Invalid",
		root: "/workspaces/invalid",
		status: "invalid",
		projectCount: 0,
		lastSeenAt: "2026-08-28T09:00:00Z",
	},
	{
		entryId: "identity-missing-entry",
		name: "Identity missing",
		root: "/workspaces/identity-missing",
		status: "identity-missing",
		projectCount: 1,
		lastSeenAt: "2026-08-27T09:00:00Z",
	},
	{
		entryId: "identity-conflict-entry",
		id: "shared-a1b2c3",
		name: "Identity conflict",
		root: "/workspaces/identity-conflict",
		status: "identity-conflict",
		projectCount: 4,
		lastSeenAt: "2026-08-26T09:00:00Z",
	},
];

function renderHome(path = "/") {
	return render(
		<SWRConfig
			value={{
				provider: () => new Map(),
				dedupingInterval: 10_000,
				shouldRetryOnError: false,
			}}
		>
			<MemoryRouter initialEntries={[path]}>
				<WorkspaceHome />
			</MemoryRouter>
		</SWRConfig>,
	);
}

describe("WorkspaceHome", () => {
	beforeAll(async () => {
		server.listen({ onUnhandledRequest: "error" });
		await i18n.changeLanguage("en-US");
	});

	afterEach(() => server.resetHandlers());
	afterAll(() => server.close());

	it("shows an explicit loading state while the registry is being read", () => {
		let releaseRequest = () => {};
		const pending = new Promise<void>((resolve) => {
			releaseRequest = resolve;
		});
		server.use(
			http.get("http://localhost/api/workspaces", async () => {
				await pending;
				return HttpResponse.json({ schema: "one-cli/workspaces/v1", workspaces: [] });
			}),
		);

		renderHome();

		expect(screen.getByRole("status").textContent).toContain("Loading Workspaces");
		releaseRequest();
	});

	it("shows every registered Workspace, its status, metadata, totals, and current marker", async () => {
		server.use(
			http.get("http://localhost/api/workspaces", () =>
				HttpResponse.json({
					schema: "one-cli/workspaces/v1",
					currentEntryId: "alpha-entry",
					workspaces,
				} satisfies WorkspacesResponse),
			),
		);

		renderHome("/?env=preview");

		expect(await screen.findByRole("heading", { name: "Workspaces" })).toBeDefined();
		expect(screen.getByText("5 registered Workspaces")).toBeDefined();
		expect(screen.getByText("8 Projects")).toBeDefined();

		const alpha = screen.getByRole("link", { name: /Alpha/ });
		expect(alpha.getAttribute("href")).toBe("/workspace/alpha-entry?env=preview");
		expect(within(alpha).getByText("alpha-a1b2c3")).toBeDefined();
		expect(within(alpha).getByText("/workspaces/alpha")).toBeDefined();
		expect(within(alpha).getByText("Ready")).toBeDefined();
		expect(within(alpha).getByText("3")).toBeDefined();
		expect(within(alpha).getByText("This one serve session")).toBeDefined();
		expect(within(alpha).getByText("Last detected")).toBeDefined();
		expect(alpha.querySelector('time[datetime="2026-08-30T09:00:00Z"]')).not.toBeNull();

		const expectedStatuses = [
			["Missing", "Path missing"],
			["Invalid", "Invalid manifest"],
			["Identity missing", "Workspace ID missing"],
			["Identity conflict", "Workspace ID conflict"],
		] as const;
		for (const [name, status] of expectedStatuses) {
			const card = screen.getByRole("link", { name: new RegExp(name) });
			expect(within(card).getByText(status)).toBeDefined();
		}
		expect(
			within(screen.getByRole("link", { name: /Missing/ })).getByText("Unavailable"),
		).toBeDefined();
		expect(
			within(screen.getByRole("link", { name: /Invalid/ })).getByText("Unavailable"),
		).toBeDefined();

		expect(
			within(screen.getByRole("link", { name: /Identity missing/ })).getByText("Not available"),
		).toBeDefined();
	});

	it("explains when the Workspace registry cannot be loaded", async () => {
		server.use(
			http.get("http://localhost/api/workspaces", () =>
				HttpResponse.json(
					{
						schema: "one-cli/error/v1",
						error: {
							code: "WORKSPACE_REGISTRY_READ_FAILED",
							message: "Registry offline",
							context: {},
							remediation: [],
						},
					},
					{ status: 500 },
				),
			),
		);

		renderHome();

		const alert = await screen.findByRole("alert");
		expect(within(alert).getByRole("heading", { name: "Could not load Workspaces" })).toBeDefined();
		expect(within(alert).getByText("Registry offline")).toBeDefined();
		expect(within(alert).getByRole("button", { name: "Retry" })).toBeDefined();
	});

	it("explains how to register the first Workspace", async () => {
		server.use(
			http.get("http://localhost/api/workspaces", () =>
				HttpResponse.json({
					schema: "one-cli/workspaces/v1",
					workspaces: [],
				} satisfies WorkspacesResponse),
			),
		);

		renderHome();

		expect(await screen.findByRole("heading", { name: "Workspaces" })).toBeDefined();
		expect(screen.getByRole("heading", { name: "No Workspaces yet" })).toBeDefined();
		expect(screen.getByText(/Run one create to create a Workspace/)).toBeDefined();
	});
});
