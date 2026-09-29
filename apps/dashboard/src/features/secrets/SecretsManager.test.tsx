import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { SWRConfig } from "swr";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";
import { SecretsManager } from "@/features/secrets/SecretsManager";
import i18n from "@/lib/i18n";

const server = setupServer(
	http.get("http://localhost/api/session", () =>
		HttpResponse.json({ session: { loggedIn: true, expired: false } }),
	),
);

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

	it.each(["en-US", "zh-CN"])(
		"initializes only on save, not on retry or cancel (%s)",
		async (locale) => {
			await i18n.changeLanguage(locale);
			let initializations = 0;
			let saves = 0;
			let lists = 0;
			server.use(
				http.get("http://localhost/api/workspaces/demo-entry/secrets", () => {
					lists++;
					if (!initializations)
						return HttpResponse.json(
							{ error: { code: "INFISICAL_NOT_CONFIGURED", message: "Storage is not connected." } },
							{ status: 409 },
						);
					return HttpResponse.json({
						schema: "one-cli/env-list/v1",
						env: "dev",
						path: "/",
						keys: saves ? ["API_TOKEN"] : [],
						total: saves,
					});
				}),
				http.post(
					"http://localhost/api/workspaces/demo-entry/environment/backend/initialize",
					() => {
						initializations++;
						return HttpResponse.json({
							schema: "one-cli/workspace-environment/v1",
							backend: "infisical",
							projectId: "new-id",
							projectName: "demo-a3f2",
							binding: {
								project_id: "new-id",
								project_name: "demo-a3f2",
								created: true,
								requested_name: "demo",
							},
						});
					},
				),
				http.post("http://localhost/api/workspaces/demo-entry/secrets", () => {
					expect(initializations).toBe(1);
					saves++;
					return HttpResponse.json({ action: "created", key: "API_TOKEN" }, { status: 201 });
				}),
			);
			const user = userEvent.setup();
			renderManager();
			expect(await screen.findByText(i18n.t("secrets.notConfiguredTitle"))).toBeDefined();
			expect(initializations).toBe(0);
			await user.click(screen.getByRole("button", { name: i18n.t("secrets.retry") }));
			await waitFor(() => expect(lists).toBeGreaterThan(1));
			expect(initializations).toBe(0);
			await user.click(screen.getByRole("button", { name: i18n.t("secrets.add") }));
			await user.click(
				within(await screen.findByRole("dialog")).getByRole("button", {
					name: i18n.t("form.cancel"),
				}),
			);
			expect(initializations).toBe(0);
			await user.click(screen.getByRole("button", { name: i18n.t("secrets.add") }));
			const dialog = within(await screen.findByRole("dialog"));
			await user.type(dialog.getByLabelText(i18n.t("secrets.key")), "1INVALID");
			await user.click(dialog.getByRole("button", { name: i18n.t("secrets.save") }));
			expect(await dialog.findByText(i18n.t("secrets.invalidKey"))).toBeDefined();
			expect(initializations).toBe(0);
			expect(saves).toBe(0);
			await user.clear(dialog.getByLabelText(i18n.t("secrets.key")));
			await user.type(dialog.getByLabelText(i18n.t("secrets.key")), "API_TOKEN");
			await user.type(dialog.getByLabelText(i18n.t("secrets.value")), "secret-value");
			await user.click(dialog.getByRole("button", { name: i18n.t("secrets.save") }));
			await waitFor(() => expect(saves).toBe(1));
			expect(initializations).toBe(1);
			await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
			await i18n.changeLanguage("en-US");
		},
	);
});
