import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { SWRConfig } from "swr";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
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

function mockUnconfiguredStorage() {
	const requests = { initializations: 0, saves: 0, lists: 0 };
	server.use(
		http.get("http://localhost/api/workspaces/demo-entry/secrets", () => {
			requests.lists++;
			if (!requests.initializations)
				return HttpResponse.json(
					{ error: { code: "INFISICAL_NOT_CONFIGURED", message: "Storage is not connected." } },
					{ status: 409 },
				);
			return HttpResponse.json({
				schema: "one-cli/env-list/v1",
				env: "dev",
				path: "/",
				keys: requests.saves ? ["API_TOKEN"] : [],
				total: requests.saves,
			});
		}),
		http.post("http://localhost/api/workspaces/demo-entry/environment/backend/initialize", () => {
			requests.initializations++;
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
		}),
		http.post("http://localhost/api/workspaces/demo-entry/secrets", async ({ request }) => {
			expect(requests.initializations).toBe(1);
			expect(await request.json()).toEqual({ key: "API_TOKEN", value: "secret-value" });
			requests.saves++;
			return HttpResponse.json({ action: "created", key: "API_TOKEN" }, { status: 201 });
		}),
	);
	return requests;
}

describe("Infisical secrets manager", () => {
	beforeAll(async () => {
		server.listen({ onUnhandledRequest: "error" });
		await i18n.changeLanguage("en-US");
	});
	afterEach(async () => {
		server.resetHandlers();
		await i18n.changeLanguage("en-US");
	});
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

	describe.each(["en-US", "zh-CN"])("unconfigured storage (%s)", (locale) => {
		let requests: ReturnType<typeof mockUnconfiguredStorage>;
		let user: ReturnType<typeof userEvent.setup>;

		beforeEach(async () => {
			await i18n.changeLanguage(locale);
			requests = mockUnconfiguredStorage();
			user = userEvent.setup();
			renderManager();
			expect(await screen.findByText(i18n.t("secrets.notConfiguredTitle"))).toBeDefined();
			expect(requests.initializations).toBe(0);
		});

		it("retries the list without initializing storage", async () => {
			await user.click(screen.getByRole("button", { name: i18n.t("secrets.retry") }));
			await waitFor(() => expect(requests.lists).toBeGreaterThan(1));
			expect(requests.initializations).toBe(0);
			expect(requests.saves).toBe(0);
		});

		it("cancels the editor without initializing storage", async () => {
			await user.click(screen.getByRole("button", { name: i18n.t("secrets.add") }));
			await user.click(
				within(await screen.findByRole("dialog")).getByRole("button", {
					name: i18n.t("form.cancel"),
				}),
			);
			await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
			expect(requests.initializations).toBe(0);
			expect(requests.saves).toBe(0);
		});

		it("rejects an invalid key before initializing storage", async () => {
			await user.click(screen.getByRole("button", { name: i18n.t("secrets.add") }));
			const dialog = within(await screen.findByRole("dialog"));
			await user.click(dialog.getByLabelText(i18n.t("secrets.key")));
			await user.paste("1INVALID");
			await user.click(dialog.getByRole("button", { name: i18n.t("secrets.save") }));
			expect(await dialog.findByText(i18n.t("secrets.invalidKey"))).toBeDefined();
			expect(requests.initializations).toBe(0);
			expect(requests.saves).toBe(0);
		});

		it("initializes storage once before saving a valid secret", async () => {
			await user.click(screen.getByRole("button", { name: i18n.t("secrets.add") }));
			const dialog = within(await screen.findByRole("dialog"));
			await user.click(dialog.getByLabelText(i18n.t("secrets.key")));
			await user.paste("API_TOKEN");
			await user.click(dialog.getByLabelText(i18n.t("secrets.value")));
			await user.paste("secret-value");
			expect(requests.initializations).toBe(0);
			await user.click(dialog.getByRole("button", { name: i18n.t("secrets.save") }));
			await waitFor(() => expect(requests.saves).toBe(1));
			expect(requests.initializations).toBe(1);
			await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
		});
	});
});
