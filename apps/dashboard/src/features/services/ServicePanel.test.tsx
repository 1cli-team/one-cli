import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { controlService, watchService } from "@/api/services";
import { useManifestDraftStore } from "@/features/manifest-draft/manifest-draft-store";
import { mergeService, ServicePanel } from "@/features/services/ServicePanel";
import { consoleText, renderConsole } from "@/features/services/console";
import i18n from "@/lib/i18n";
import type { DevService } from "@/types/api";
vi.mock("@/api/services", () => ({ controlService: vi.fn(), watchService: vi.fn() }));
const idle: DevService = {
	id: "",
	project: "web",
	environment: "dev",
	status: "stopped",
	startedAt: "",
	logs: [],
	endpoints: [],
	firstSeq: 1,
	nextSeq: 1,
};
let receive: (state: DevService) => void;
let connection: (connected: boolean) => void;
const close = vi.fn();
beforeEach(async () => {
	vi.resetAllMocks();
	useManifestDraftStore.setState({ drafts: {} });
	await i18n.changeLanguage("en-US");
	vi.mocked(watchService).mockImplementation((_project, _entry, onState, onConnection) => {
		receive = onState;
		connection = onConnection;
		onState(idle);
		onConnection(true);
		return close;
	});
	vi.mocked(controlService).mockResolvedValue({ ...idle, id: "1", status: "preparing" });
});
function mount(environment = "dev") {
	return render(
		<ServicePanel
			project={{
				name: "web",
				relativeDir: "apps/web",
				kind: "app",
				devAvailable: true,
				environment: { inherits: true },
			}}
			environment={environment}
			entryId="entry"
		/>,
	);
}
describe("service console", () => {
	it("launches the selected environment and keeps stop available with a draft", async () => {
		const user = userEvent.setup();
		mount("prod");
		await user.click(screen.getByRole("button", { name: "Start service" }));
		expect(controlService).toHaveBeenCalledWith("web", "entry", "start", "prod");
		act(() =>
			receive({
				...idle,
				id: "1",
				status: "running",
				environment: "dev",
				endpoints: [{ url: "http://localhost:3000/", reachable: true }],
				logs: [{ seq: 1, text: "ready\n" }],
				nextSeq: 2,
			}),
		);
		expect(screen.getByText(/This service uses dev/)).toBeTruthy();
		expect(screen.getByRole("link", { name: "Open" }).getAttribute("href")).toBe(
			"http://localhost:3000/",
		);
		act(() =>
			useManifestDraftStore.getState().stageWorkspaceSection({
				entryId: "entry",
				revision: "x",
				section: "environment",
				initial: { backend: "infisical", projectId: "old" },
				next: { backend: "infisical", projectId: "new" },
				labels: {},
			}),
		);
		expect((screen.getByRole("button", { name: "Restart" }) as HTMLButtonElement).disabled).toBe(
			true,
		);
		await user.click(screen.getByRole("button", { name: "Stop" }));
		expect(controlService).toHaveBeenLastCalledWith("web", "entry", "stop", "prod");
	});
	it("reconnects without duplicating logs, filters text and closes only the subscription", async () => {
		const user = userEvent.setup();
		const view = mount();
		const next = {
			...idle,
			id: "1",
			status: "running" as const,
			logs: [
				{ seq: 1, text: "hello\n" },
				{ seq: 2, text: "world\n" },
			],
			nextSeq: 3,
		};
		act(() => {
			receive(next);
			connection(false);
			receive(next);
			connection(true);
		});
		expect(screen.getByLabelText("Console", { selector: "pre" }).textContent).toBe(
			"hello\nworld\n",
		);
		await user.type(screen.getByRole("textbox", { name: "Search logs" }), "world");
		expect(screen.getByLabelText("Console", { selector: "pre" }).textContent).toBe("world");
		view.unmount();
		expect(close).toHaveBeenCalled();
		expect(controlService).not.toHaveBeenCalled();
	});
	it("shows failures without claiming the service started, in both languages", async () => {
		await i18n.changeLanguage("zh-CN");
		vi.mocked(controlService).mockRejectedValue({ message: "端口占用" });
		const user = userEvent.setup();
		mount();
		await user.click(screen.getByRole("button", { name: "启动服务" }));
		await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("端口占用"));
		expect(screen.getByText("已停止")).toBeTruthy();
	});
	it("resets logs on a new run and removes expired chunks", () => {
		const previous = {
			...idle,
			id: "1",
			logs: [
				{ seq: 1, text: "old" },
				{ seq: 2, text: "keep" },
			],
			nextSeq: 3,
		};
		expect(
			mergeService(previous, {
				...previous,
				firstSeq: 2,
				logs: [{ seq: 3, text: "new" }],
				nextSeq: 4,
			}).logs.map((x) => x.text),
		).toEqual(["keep", "new"]);
		expect(mergeService(previous, { ...idle, id: "2" }).logs).toEqual([]);
	});
	it("renders ANSI text without interpreting HTML or terminal hyperlinks", () => {
		const raw =
			"\x1b[31m<script>alert(1)</script>\x1b[0m\x1b]8;;https://evil.example\x07link\x1b]8;;\x07";
		const { container } = render(<pre>{renderConsole(raw)}</pre>);
		expect(container.querySelector("script,a")).toBeNull();
		expect(container.textContent).toBe("<script>alert(1)</script>link");
		expect(consoleText(raw)).toBe("<script>alert(1)</script>link");
	});
});
