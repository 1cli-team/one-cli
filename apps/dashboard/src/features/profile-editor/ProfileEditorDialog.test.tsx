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

const vercelBackend: BackendSpec = {
	id: "deploy/vercel",
	domain: "deploy",
	name: "vercel",
	capabilities: ["deploy"],
	profile: {
		configurable: true,
		fields: [
			{ path: "team", input_name: "team", type: "string", label_key: "form.fields.teamSlug" },
			{
				path: "credentials/apiToken",
				input_name: "token",
				type: "secret",
				label_key: "form.fields.apiToken",
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
			http.post("http://localhost/api/configure/deploy/vercel", async ({ request }) => {
				requestBody = await request.json();
				return HttpResponse.json({
					schema: "one-cli/serve-configure-upsert/v1",
					status: "completed",
					domain: "deploy",
					backend: "vercel",
					name: "production",
					default: true,
				});
			}),
		);
		const onOpenChange = vi.fn();
		const onSaved = vi.fn();
		const target: ProfileEditorTarget = {
			backend: vercelBackend,
			name: "production",
			profile: { team: "one-team", credentials: { apiToken: "" } },
			mode: "edit",
			hasDefault: true,
		};

		render(
			<MemoryRouter>
				<ProfileEditorDialog target={target} onOpenChange={onOpenChange} onSaved={onSaved} />
			</MemoryRouter>,
		);

		await userEvent.type(screen.getByLabelText("API Token"), "secret-token");
		await userEvent.click(screen.getByRole("button", { name: "Save" }));

		await waitFor(() => {
			expect(requestBody).toEqual({
				name: "production",
				profile: { team: "one-team", credentials: { apiToken: "secret-token" } },
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
			http.post("http://localhost/api/configure/deploy/vercel", async ({ request }) => {
				requestBody = await request.json();
				return HttpResponse.json({
					schema: "one-cli/serve-configure-upsert/v1",
					status: "updated",
					domain: "deploy",
					backend: "vercel",
					name: "production",
					default: true,
				});
			}),
		);

		render(
			<MemoryRouter>
				<ProfileEditorDialog
					target={{
						backend: vercelBackend,
						name: "production",
						profile: { team: "one-team", credentials: { apiToken: "********" } },
						mode: "edit",
						hasDefault: true,
					}}
					onOpenChange={() => {}}
				/>
			</MemoryRouter>,
		);

		const token = screen.getByLabelText("API Token") as HTMLInputElement;
		expect(token.type).toBe("password");
		expect(token.value).toBe("");
		expect(token.placeholder).toBe("Leave blank to keep unchanged");
		expect(screen.queryByDisplayValue("********")).toBeNull();

		await userEvent.click(screen.getByRole("button", { name: "Save" }));
		await waitFor(() => {
			expect(requestBody).toEqual({
				name: "production",
				profile: { team: "one-team", credentials: { apiToken: "********" } },
				use: false,
			});
		});
	});
});
