import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { MemoryRouter } from "react-router-dom";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";
import { Overview } from "@/pages/Overview";
import type { Overview as OverviewPayload } from "@/types/api";

const server = setupServer();

describe("workspace overview", () => {
	beforeAll(async () => {
		server.listen({ onUnhandledRequest: "error" });
		await i18n.changeLanguage("en-US");
	});
	beforeEach(() => {
		server.use(
			http.get("http://localhost/api/catalog", () =>
				HttpResponse.json({ schema: "one-cli/catalog/v1", backends: [] }),
			),
		);
	});
	afterEach(() => server.resetHandlers());
	afterAll(() => server.close());

	it("offers only deployment targets compatible with the project's technology stack", async () => {
		const user = userEvent.setup();
		const data = {
			schema: "one-cli/workspace-overview/v1",
			present: true,
			root: "/workspace/demo",
			workspace: { id: "demo", name: "demo", manifestVersion: 1 },
			projects: [
				{
					name: "api",
					relativeDir: "services/api",
					kind: "service",
					templateId: "go-api",
					toolchain: "go",
					compatibleDeployTargets: ["kustomize"],
					issues: [
						{
							domain: "deploy",
							severity: "missing",
							reason: "backend",
							message: "deploy target is not configured",
						},
					],
				},
			],
		} as OverviewPayload;

		render(
			<MemoryRouter>
				<Overview data={data} />
			</MemoryRouter>,
		);

		expect(screen.getByRole("heading", { name: "demo" })).toBeDefined();
		expect(screen.getByText("api")).toBeDefined();
		await user.click(screen.getByRole("button", { name: "deploy" }));

		const dialog = await screen.findByRole("dialog");
		const select = within(dialog).getByLabelText("Backend kind");
		const choices = within(select)
			.getAllByRole("option")
			.map((option) => option.getAttribute("value"));
		expect(choices).toContain("kustomize");
		expect(choices).not.toContain("vercel");
	});

	it("saves a compatible deployment target through the workspace HTTP API", async () => {
		let receivedKind = "";
		let receivedToken = "";
		server.use(
			http.put("http://localhost/api/workspace/projects/api/deploy", async ({ request }) => {
				const body = (await request.json()) as { kind?: string };
				receivedKind = body.kind ?? "";
				receivedToken = new URL(request.url).searchParams.get("token") ?? "";
				return HttpResponse.json({
					schema: "one-cli/workspace-overview/v1",
					present: true,
					workspace: { id: "demo", name: "demo", manifestVersion: 1 },
					projects: [],
				});
			}),
		);
		const user = userEvent.setup();
		const data = {
			schema: "one-cli/workspace-overview/v1",
			present: true,
			workspace: { id: "demo", name: "demo", manifestVersion: 1 },
			projects: [
				{
					name: "api",
					relativeDir: "services/api",
					kind: "service",
					templateId: "go-api",
					compatibleDeployTargets: ["kustomize"],
					issues: [
						{
							domain: "deploy",
							severity: "missing",
							reason: "backend",
							message: "deploy target is not configured",
						},
					],
				},
			],
		} as OverviewPayload;

		render(
			<MemoryRouter>
				<Overview data={data} />
			</MemoryRouter>,
		);

		await user.click(screen.getByRole("button", { name: "deploy" }));
		const dialog = await screen.findByRole("dialog");
		await user.selectOptions(within(dialog).getByLabelText("Backend kind"), "kustomize");
		await user.click(within(dialog).getByRole("button", { name: "Save" }));

		await waitFor(() => expect(receivedKind).toBe("kustomize"));
		expect(receivedToken).toBe("test-token");
	});
});
