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
	getProjects: vi.fn(),
	createRemoteProject: vi.fn(),
	initializeGlobalLocation: vi.fn(),
	bindLocation: vi.fn(),
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
	vi.mocked(api.getProjects).mockResolvedValue([]);
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

describe("shared credential setup", () => {
	it("initializes the default location only on click and then opens the credential list", async () => {
		vi.mocked(api.getLocation).mockResolvedValue({ location: null });
		vi.mocked(api.initializeGlobalLocation).mockImplementation(async () => {
			vi.mocked(api.getLocation).mockResolvedValue({ location });
			return { location };
		});
		mount();
		const user = userEvent.setup();
		await screen.findByRole("button", { name: "Initialize default location" });
		expect(api.initializeGlobalLocation).not.toHaveBeenCalled();
		expect(api.getGlobalListing).not.toHaveBeenCalled();
		await user.click(screen.getByRole("button", { name: "Initialize default location" }));
		await screen.findByText("OSS_AK");
		expect(api.initializeGlobalLocation).toHaveBeenCalledTimes(1);
	});
	it("creates and selects a project without changing storage until Save location", async () => {
		vi.mocked(api.getLocation).mockResolvedValue({ location: null });
		const created = {
			id: "team",
			name: "Team",
			orgId: "org",
			environments: [{ name: "Development", slug: "dev" }],
		};
		vi.mocked(api.createRemoteProject).mockResolvedValue(created);
		vi.mocked(api.getProject).mockResolvedValue(created);
		vi.mocked(api.bindLocation).mockImplementation(async () => {
			vi.mocked(api.getLocation).mockResolvedValue({ location });
			return { location };
		});
		mount();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: "New project" }));
		await user.type(screen.getByLabelText("Project name"), "Team");
		await user.click(screen.getByRole("button", { name: "Create and select" }));
		await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
		expect(api.createRemoteProject).toHaveBeenCalledWith("Team");
		expect(screen.getByRole("combobox", { name: "Storage project" }).textContent).toContain("Team");
		expect(
			screen.getByRole("combobox", { name: "Default browsing environment" }).textContent,
		).toContain("dev");
		expect(api.bindLocation).not.toHaveBeenCalled();
		await user.click(screen.getByRole("button", { name: "Save location" }));
		await waitFor(() => expect(api.bindLocation).toHaveBeenCalledWith("team", "dev"));
		await screen.findByText("OSS_AK");
	});
	it("keeps a failed creation editable and leaves the existing location alone", async () => {
		vi.mocked(api.createRemoteProject).mockRejectedValue(
			new Error("No permission to create projects"),
		);
		mount();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: "Default storage location" }));
		expect(screen.queryByRole("button", { name: "Initialize default location" })).toBeNull();
		await user.click(screen.getByRole("button", { name: "New project" }));
		await user.type(screen.getByLabelText("Project name"), "Team");
		await user.click(screen.getByRole("button", { name: "Create and select" }));
		await screen.findByText("No permission to create projects");
		expect(screen.getByRole("dialog")).toBeTruthy();
		expect((screen.getByLabelText("Project name") as HTMLInputElement).value).toBe("Team");
		expect(api.bindLocation).not.toHaveBeenCalled();
		expect(api.initializeGlobalLocation).not.toHaveBeenCalled();
	});
	it("shows default setup failure without falling through to an empty credential list", async () => {
		vi.mocked(api.getLocation).mockResolvedValue({ location: null });
		vi.mocked(api.initializeGlobalLocation).mockRejectedValue(
			new Error("Default environment is missing"),
		);
		mount();
		const user = userEvent.setup();
		await user.click(await screen.findByRole("button", { name: "Initialize default location" }));
		await screen.findByText("Default environment is missing");
		expect(api.getGlobalListing).not.toHaveBeenCalled();
		expect(screen.getByRole("button", { name: "New project" })).toBeTruthy();
	});
});
