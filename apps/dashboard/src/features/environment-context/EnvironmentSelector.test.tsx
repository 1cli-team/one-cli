import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, beforeAll, describe, expect, it } from "vitest";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { EnvironmentSelector } from "@/features/environment-context/EnvironmentSelector";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
import i18n from "@/lib/i18n";

function LocationProbe() {
	const location = useLocation();
	return <output data-testid="search">{location.search}</output>;
}

function RouteProbe() {
	const location = useLocation();
	return <output data-testid="route">{`${location.pathname}${location.search}`}</output>;
}

function renderSelector(path: string) {
	return render(
		<MemoryRouter initialEntries={[path]}>
			<EnvironmentSelector />
			<LocationProbe />
		</MemoryRouter>,
	);
}

describe("EnvironmentSelector", () => {
	beforeAll(async () => {
		await i18n.changeLanguage("en-US");
	});
	afterEach(() => {
		useEnvironmentDirtyStore.getState().reset();
	});

	it("defaults to dev without writing the URL on mount", () => {
		renderSelector("/workspace/demo?panel=summary");

		const selector = screen.getByRole("combobox", { name: "Environment: Development" });
		expect(selector.textContent).toContain("Development");
		expect(screen.getByTestId("search").textContent).toBe("?panel=summary");
	});

	it("uses the env query and preserves every other query parameter", async () => {
		const user = userEvent.setup();
		renderSelector("/workspace/demo?view=compact&panel=deploy&env=preview");

		const selector = screen.getByRole("combobox", { name: "Environment: Preview" });
		expect(selector.textContent).toContain("Preview");
		await user.click(selector);
		await user.click(await screen.findByRole("option", { name: "Production" }));

		const search = new URLSearchParams(
			screen.getByTestId("search").textContent?.replace(/^\?/, ""),
		);
		expect(search.get("env")).toBe("prod");
		expect(search.get("view")).toBe("compact");
		expect(search.get("panel")).toBe("deploy");
	});

	it("keeps the selected environment while navigating between Dashboard pages", async () => {
		const user = userEvent.setup();
		render(
			<MemoryRouter initialEntries={["/settings?env=preview"]}>
				<EnvironmentLink to="/settings/env/infisical">Open Infisical</EnvironmentLink>
				<RouteProbe />
			</MemoryRouter>,
		);

		await user.click(screen.getByRole("link", { name: "Open Infisical" }));

		expect(screen.getByTestId("route").textContent).toBe("/settings/env/infisical?env=preview");
	});

	it("keeps guarding while any independent editor remains dirty", async () => {
		const state = useEnvironmentDirtyStore.getState();
		state.setDirty("workspace-editor", true);
		state.setDirty("project-editor", true);
		state.clearOwner("project-editor");
		expect(useEnvironmentDirtyStore.getState().dirty).toBe(true);

		const user = userEvent.setup();
		renderSelector("/workspace/demo?env=dev");
		await user.click(screen.getByRole("combobox", { name: "Environment: Development" }));
		await user.click(await screen.findByRole("option", { name: "Production" }));

		expect(await screen.findByRole("alertdialog")).toBeDefined();
		expect(screen.getByTestId("search").textContent).toBe("?env=dev");
	});
});
