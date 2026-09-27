import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { MemoryRouter } from "react-router-dom";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import {
	ProfileEditorDialog,
	type ProfileEditorTarget,
} from "@/features/profile-editor/ProfileEditorDialog";
import i18n from "@/lib/i18n";
import type { BackendSpec } from "@/types/api";

const server = setupServer();

const infisicalBackend: BackendSpec = {
	id: "env/infisical",
	domain: "env",
	name: "infisical",
	capabilities: ["env"],
	profile: {
		configurable: true,
		fields: [
			{ path: "siteUrl", input_name: "siteUrl", type: "string", label_key: "form.fields.siteUrl" },
			{
				path: "credentials/clientSecret",
				input_name: "client-secret",
				type: "secret",
				label_key: "form.fields.clientSecret",
				required: true,
			},
		],
	},
};

describe("profile editor dialog", () => {
	beforeAll(async () => {
		server.listen({ onUnhandledRequest: "error" });
		await i18n.changeLanguage("en-US");
	});
	afterEach(() => server.resetHandlers());
	afterAll(() => server.close());

	it("owns profile upsert and reports the saved result", async () => {
		let requestBody: unknown;
		server.use(
			http.post("http://localhost/api/configure/env/infisical", async ({ request }) => {
				requestBody = await request.json();
				return HttpResponse.json({
					schema: "one-cli/serve-configure-upsert/v1",
					status: "completed",
					domain: "env",
					backend: "infisical",
					name: "production",
					default: true,
				});
			}),
		);
		const onOpenChange = vi.fn();
		const onSaved = vi.fn();
		const target: ProfileEditorTarget = {
			backend: infisicalBackend,
			name: "production",
			profile: { siteUrl: "https://app.infisical.com", credentials: { clientSecret: "" } },
			mode: "edit",
			hasDefault: true,
		};

		render(
			<MemoryRouter>
				<ProfileEditorDialog target={target} onOpenChange={onOpenChange} onSaved={onSaved} />
			</MemoryRouter>,
		);

		await userEvent.type(screen.getByLabelText("Client Secret"), "secret-token");
		await userEvent.click(screen.getByRole("button", { name: "Save" }));

		await waitFor(() => {
			expect(requestBody).toEqual({
				name: "production",
				profile: {
					siteUrl: "https://app.infisical.com",
					credentials: { clientSecret: "secret-token" },
				},
				use: false,
			});
		});
		expect(onOpenChange).toHaveBeenCalledWith(false);
		expect(onSaved).toHaveBeenCalledWith(
			expect.objectContaining({ name: "production", status: "completed" }),
		);
	});

	it("keeps a masked secret unchanged when the user leaves it blank", async () => {
		let requestBody: unknown;
		server.use(
			http.post("http://localhost/api/configure/env/infisical", async ({ request }) => {
				requestBody = await request.json();
				return HttpResponse.json({
					schema: "one-cli/serve-configure-upsert/v1",
					status: "updated",
					domain: "env",
					backend: "infisical",
					name: "production",
					default: true,
				});
			}),
		);

		render(
			<MemoryRouter>
				<ProfileEditorDialog
					target={{
						backend: infisicalBackend,
						name: "production",
						profile: {
							siteUrl: "https://app.infisical.com",
							credentials: { clientSecret: "********" },
						},
						mode: "edit",
						hasDefault: true,
					}}
					onOpenChange={() => {}}
				/>
			</MemoryRouter>,
		);

		const token = screen.getByLabelText("Client Secret") as HTMLInputElement;
		expect(token.type).toBe("password");
		expect(token.value).toBe("");
		expect(token.placeholder).toBe("Leave blank to keep unchanged");
		expect(screen.queryByDisplayValue("********")).toBeNull();

		await userEvent.click(screen.getByRole("button", { name: "Save" }));
		await waitFor(() => {
			expect(requestBody).toEqual({
				name: "production",
				profile: {
					siteUrl: "https://app.infisical.com",
					credentials: { clientSecret: "********" },
				},
				use: false,
			});
		});
	});

	it("sends the default-profile choice from the checkbox", async () => {
		let requestBody: unknown;
		server.use(
			http.post("http://localhost/api/configure/env/infisical", async ({ request }) => {
				requestBody = await request.json();
				return HttpResponse.json({
					schema: "one-cli/serve-configure-upsert/v1",
					status: "updated",
					domain: "env",
					backend: "infisical",
					name: "production",
					default: true,
				});
			}),
		);

		render(
			<MemoryRouter>
				<ProfileEditorDialog
					target={{
						backend: infisicalBackend,
						name: "production",
						profile: {
							siteUrl: "https://app.infisical.com",
							credentials: { clientSecret: "********" },
						},
						mode: "edit",
						hasDefault: true,
					}}
					onOpenChange={() => {}}
				/>
			</MemoryRouter>,
		);

		await userEvent.click(screen.getByLabelText("Set default after save"));
		await userEvent.click(screen.getByRole("button", { name: "Save" }));

		await waitFor(() => {
			expect(requestBody).toMatchObject({ name: "production", use: true });
		});
	});
});
