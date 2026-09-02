import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { SWRConfig } from "swr";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { applyManifestDraft, previewManifestDraft } from "@/api/manifest";
import { switchWorkspaceEnvironmentBackend } from "@/api/workspace";
import { workspacesKey } from "@/api/workspaces";
import { ManifestSaveControl, TopBar } from "@/components/TopBar";
import { useManifestDraftStore } from "@/features/manifest-draft/manifest-draft-store";
import i18n from "@/lib/i18n";
import type { WorkspacesResponse } from "@/types/api";

vi.mock("@/api/manifest", () => ({
	applyManifestDraft: vi.fn(),
	previewManifestDraft: vi.fn(),
}));
vi.mock("@/api/workspace", () => ({ switchWorkspaceEnvironmentBackend: vi.fn() }));

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
			before: JSON.stringify(
				{
					domains: { env: "infisical" },
					projects: [{ name: "web", general: { buildVersion: "1.0.0" } }],
				},
				null,
				2,
			),
			after: JSON.stringify(
				{
					domains: { env: "dotenv" },
					projects: [{ name: "web", general: { buildVersion: "2.0.0" } }],
				},
				null,
				2,
			),
		});
	});
	afterEach(() => {
		useManifestDraftStore.getState().clearWorkspace("demo-entry");
		vi.clearAllMocks();
	});

	it.each([
		"/",
		"/settings?env=preview",
		"/settings/env/infisical?env=preview",
		"/profile?env=preview",
		"/section/env/infisical?env=preview",
	])("keeps the global TopBar free of workspace environment controls at %s", (path) => {
		renderTopBar(path);
		expect(screen.queryByRole("combobox", { name: /^Environment:/ })).toBeNull();
	});

	it("reviews and publishes a revision-checked manifest draft", async () => {
		vi.mocked(applyManifestDraft).mockResolvedValue({
			schema: "one-cli/workspace-manifest-apply/v1",
			revision: "sha256:next",
			applied: 1,
		});
		useManifestDraftStore.getState().stageSection({
			entryId: "demo-entry",
			revision: "sha256:base",
			project: "web",
			section: "general",
			initial: { buildVersion: "1.0.0", devCommand: "pnpm dev" },
			next: { buildVersion: "2.0.0", devCommand: "pnpm dev" },
			labels: { buildVersion: "projectInspector.general.buildVersion" },
		});
		const user = userEvent.setup();
		renderManifestSaveControl("/workspace/demo-entry?env=dev");

		await user.click(screen.getByRole("button", { name: "Save changes · 1" }));
		const dialog = await screen.findByRole("alertdialog");
		expect(await within(dialog).findByText(/"buildVersion": "1\.0\.0"/)).toBeDefined();
		expect(within(dialog).getByText(/"buildVersion": "2\.0\.0"/)).toBeDefined();
		expect(previewManifestDraft).toHaveBeenCalledWith(
			{
				revision: "sha256:base",
				workspace: undefined,
				changes: [
					{
						project: "web",
						general: { buildVersion: "2.0.0", devCommand: "pnpm dev" },
					},
				],
			},
			"demo-entry",
		);

		await user.click(within(dialog).getByRole("button", { name: "Save to manifest" }));
		await waitFor(() =>
			expect(applyManifestDraft).toHaveBeenCalledWith(
				{
					revision: "sha256:base",
					changes: [
						{
							project: "web",
							general: { buildVersion: "2.0.0", devCommand: "pnpm dev" },
						},
					],
				},
				"demo-entry",
			),
		);
		expect(screen.queryByRole("button", { name: /Save changes/ })).toBeNull();
	});

	it("reviews and publishes a Workspace environment backend draft", async () => {
		vi.mocked(switchWorkspaceEnvironmentBackend).mockResolvedValue({
			schema: "one-cli/workspace-profile/v1",
			root: "/workspace/demo",
			environment: "dev",
			revision: "sha256:next",
			domain: "env",
			backend: "dotenv",
			configurable: false,
			selectedProfile: "",
		});
		useManifestDraftStore.getState().stageWorkspaceSection({
			entryId: "demo-entry",
			revision: "sha256:base",
			section: "environment",
			initial: { backend: "infisical" },
			next: { backend: "dotenv" },
			labels: { backend: "overview.workspaceEnv.backend" },
		});
		const user = userEvent.setup();
		renderManifestSaveControl("/workspace/demo-entry?env=dev");

		await user.click(screen.getByRole("button", { name: "Save changes · 1" }));
		const dialog = await screen.findByRole("alertdialog");
		expect(await within(dialog).findByText(/"env": "infisical"/)).toBeDefined();
		expect(within(dialog).getByText(/"env": "dotenv"/)).toBeDefined();
		expect(previewManifestDraft).toHaveBeenCalledWith(
			{
				revision: "sha256:base",
				workspace: { environment: { backend: "dotenv" } },
				changes: [],
			},
			"demo-entry",
		);

		await user.click(within(dialog).getByRole("button", { name: "Save to manifest" }));
		await waitFor(() =>
			expect(switchWorkspaceEnvironmentBackend).toHaveBeenCalledWith(
				"dotenv",
				"sha256:base",
				"demo-entry",
				"dev",
			),
		);
		expect(applyManifestDraft).not.toHaveBeenCalled();
	});

	it("rebases remaining Project changes after a successful Backend switch", async () => {
		vi.mocked(switchWorkspaceEnvironmentBackend).mockResolvedValue({
			schema: "one-cli/workspace-profile/v1",
			root: "/workspace/demo",
			environment: "dev",
			revision: "sha256:after-switch",
			domain: "env",
			backend: "dotenv",
			configurable: false,
		});
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
			initial: { backend: "infisical" },
			next: { backend: "dotenv" },
			labels: { backend: "overview.workspaceEnv.backend" },
		});
		useManifestDraftStore.getState().stageSection({
			entryId: "demo-entry",
			revision: "sha256:base",
			project: "web",
			section: "general",
			initial: { buildVersion: "1.0.0", devCommand: "pnpm dev" },
			next: { buildVersion: "2.0.0", devCommand: "pnpm dev" },
			labels: { buildVersion: "projectInspector.general.buildVersion" },
		});
		const user = userEvent.setup();
		renderManifestSaveControl("/workspace/demo-entry?env=dev");

		await user.click(screen.getByRole("button", { name: "Save changes · 2" }));
		const dialog = await screen.findByRole("alertdialog");
		const saveButton = within(dialog).getByRole("button", { name: "Save to manifest" });
		await waitFor(() => expect((saveButton as HTMLButtonElement).disabled).toBe(false));
		await user.click(saveButton);
		expect(await screen.findByText("Project publication failed.")).toBeDefined();
		expect(applyManifestDraft).toHaveBeenCalledWith(
			{
				revision: "sha256:after-switch",
				changes: [
					{
						project: "web",
						general: { buildVersion: "2.0.0", devCommand: "pnpm dev" },
					},
				],
			},
			"demo-entry",
		);
		const remaining = useManifestDraftStore.getState().drafts["demo-entry"];
		expect(remaining.revision).toBe("sha256:after-switch");
		expect(remaining.workspace).toBeUndefined();
		expect(Object.keys(remaining.changes)).toEqual(["web"]);
	});
});
