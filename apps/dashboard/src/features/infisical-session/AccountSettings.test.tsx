import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { SWRConfig } from "swr";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as api from "@/api/session";
import i18n from "@/lib/i18n";
import { AccountSettings } from "./AccountSettings";
vi.mock("@/api/session", async (original) => ({
	...(await original<typeof api>()),
	getSession: vi.fn(),
	startLogin: vi.fn(),
	cancelLogin: vi.fn(),
	logout: vi.fn(),
}));
beforeEach(async () => {
	vi.resetAllMocks();
	await i18n.changeLanguage("en-US");
});
function mount() {
	return render(
		<SWRConfig
			value={{ provider: () => new Map(), dedupingInterval: 0, shouldRetryOnError: false }}
		>
			<MemoryRouter>
				<AccountSettings />
			</MemoryRouter>
		</SWRConfig>,
	);
}
describe("account state and recovery", () => {
	it("shows only a loading state before the session resolves", async () => {
		let finish!: (value: api.SessionState) => void;
		vi.mocked(api.getSession).mockReturnValue(
			new Promise((resolve) => {
				finish = resolve;
			}),
		);
		mount();
		expect(screen.getByRole("status")).toBeDefined();
		expect(screen.queryByRole("button", { name: "Sign in with browser" })).toBeNull();
		await act(async () =>
			finish({ session: { loggedIn: true, expired: false, email: "demo@example.com" } }),
		);
		expect(await screen.findByText("Connected")).toBeDefined();
		expect(screen.getByText("demo@example.com")).toBeDefined();
	});
	it("offers retry on session failure and recovers to the signed-out state", async () => {
		vi.mocked(api.getSession)
			.mockRejectedValueOnce(new Error("Session unavailable"))
			.mockResolvedValue({ session: { loggedIn: false, expired: false } });
		mount();
		const user = userEvent.setup();
		expect(await screen.findByText("Session unavailable")).toBeDefined();
		await user.click(screen.getByRole("button", { name: "Retry" }));
		expect(await screen.findByRole("button", { name: "Sign in with browser" })).toBeDefined();
	});
	it("keeps the waiting login visible with reopen and cancel actions", async () => {
		vi.mocked(api.getSession).mockResolvedValue({
			session: { loggedIn: false, expired: false },
			login: { status: "waiting", url: "https://app.infisical.com/test-login" },
		});
		mount();
		expect(await screen.findByRole("link", { name: "Reopen login page" })).toBeDefined();
		expect(screen.getByRole("button", { name: "Cancel" })).toBeDefined();
		expect(api.startLogin).not.toHaveBeenCalled();
	});
});
