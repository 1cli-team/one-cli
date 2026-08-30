import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { SWRConfig } from "swr";
import { beforeAll, describe, expect, it } from "vitest";
import { workspacesKey } from "@/api/workspaces";
import { TopBar } from "@/components/TopBar";
import i18n from "@/lib/i18n";
import type { WorkspacesResponse } from "@/types/api";

const emptyRegistry: WorkspacesResponse = {
	schema: "one-cli/workspaces/v1",
	workspaces: [],
};

function renderTopBar(path: string) {
	return render(
		<SWRConfig
			value={{
				provider: () => new Map(),
				fallback: { [workspacesKey]: emptyRegistry },
				revalidateOnMount: false,
			}}
		>
			<MemoryRouter initialEntries={[path]}>
				<TopBar />
			</MemoryRouter>
		</SWRConfig>,
	);
}

describe("TopBar environment scope", () => {
	beforeAll(async () => {
		await i18n.changeLanguage("en-US");
	});

	it("shows the environment selector only inside a concrete Workspace", () => {
		renderTopBar("/workspace/demo-entry?env=preview");
		expect(screen.getAllByRole("radio")).toHaveLength(3);
	});

	it.each([
		"/settings?env=preview",
		"/settings/env/infisical?env=preview",
		"/profile?env=preview",
		"/section/env/infisical?env=preview",
	])("does not imply that machine-global Profile settings vary by env at %s", (path) => {
		renderTopBar(path);
		expect(screen.queryByRole("radio")).toBeNull();
	});
});
