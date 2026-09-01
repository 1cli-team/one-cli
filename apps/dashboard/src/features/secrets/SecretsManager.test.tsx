import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { SWRConfig } from "swr";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { SecretsManager } from "@/features/secrets/SecretsManager";
import i18n from "@/lib/i18n";

const server = setupServer();

function renderManager() {
	return render(
		<SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
			<SecretsManager
				workspaceEntryId="demo-entry"
				environment="dev"
				projects={[{ name: "web", relativeDir: "apps/web", kind: "app" }]}
			/>
		</SWRConfig>,
	);
}

describe("Infisical secrets manager", () => {
	beforeAll(async () => {
		server.listen({ onUnhandledRequest: "error" });
		await i18n.changeLanguage("en-US");
	});
	afterEach(() => server.resetHandlers());
	afterAll(() => server.close());

	it("lists names without values and reveals only one requested secret", async () => {
		let reveals = 0;
		server.use(
			http.get("http://localhost/api/workspaces/demo-entry/secrets", ({ request }) => {
				const url = new URL(request.url);
				expect(url.searchParams.get("env")).toBe("dev");
				return HttpResponse.json({
					schema: "one-cli/env-list/v1",
					env: "dev",
					path: "/",
					keys: ["API_TOKEN", "DATABASE_URL"],
					total: 2,
				});
			}),
			http.get("http://localhost/api/workspaces/demo-entry/secrets/API_TOKEN", () => {
				reveals += 1;
				return HttpResponse.json({
					schema: "one-cli/env-get/v1",
					env: "dev",
					path: "/",
					key: "API_TOKEN",
					value: "top-secret",
				});
			}),
		);
		const user = userEvent.setup();
		renderManager();

		const row = within(await screen.findByText("API_TOKEN").then((node) => node.closest("tr")!));
		expect(row.getByText("••••••••••••")).toBeDefined();
		expect(screen.queryByText("top-secret")).toBeNull();
		await user.click(row.getByRole("button", { name: "Reveal value" }));
		expect(await row.findByText("top-secret")).toBeDefined();
		expect(reveals).toBe(1);
	});

	it("creates a secret in the selected scope without touching a manifest API", async () => {
		let requestBody: unknown;
		server.use(
			http.get("http://localhost/api/workspaces/demo-entry/secrets", () =>
				HttpResponse.json({
					schema: "one-cli/env-list/v1",
					env: "dev",
					path: "/",
					keys: [],
					total: 0,
				}),
			),
			http.post("http://localhost/api/workspaces/demo-entry/secrets", async ({ request }) => {
				requestBody = await request.json();
				return HttpResponse.json(
					{
						schema: "one-cli/env-set/v1",
						env: "dev",
						path: "/",
						key: "API_TOKEN",
						action: "created",
					},
					{ status: 201 },
				);
			}),
		);
		const user = userEvent.setup();
		renderManager();
		await user.click(await screen.findByRole("button", { name: "Add secret" }));
		const dialog = await screen.findByRole("dialog");
		await user.type(within(dialog).getByLabelText("Key"), "api_token");
		await user.type(within(dialog).getByLabelText("Value"), "secret-value");
		await user.click(within(dialog).getByRole("button", { name: "Save secret" }));

		await waitFor(() => expect(requestBody).toEqual({ key: "API_TOKEN", value: "secret-value" }));
	});

	it("repairs an older missing Infisical binding before retrying the list", async () => {
		let initialized = false;
		server.use(
			http.get("http://localhost/api/workspaces/demo-entry/secrets", () => {
				if (!initialized) {
					return HttpResponse.json(
						{
							schema: "one-cli/error/v1",
							error: {
								code: "INFISICAL_NOT_CONFIGURED",
								message: "Infisical project binding is missing.",
								context: {},
								remediation: [],
							},
						},
						{ status: 409 },
					);
				}
				return HttpResponse.json({
					schema: "one-cli/env-list/v1",
					env: "dev",
					path: "/",
					keys: [],
					total: 0,
				});
			}),
			http.post(
				"http://localhost/api/workspaces/demo-entry/environment/backend/initialize",
				({ request }) => {
					const url = new URL(request.url);
					expect(url.searchParams.get("env")).toBe("dev");
					initialized = true;
					return HttpResponse.json({
						schema: "one-cli/workspace-profile/v1",
						root: "/workspace/demo",
						environment: "dev",
						revision: "sha256:repaired",
						domain: "env",
						backend: "infisical",
						configurable: true,
					});
				},
			),
		);
		const user = userEvent.setup();
		renderManager();

		expect(await screen.findByText("Infisical project binding is missing.")).toBeDefined();
		await user.click(screen.getByRole("button", { name: "Retry" }));
		expect(
			await screen.findByText("No secrets are defined directly in this scope yet."),
		).toBeDefined();
		expect(initialized).toBe(true);
	});
});
