import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { SWRConfig } from "swr";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { applyManifestDraft } from "@/api/manifest";
import { switchWorkspaceEnvironmentBackend } from "@/api/workspace";
import { workspacesKey } from "@/api/workspaces";
import { TopBar } from "@/components/TopBar";
import { useManifestDraftStore } from "@/features/manifest-draft/manifest-draft-store";
import i18n from "@/lib/i18n";
import type { WorkspacesResponse } from "@/types/api";

vi.mock("@/api/manifest", () => ({ applyManifestDraft: vi.fn() }));
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

describe("TopBar environment scope", () => {
	beforeAll(async () => {
		await i18n.changeLanguage("en-US");
	});
	afterEach(() => {
		useManifestDraftStore.getState().clearWorkspace("demo-entry");
		vi.clearAllMocks();
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
		renderTopBar("/workspace/demo-entry?env=dev");

		await user.click(screen.getByRole("button", { name: "Save changes · 1" }));
		const dialog = await screen.findByRole("alertdialog");
		expect(within(dialog).getByText("web")).toBeDefined();
		expect(within(dialog).getByText("1.0.0")).toBeDefined();
		expect(within(dialog).getByText("2.0.0")).toBeDefined();

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
		renderTopBar("/workspace/demo-entry?env=dev");

		await user.click(screen.getByRole("button", { name: "Save changes · 1" }));
		const dialog = await screen.findByRole("alertdialog");
		expect(within(dialog).getByText("Workspace")).toBeDefined();
		expect(within(dialog).getByText("infisical")).toBeDefined();
		expect(within(dialog).getByText("dotenv")).toBeDefined();

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
		renderTopBar("/workspace/demo-entry?env=dev");

		await user.click(screen.getByRole("button", { name: "Save changes · 2" }));
		await user.click(
			within(await screen.findByRole("alertdialog")).getByRole("button", {
				name: "Save to manifest",
			}),
		);
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
