import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

beforeAll(async () => {
	server.listen({ onUnhandledRequest: "error" });
	await i18n.changeLanguage("en-US");
});

afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe("WorkspaceHome", () => {
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

	it("shows every registered Workspace with project totals and detection timestamps", async () => {
		server.use(
			http.get("http://localhost/api/workspaces", () =>
				HttpResponse.json({
					schema: "one-cli/workspaces/v1",
					currentEntryId: "alpha-entry",
					workspaces,
				} satisfies WorkspacesResponse),
			),
		);

		renderHome("/?env=staging");

		expect(await screen.findByRole("heading", { name: "Workspaces" })).toBeDefined();

		const alpha = screen.getByRole("link", { name: /Alpha/ });
		expect(alpha.getAttribute("href")).toBe("/workspace/alpha-entry?env=staging");
		expect(within(alpha).getByText("Projects")).toBeDefined();
		expect(within(alpha).getByText("3")).toBeDefined();
		expect(within(alpha).getByText(/^Last detected ·/)).toBeDefined();
		expect(alpha.querySelector('time[datetime="2026-08-30T09:00:00Z"]')).not.toBeNull();

		for (const name of ["Missing", "Invalid", "Identity missing", "Identity conflict"]) {
			expect(screen.getByRole("link", { name: new RegExp(name) })).toBeDefined();
		}
		expect(within(screen.getByRole("link", { name: /Missing/ })).getByText("-")).toBeDefined();
		expect(within(screen.getByRole("link", { name: /Invalid/ })).getByText("-")).toBeDefined();
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

describe("Workspace discovery and recovery", () => {
	function serveRegistry() {
		server.use(
			http.get("http://localhost/api/workspaces", () =>
				HttpResponse.json({ schema: "one-cli/workspaces/v1", workspaces }),
			),
		);
	}

	// This multi-step jsdom interaction shares CI CPUs with the cold Go build.
	it("combines path search with attention filtering and restores the list on clear", async () => {
		serveRegistry();
		const user = userEvent.setup();
		renderHome("/?env=staging");
		await screen.findByRole("link", { name: /Alpha/ });
		const search = screen.getByRole("textbox", { name: "Search name, path or ID…" });
		// The case/whitespace filter assertion only needs the final query. Pasting
		// avoids a full workspace-card render for every character on CI runners.
		await user.click(search);
		await user.paste(" /WORKSPACES/ALPHA ");
		expect(screen.getAllByRole("article")).toHaveLength(1);
		expect(screen.getByRole("link", { name: /Alpha/ }).getAttribute("href")).toBe(
			"/workspace/alpha-entry?env=staging",
		);
		await user.click(screen.getByRole("button", { name: /Needs attention/ }));
		expect(screen.getByText("No matching workspaces")).toBeDefined();
		await user.click(screen.getByRole("button", { name: "Clear filters" }));
		expect(document.activeElement).toBe(search);
		expect(screen.getAllByRole("article")).toHaveLength(5);
		await user.click(screen.getByRole("button", { name: /Needs attention/ }));
		expect(screen.getAllByRole("article")).toHaveLength(4);
		expect(screen.queryByRole("link", { name: /Alpha/ })).toBeNull();
	}, 10_000);

	it("keeps existing workspaces during a failed refresh and supports retry", async () => {
		serveRegistry();
		const user = userEvent.setup();
		renderHome();
		await screen.findByRole("link", { name: /Alpha/ });
		server.use(
			http.get("http://localhost/api/workspaces", () =>
				HttpResponse.json({ error: { message: "Registry offline" } }, { status: 500 }),
			),
		);
		await user.click(screen.getByRole("button", { name: "Refresh" }));
		const alert = await screen.findByRole("alert");
		expect(screen.getAllByRole("article")).toHaveLength(5);
		serveRegistry();
		await user.click(within(alert).getByRole("button", { name: "Retry" }));
		await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
		expect(screen.getAllByRole("article")).toHaveLength(5);
	});

	it("keeps a failed removal open and removes only the selected workspace on retry", async () => {
		serveRegistry();
		const user = userEvent.setup();
		renderHome();
		await screen.findByRole("link", { name: /Alpha/ });
		server.use(
			http.delete("http://localhost/api/workspaces/alpha-entry", () =>
				HttpResponse.json({ error: { message: "Registry is busy" } }, { status: 500 }),
			),
		);
		await user.click(screen.getByRole("button", { name: "Remove Alpha" }));
		const dialog = await screen.findByRole("alertdialog");
		await user.click(within(dialog).getByRole("button", { name: "Remove Alpha" }));
		expect((await within(dialog).findByRole("alert")).textContent).toContain("Registry is busy");
		expect(screen.getAllByRole("article", { hidden: true })).toHaveLength(5);
		server.use(
			http.delete(
				"http://localhost/api/workspaces/alpha-entry",
				() => new HttpResponse(null, { status: 204 }),
			),
		);
		await user.click(within(dialog).getByRole("button", { name: "Remove Alpha" }));
		await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
		expect(screen.queryByRole("link", { name: /Alpha/ })).toBeNull();
		expect(screen.getAllByRole("article")).toHaveLength(4);
	});
});
