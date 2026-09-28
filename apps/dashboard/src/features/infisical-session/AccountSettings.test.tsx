import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { SWRConfig } from "swr";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import * as api from "@/api/session";
import i18n from "@/lib/i18n";
import { AccountSettings } from "./AccountSettings";
import { useThemeStore } from "@/lib/stores/theme";
import { ThemeProvider } from "@/providers/ThemeProvider";
vi.mock("@/api/session", async (original) => ({
	...(await original<typeof api>()),
	getSession: vi.fn(),
	startLogin: vi.fn(),
	cancelLogin: vi.fn(),
	logout: vi.fn(),
}));
beforeEach(async () => {
	vi.resetAllMocks();
	useThemeStore.setState({ mode: "light" });
	await i18n.changeLanguage("en-US");
});
afterEach(() => {
	localStorage.removeItem("app_theme_mode");
	document.documentElement.classList.remove("dark", "light");
	document.documentElement.removeAttribute("data-theme");
	document.documentElement.style.colorScheme = "";
});
function mount() {
	return render(
		<SWRConfig
			value={{ provider: () => new Map(), dedupingInterval: 0, shouldRetryOnError: false }}
		>
			<MemoryRouter>
				<ThemeProvider>
					<AccountSettings />
				</ThemeProvider>
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

describe("theme preference", () => {
	it.each(["en-US", "zh-CN"])(
		"saves the theme and follows shared theme changes in %s",
		async (locale) => {
			await i18n.changeLanguage(locale);
			vi.mocked(api.getSession).mockResolvedValue({ session: { loggedIn: false, expired: false } });
			const user = userEvent.setup();
			mount();
			await user.click(screen.getByRole("combobox", { name: i18n.t("session.theme") }));
			await user.click(await screen.findByRole("option", { name: i18n.t("session.themeDark") }));
			expect(useThemeStore.getState().mode).toBe("dark");
			expect(localStorage.getItem("app_theme_mode")).toBe("dark");
			expect(document.documentElement.classList.contains("dark")).toBe(true);
			await act(async () => {
				useThemeStore.getState().toggle();
			});
			expect(screen.getByRole("combobox", { name: i18n.t("session.theme") }).textContent).toBe(
				i18n.t("session.themeLight"),
			);
			expect(localStorage.getItem("app_theme_mode")).toBe("light");
			expect(document.documentElement.classList.contains("dark")).toBe(false);
		},
	);
});
