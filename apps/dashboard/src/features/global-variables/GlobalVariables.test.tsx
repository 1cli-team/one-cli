import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { SWRConfig } from "swr";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as api from "@/api/session";
import i18n from "@/lib/i18n";
import { GlobalVariables } from "./GlobalVariables";

vi.mock("@/api/session", async (original) => ({
	...(await original<typeof api>()),
	getSession: vi.fn(),
	getLocation: vi.fn(),
	getProject: vi.fn(),
	getGlobalListing: vi.fn(),
	readGlobalSecret: vi.fn(),
	saveGlobalSecret: vi.fn(),
}));
const location: api.GlobalLocation = {
	siteUrl: "https://app.infisical.com",
	userId: "user",
	organizationId: "org",
	projectId: "shared",
	projectName: "Shared",
	defaultEnvironment: "dev",
};
beforeEach(async () => {
	vi.resetAllMocks();
	await i18n.changeLanguage("en-US");
	vi.mocked(api.getSession).mockResolvedValue({
		session: {
			loggedIn: true,
			expired: false,
			userId: "user",
			siteUrl: location.siteUrl,
			organizationId: "org",
		},
	});
	vi.mocked(api.getLocation).mockResolvedValue({ location });
	vi.mocked(api.getProject).mockResolvedValue({
		id: "shared",
		name: "Shared",
		orgId: "org",
		environments: [
			{ name: "Development", slug: "dev" },
			{ name: "Production", slug: "prod" },
		],
	});
	vi.mocked(api.getGlobalListing).mockImplementation(async (query) => ({
		location,
		environment: new URLSearchParams(query).get("env")!,
		path: "/",
		folders: [],
		variables: [{ key: "OSS_AK", description: "Upload assets" }],
	}));
	vi.mocked(api.readGlobalSecret).mockResolvedValue({ value: "sensitive-test-value" });
});
function mount() {
	return render(
		<SWRConfig
			value={{ provider: () => new Map(), dedupingInterval: 0, shouldRetryOnError: false }}
		>
			<MemoryRouter>
				<GlobalVariables />
			</MemoryRouter>
		</SWRConfig>,
	);
}
describe("global credential browsing", () => {
	it("lists metadata without fetching values and clears a revealed value on environment change", async () => {
		mount();
		const user = userEvent.setup();
		await screen.findByText("OSS_AK");
		expect(api.readGlobalSecret).not.toHaveBeenCalled();
		await user.click(screen.getByRole("button", { name: "Reveal" }));
		await screen.findByText("sensitive-test-value");
		await user.click(screen.getByRole("combobox", { name: "Browsing environment" }));
		await user.click(await screen.findByRole("option", { name: "Production (prod)" }));
		await waitFor(() => expect(api.getGlobalListing).toHaveBeenCalledWith("?env=prod&path=%2F"));
		expect(screen.queryByText("sensitive-test-value")).toBeNull();
	});
	it("discards a late plaintext response after leaving the page", async () => {
		let resolve!: (v: { value: string }) => void;
		vi.mocked(api.readGlobalSecret).mockReturnValue(
			new Promise((r) => {
				resolve = r;
			}),
		);
		const page = mount();
		const user = userEvent.setup();
		await screen.findByText("OSS_AK");
		await user.click(screen.getByRole("button", { name: "Reveal" }));
		page.unmount();
		await act(async () => resolve({ value: "late-secret" }));
		expect(screen.queryByText("late-secret")).toBeNull();
	});
	it("shows fetch failures instead of an empty-folder result", async () => {
		vi.mocked(api.getGlobalListing).mockRejectedValue(new Error("Permission denied"));
		mount();
		await screen.findByRole("alert");
		expect(screen.getByRole("alert").textContent).toContain("Permission denied");
		expect(screen.queryByText("No variables in this folder.")).toBeNull();
	});
});
