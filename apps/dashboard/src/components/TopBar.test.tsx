import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { SWRConfig } from "swr";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { applyManifestDraft, previewManifestDraft } from "@/api/manifest";
import { workspacesKey } from "@/api/workspaces";
import { ManifestSaveControl, TopBar } from "@/components/TopBar";
import { useManifestDraftStore } from "@/features/manifest-draft/manifest-draft-store";
import i18n from "@/lib/i18n";
import type { WorkspacesResponse } from "@/types/api";

vi.mock("@/api/manifest", () => ({
	applyManifestDraft: vi.fn(),
	previewManifestDraft: vi.fn(),
}));

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

function renderManifestSaveControl(path: string) {
	return render(
		<SWRConfig value={{ provider: () => new Map(), revalidateOnMount: false }}>
			<MemoryRouter initialEntries={[path]}>
				<ManifestSaveControl entryId="demo-entry" />
			</MemoryRouter>
		</SWRConfig>,
	);
}

describe("TopBar and manifest review", () => {
	beforeAll(async () => {
		await i18n.changeLanguage("en-US");
	});
	beforeEach(() => {
		vi.mocked(previewManifestDraft).mockResolvedValue({
			schema: "one-cli/workspace-manifest-preview/v1",
			revision: "sha256:base",
			before:
				"version = 2\n[env.infisical]\nprojectId = 'old-project'\nenvironments = ['dev','staging','prod']\n",
			after:
				"version = 2\n[env.infisical]\nprojectId = 'new-project'\nenvironments = ['dev','staging','prod']\n",
		});
	});
	afterEach(() => {
		useManifestDraftStore.getState().clearWorkspace("demo-entry");
		vi.clearAllMocks();
	});

	it.each([
		"/",
		"/settings?env=staging",
		"/settings/env/infisical?env=staging",
		"/profile?env=staging",
		"/section/env/infisical?env=staging",
	])("keeps the global TopBar free of workspace environment controls at %s", (path) => {
		renderTopBar(path);
		expect(screen.queryByRole("combobox", { name: /^Environment:/ })).toBeNull();
	});

	it("reviews and publishes a Workspace Infisical binding draft", async () => {
		useManifestDraftStore.getState().stageWorkspaceSection({
			entryId: "demo-entry",
			revision: "sha256:base",
			section: "environment",
			initial: { backend: "infisical", projectId: "old-project" },
			next: { backend: "infisical", projectId: "new-project" },
			labels: { projectId: "global.project" },
		});
		const user = userEvent.setup();
		renderManifestSaveControl("/workspace/demo-entry?env=dev");

		await user.click(screen.getByRole("button", { name: "Save changes · 1" }));
		const dialog = await screen.findByRole("alertdialog");
		expect(await within(dialog).findByText(/projectId = 'old-project'/)).toBeDefined();
		expect(within(dialog).getByText(/projectId = 'new-project'/)).toBeDefined();
		expect(previewManifestDraft).toHaveBeenCalledWith(
			{
				revision: "sha256:base",
				workspace: { environment: { backend: "infisical", projectId: "new-project" } },
				changes: [],
			},
			"demo-entry",
		);

		await user.click(within(dialog).getByRole("button", { name: "Save to manifest" }));
		await waitFor(() =>
			expect(applyManifestDraft).toHaveBeenCalledWith(
				{
					revision: "sha256:base",
					workspace: { environment: { backend: "infisical", projectId: "new-project" } },
					changes: [],
				},
				"demo-entry",
			),
		);
	});

	it("retains the entire draft when atomic publication fails", async () => {
		vi.mocked(applyManifestDraft).mockRejectedValue({
			status: 500,
			code: "ONE_CLI_ERROR",
			message: "Project publication failed.",
			context: {},
			remediation: [],
		});
		useManifestDraftStore.getState().stageWorkspaceSection({
			entryId: "demo-entry",
			revision: "sha256:base",
			section: "environment",
			initial: { backend: "infisical", projectId: "old-project" },
			next: { backend: "infisical", projectId: "new-project" },
			labels: { projectId: "global.project" },
		});
		const user = userEvent.setup();
		renderManifestSaveControl("/workspace/demo-entry?env=dev");

		await user.click(screen.getByRole("button", { name: "Save changes · 1" }));
		const dialog = await screen.findByRole("alertdialog");
		const saveButton = within(dialog).getByRole("button", { name: "Save to manifest" });
		await waitFor(() => expect((saveButton as HTMLButtonElement).disabled).toBe(false));
		await user.click(saveButton);
		expect(await screen.findByText("Project publication failed.")).toBeDefined();
		expect(applyManifestDraft).toHaveBeenCalledWith(
			{
				revision: "sha256:base",
				workspace: { environment: { backend: "infisical", projectId: "new-project" } },
				changes: [],
			},
			"demo-entry",
		);
		const remaining = useManifestDraftStore.getState().drafts["demo-entry"];
		expect(remaining.revision).toBe("sha256:base");
		expect(remaining.workspace).toEqual({
			environment: { backend: "infisical", projectId: "new-project" },
		});
		expect(Object.keys(remaining.changes)).toEqual([]);
	});
});
