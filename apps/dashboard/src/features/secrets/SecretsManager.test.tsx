import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { SWRConfig } from "swr";
import { MemoryRouter } from "react-router-dom";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { SecretsManager } from "@/features/secrets/SecretsManager";
import i18n from "@/lib/i18n";
import { useManifestDraftStore } from "@/features/manifest-draft/manifest-draft-store";

const server = setupServer(
	http.get("http://localhost/api/session", () =>
		HttpResponse.json({
			session: {
				loggedIn: true,
				expired: false,
				siteUrl: "https://secrets.example.com",
				userId: "user",
				organizationId: "org",
			},
		}),
	),
);

function renderManager() {
	return render(
		<MemoryRouter>
			<SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
				<SecretsManager
					workspaceEntryId="demo-entry"
					environment="dev"
					projects={[{ name: "web", relativeDir: "apps/web", kind: "app" }]}
				/>
			</SWRConfig>
		</MemoryRouter>,
	);
}

function mockUnconfiguredStorage() {
	const requests = { initializations: 0, bindings: 0, saves: 0, lists: 0 };
	server.use(
		http.get("http://localhost/api/workspaces/demo-entry/secrets", () => {
			requests.lists++;
			if (!requests.bindings)
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
		http.get("http://localhost/api/workspaces/demo-entry/environment", () =>
			HttpResponse.json({
				revision: "revision-1",
				backend: "",
				projectId: "",
				siteUrl: "",
				environments: ["dev"],
			}),
		),
		http.get("http://localhost/api/workspaces/demo-entry/overview", () =>
			HttpResponse.json({ present: true, workspace: { name: "demo" } }),
		),
		http.get("http://localhost/api/infisical/projects/existing", () =>
			HttpResponse.json({
				id: "existing",
				name: "Existing project",
				orgId: "org",
				environments: [{ name: "Development", slug: "dev" }],
			}),
		),
		http.post(
			"http://localhost/api/workspaces/demo-entry/environment/bind",
			async ({ request }) => {
				const body = (await request.json()) as {
					revision: string;
					create: boolean;
					projectId?: string;
				};
				expect(body.revision).toBe("revision-1");
				if (!body.create) expect(body.projectId).toBe("existing");
				requests.bindings++;
				return HttpResponse.json({
					project_id: body.projectId || "new",
					project_name: body.create ? "demo" : "Existing project",
					created: body.create,
					environments: ["dev"],
				});
			},
		),
		http.get("http://localhost/api/infisical/projects", () =>
			HttpResponse.json([
				{
					id: "existing",
					name: "Existing project",
					orgId: "org",
					environments: [{ name: "Development", slug: "dev" }],
				},
			]),
		),
		http.post("http://localhost/api/workspaces/demo-entry/manifest/preview", () =>
			HttpResponse.json({
				revision: "revision-1",
				path: "one.manifest.toml",
				before: "version = 2\n",
				after: 'version = 2\n[env.infisical]\nprojectId = "existing"\n',
			}),
		),
		http.put("http://localhost/api/workspaces/demo-entry/manifest", async ({ request }) => {
			const body = (await request.json()) as { workspace: { environment: { projectId: string } } };
			expect(body.workspace.environment.projectId).toBe("existing");
			requests.bindings++;
			return HttpResponse.json({ revision: "revision-2" });
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
			expect(requests.bindings).toBe(1);
			expect(requests.initializations).toBe(0);
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
		useManifestDraftStore.setState({ drafts: {} });
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

	it("blocks duplicate binding submissions and keeps the pending dialog open", async () => {
		const requests = mockUnconfiguredStorage();
		let complete!: () => void;
		let submits = 0;
		server.use(
			http.post("http://localhost/api/workspaces/demo-entry/environment/bind", async () => {
				submits++;
				await new Promise<void>((resolve) => {
					complete = resolve;
				});
				requests.bindings++;
				return HttpResponse.json({
					project_id: "new",
					project_name: "demo",
					created: true,
					environments: ["dev"],
				});
			}),
		);
		renderManager();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: i18n.t("binding.workspaceTitle") }));
		const confirm = await screen.findByRole("button", { name: i18n.t("binding.createAndBind") });
		await waitFor(() => expect(confirm.hasAttribute("disabled")).toBe(false));
		await user.dblClick(confirm);
		await waitFor(() => expect(submits).toBe(1));
		expect(
			screen.getByRole("button", { name: i18n.t("binding.binding") }).hasAttribute("disabled"),
		).toBe(true);
		await user.keyboard("{Escape}");
		expect(screen.getByRole("dialog")).toBeDefined();
		complete();
		await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
		expect(submits).toBe(1);
	});

	it("recovers a created project after a revision conflict without creating again", async () => {
		const requests = mockUnconfiguredStorage();
		let creates = 0;
		let revision = "revision-1";
		server.use(
			http.get("http://localhost/api/workspaces/demo-entry/environment", () =>
				HttpResponse.json({ revision, projectId: "", environments: ["dev"] }),
			),
			http.post(
				"http://localhost/api/workspaces/demo-entry/environment/bind",
				async ({ request }) => {
					const body = (await request.json()) as {
						create: boolean;
						revision: string;
						projectId?: string;
					};
					if (body.create) {
						creates++;
						revision = "revision-2";
						return HttpResponse.json(
							{
								error: {
									code: "SERVE_MANIFEST_CONFLICT",
									message: "Configuration changed",
									context: {
										partial_state: "project_created_binding_unsaved",
										project_id: "existing",
									},
								},
							},
							{ status: 409 },
						);
					}
					expect(body).toEqual({ create: false, revision: "revision-2", projectId: "existing" });
					requests.bindings++;
					return HttpResponse.json({
						project_id: "existing",
						project_name: "demo",
						created: false,
						environments: ["dev"],
					});
				},
			),
		);
		renderManager();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: i18n.t("binding.workspaceTitle") }));
		const create = await screen.findByRole("button", { name: i18n.t("binding.createAndBind") });
		await waitFor(() => expect(create.hasAttribute("disabled")).toBe(false));
		await user.click(create);
		await screen.findByText(i18n.t("binding.conflict"));
		const bind = screen.getByRole("button", { name: i18n.t("binding.bindExisting") });
		expect(bind.hasAttribute("disabled")).toBe(true);
		await user.click(
			within(screen.getByRole("dialog")).getByRole("button", { name: i18n.t("secrets.retry") }),
		);
		await waitFor(() => expect(bind.hasAttribute("disabled")).toBe(false));
		await user.click(bind);
		await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
		expect(creates).toBe(1);
		expect(requests.bindings).toBe(1);
	});

	it("requires pending configuration to be saved or discarded before binding", async () => {
		const requests = mockUnconfiguredStorage();
		useManifestDraftStore.getState().stageWorkspaceSection({
			entryId: "demo-entry",
			revision: "revision-1",
			section: "environment",
			initial: {},
			next: { backend: "infisical", projectId: "existing" },
			labels: {},
		});
		renderManager();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: i18n.t("binding.workspaceTitle") }));
		await screen.findByText(i18n.t("binding.pendingDraft"));
		const confirm = screen.getByRole("button", { name: i18n.t("binding.createAndBind") });
		expect(confirm.hasAttribute("disabled")).toBe(true);
		expect(requests.bindings).toBe(0);
		await user.click(screen.getByRole("button", { name: i18n.t("manifestDraft.discard") }));
		await waitFor(() => expect(confirm.hasAttribute("disabled")).toBe(false));
		expect(useManifestDraftStore.getState().drafts["demo-entry"]).toBeUndefined();
	});

	it("rejects a workspace project without the required dev environment", async () => {
		const requests = mockUnconfiguredStorage();
		server.use(
			http.get("http://localhost/api/infisical/projects/existing", () =>
				HttpResponse.json({
					id: "existing",
					name: "Existing project",
					orgId: "org",
					environments: [{ slug: "prod", name: "Production" }],
				}),
			),
		);
		renderManager();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: i18n.t("binding.workspaceTitle") }));
		await user.click(screen.getByRole("combobox", { name: i18n.t("binding.method") }));
		await user.click(await screen.findByRole("option", { name: i18n.t("binding.existing") }));
		await user.click(screen.getByRole("combobox", { name: i18n.t("global.project") }));
		await user.click(await screen.findByRole("option", { name: "Existing project" }));
		await screen.findByText(i18n.t("binding.devRequired"));
		expect(
			screen.getByRole("button", { name: i18n.t("binding.bindExisting") }).hasAttribute("disabled"),
		).toBe(true);
		expect(requests.bindings).toBe(0);
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

		it("disables adding secrets until storage is explicitly connected", async () => {
			expect(
				screen.getByRole("button", { name: i18n.t("secrets.add") }).hasAttribute("disabled"),
			).toBe(true);
			await user.click(screen.getByRole("button", { name: i18n.t("secrets.add") }));
			expect(screen.queryByRole("dialog")).toBeNull();
			expect(requests.initializations).toBe(0);
			expect(requests.saves).toBe(0);
		});

		it("opens and closes the connection dialog without creating storage", async () => {
			await user.click(screen.getByRole("button", { name: i18n.t("binding.workspaceTitle") }));
			const dialog = within(await screen.findByRole("dialog"));
			expect(await dialog.findByRole("combobox", { name: i18n.t("binding.method") })).toBeDefined();
			await user.click(dialog.getAllByRole("button", { name: i18n.t("form.close") })[0]);
			await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
			expect(requests.initializations).toBe(0);
			expect(requests.bindings).toBe(0);
			expect(requests.saves).toBe(0);
		});

		it("creates and binds storage only after the explicit confirmation", async () => {
			await user.click(screen.getByRole("button", { name: i18n.t("binding.workspaceTitle") }));
			const dialog = within(await screen.findByRole("dialog"));
			const confirm = await dialog.findByRole("button", { name: i18n.t("binding.createAndBind") });
			await waitFor(() => expect(confirm.hasAttribute("disabled")).toBe(false));
			expect(requests.bindings).toBe(0);
			await user.click(confirm);
			await waitFor(() => expect(requests.bindings).toBe(1));
			await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
			expect(requests.initializations).toBe(0);
			expect(
				screen.getAllByRole("button", { name: i18n.t("secrets.add") })[0].hasAttribute("disabled"),
			).toBe(false);
		});
		it("saves secrets only after confirming an explicit binding", async () => {
			await user.click(screen.getByRole("button", { name: i18n.t("binding.workspaceTitle") }));
			const connection = within(await screen.findByRole("dialog"));
			await user.click(await connection.findByRole("combobox", { name: i18n.t("binding.method") }));
			await user.click(await screen.findByRole("option", { name: i18n.t("binding.existing") }));
			const selector = await connection.findByRole("combobox", { name: i18n.t("global.project") });
			await waitFor(() => expect(selector.hasAttribute("disabled")).toBe(false));
			await user.click(selector);
			await user.click(await screen.findByRole("option", { name: "Existing project" }));
			expect(requests.bindings).toBe(0);
			const confirm = connection.getByRole("button", { name: i18n.t("binding.bindExisting") });
			await waitFor(() => expect(confirm.hasAttribute("disabled")).toBe(false));
			await user.click(confirm);
			await waitFor(() => expect(requests.bindings).toBe(1));
			await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
			const add = await screen.findAllByRole("button", { name: i18n.t("secrets.add") });
			await user.click(add[0]);
			const dialog = within(await screen.findByRole("dialog"));
			await user.click(dialog.getByLabelText(i18n.t("secrets.key")));
			await user.paste("API_TOKEN");
			await user.click(dialog.getByLabelText(i18n.t("secrets.value")));
			await user.paste("secret-value");
			expect(requests.initializations).toBe(0);
			await user.click(dialog.getByRole("button", { name: i18n.t("secrets.save") }));
			await waitFor(() => expect(requests.saves).toBe(1));
			expect(requests.initializations).toBe(0);
			await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
		});
	});
});
