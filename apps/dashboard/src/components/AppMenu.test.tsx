import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { MemoryRouter, useLocation } from "react-router-dom";
import { SWRConfig } from "swr";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { AppMenu } from "@/components/AppMenu";
import { TopBar } from "@/components/TopBar";
import { ThemeProvider } from "@/providers/ThemeProvider";
import i18n from "@/lib/i18n";
import { useLocaleStore } from "@/lib/stores/locale";
import { useThemeStore } from "@/lib/stores/theme";

const workspaces = [
	{ entryId: "alpha", name: "Alpha", root: "/workspaces/alpha", status: "ready", projectCount: 1 },
	{
		entryId: "beta",
		name: "Beta",
		root: "/workspaces/beta",
		status: "identity-conflict",
		projectCount: 2,
	},
];
const server = setupServer(
	http.get("http://localhost/api/workspaces", () => HttpResponse.json({ workspaces })),
	http.get("http://localhost/api/session", () =>
		HttpResponse.json({ session: { loggedIn: true, email: "test@example.com" } }),
	),
);
function LocationProbe() {
	const location = useLocation();
	return (
		<output data-testid="location">
			{location.pathname}
			{location.search}
		</output>
	);
}
function show(withHeader = false) {
	return render(
		<SWRConfig value={{ provider: () => new Map(), shouldRetryOnError: false }}>
			<MemoryRouter initialEntries={["/workspace/alpha?env=staging"]}>
				<ThemeProvider>{withHeader ? <TopBar /> : <AppMenu />}</ThemeProvider>
				<LocationProbe />
			</MemoryRouter>
		</SWRConfig>,
	);
}
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
beforeEach(async () => {
	await i18n.changeLanguage("en-US");
	useThemeStore.setState({ mode: "light" });
	useLocaleStore.setState({ mode: "auto", resolved: "en-US" });
});
afterEach(() => {
	server.resetHandlers();
	localStorage.removeItem("app_theme_mode");
	localStorage.removeItem("app_locale_mode");
	document.documentElement.classList.remove("light", "dark");
	document.documentElement.removeAttribute("data-theme");
	document.documentElement.style.colorScheme = "";
});
afterAll(() => server.close());

describe("brand navigation menu", () => {
	it.each(["en-US", "zh-CN"])(
		"opens from the brand and switches workspace with the environment preserved in %s",
		async (locale) => {
			await i18n.changeLanguage(locale);
			const user = userEvent.setup();
			show();
			const trigger = screen.getByRole("button", { name: i18n.t("appMenu.label") });
			expect(screen.queryByRole("menu")).toBeNull();
			await user.click(trigger);
			const menu = await screen.findByRole("menu", { name: i18n.t("appMenu.label") });
			const selected = await within(menu).findByRole("menuitem", { name: /Alpha/ });
			expect(selected.getAttribute("aria-current")).toBe("page");
			expect(within(menu).getByRole("menuitem", { name: /Beta/ }).getAttribute("href")).toBe(
				"/workspace/beta?env=staging",
			);
			expect(await within(menu).findByText("test@example.com")).toBeDefined();
			await user.click(within(menu).getByRole("menuitem", { name: /Beta/ }));
			await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
			expect(screen.getByTestId("location").textContent).toBe("/workspace/beta?env=staging");
		},
	);

	it("opens with the keyboard, dismisses with Escape and returns focus to the brand", async () => {
		const user = userEvent.setup();
		show();
		const trigger = screen.getByRole("button", { name: "One CLI navigation menu" });
		trigger.focus();
		await user.keyboard("{ArrowDown}");
		await screen.findByRole("menu");
		await user.keyboard("{Home}");
		expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Home" }));
		await user.keyboard("{Escape}");
		await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
		expect(document.activeElement).toBe(trigger);
	});

	it.each(["en-US", "zh-CN"])(
		"switches theme and language from the header in %s",
		async (locale) => {
			await i18n.changeLanguage(locale);
			const user = userEvent.setup();
			show(true);
			const header = within(screen.getByRole("banner"));
			await user.click(header.getByRole("button", { name: i18n.t("sidebar.themeToDark") }));
			expect(useThemeStore.getState().mode).toBe("dark");
			expect(localStorage.getItem("app_theme_mode")).toBe("dark");
			expect(document.documentElement.classList.contains("dark")).toBe(true);
			await user.click(header.getByRole("button", { name: i18n.t("sidebar.themeToLight") }));
			expect(useThemeStore.getState().mode).toBe("light");
			const language = header.getByRole("button", { name: i18n.t("sidebar.language") });
			await user.click(language);
			await user.click(await screen.findByRole("menuitemradio", { name: "中文" }));
			expect(useLocaleStore.getState().mode).toBe("zh-CN");
			expect(localStorage.getItem("app_locale_mode")).toBe("zh-CN");
			await waitFor(() => expect(screen.queryByRole("menu")).toBeNull());
			expect(document.activeElement).toBe(language);
			await user.click(header.getByRole("button", { name: i18n.t("appMenu.label") }));
			const menu = await screen.findByRole("menu");
			expect(within(menu).queryByRole("menuitem", { name: i18n.t("sidebar.language") })).toBeNull();
			expect(
				within(menu).queryByRole("menuitem", { name: i18n.t("sidebar.themeToDark") }),
			).toBeNull();
		},
	);

	it("shows a retry action when the workspace registry fails", async () => {
		server.use(
			http.get("http://localhost/api/workspaces", () => new HttpResponse(null, { status: 500 })),
		);
		const user = userEvent.setup();
		show();
		await user.click(screen.getByRole("button", { name: "One CLI navigation menu" }));
		expect(await screen.findByRole("alert")).toBeDefined();
		server.use(
			http.get("http://localhost/api/workspaces", () => HttpResponse.json({ workspaces })),
		);
		await user.click(screen.getByRole("menuitem", { name: "Retry" }));
		expect(await screen.findByRole("menuitem", { name: /Alpha/ })).toBeDefined();
		expect(screen.getByRole("menuitem", { name: "Home" }).getAttribute("href")).toBe(
			"/?env=staging",
		);
	});
});
