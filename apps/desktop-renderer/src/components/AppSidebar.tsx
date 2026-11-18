import {
	Activity,
	BookOpen,
	ClipboardList,
	ArrowLeft,
	House,
	GitBranch,
	KeyRound,
	MonitorCog,
	Plus,
	Network,
	Settings2,
	SlidersHorizontal,
} from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useMatch, useNavigate, useParams } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { DiscardDialog } from "@/components/ui/discard-dialog";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import {
	EnvironmentLink,
	EnvironmentNavLink,
} from "@/features/environment-context/EnvironmentLink";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
import { preserveEnvironment } from "@/features/environment-context/environment";
import { CreateProjectDialog } from "@/features/project-creation/CreateProjectDialog";
import { useWorkspace } from "@/features/workspace-navigation/useWorkspace";
import { useThemeStore } from "@/lib/stores/theme";
import { cn } from "@/lib/utils";
import { workspaceKnowledgePath, workspacePath, workspaceVariablesPath } from "@/router/paths";
import { WorkspaceGroupsSidebar } from "@/features/workspace-groups/WorkspaceGroupsSidebar";

interface SidebarProps {
	entryId?: string;
	homeTo: string;
	mobileOpen: boolean;
	onMobileOpenChange(open: boolean): void;
}

export function AppSidebar({ mobileOpen, onMobileOpenChange, ...props }: SidebarProps) {
	const { t } = useTranslation();
	return (
		<>
			<aside
				className="hidden h-full w-[248px] shrink-0 overflow-hidden border-r border-sidebar-border bg-sidebar ud-md:block"
				aria-label={t("navigation.label")}
			>
				<SidebarPanels {...props} />
			</aside>
			<Dialog open={mobileOpen} onOpenChange={onMobileOpenChange}>
				<DialogContent className="top-0 left-0 h-dvh max-h-dvh w-[248px] max-w-[calc(100vw-3rem)] ud-sm:max-w-[248px] translate-x-0 translate-y-0 gap-0 overflow-hidden rounded-none bg-sidebar p-0">
					<DialogTitle className="sr-only">{t("navigation.label")}</DialogTitle>
					<SidebarPanels {...props} />
				</DialogContent>
			</Dialog>
		</>
	);
}

function SidebarPanels({ entryId, homeTo }: Pick<SidebarProps, "entryId" | "homeTo">) {
	const settingsMode = Boolean(useMatch("/settings/*"));
	const section = settingsMode ? "settings" : entryId ? "workspace" : null;
	const [lastSection, setLastSection] = useState(section);
	const [lastEntryId, setLastEntryId] = useState(entryId);
	if (section && section !== lastSection) setLastSection(section);
	if (entryId && entryId !== lastEntryId) setLastEntryId(entryId);
	const workspaceMode = Boolean(entryId);
	const detailMode = section !== null;
	return (
		<div className="flex h-full flex-col overflow-hidden" data-sidebar-level={section ?? "global"}>
			{/* Electron hit-tests the title bar separately from the sliding panels. */}
			<SidebarTitlebar homeTo={detailMode ? homeTo : undefined} />
			<div
				className={cn(
					"flex min-h-0 w-[200%] flex-1 transition-transform duration-200 ease-ui motion-reduce:transition-none",
					detailMode && "-translate-x-1/2",
				)}
			>
				<div
					className="flex h-full w-1/2 min-w-0 shrink-0 flex-col"
					inert={detailMode}
					aria-hidden={detailMode}
				>
					<GlobalSidebar />
				</div>
				<div
					className="flex h-full w-1/2 min-w-0 shrink-0 flex-col"
					inert={!detailMode}
					aria-hidden={!detailMode}
				>
					{lastSection === "settings" ? (
						<SettingsSidebar />
					) : lastEntryId ? (
						<WorkspaceSidebar key={lastEntryId} entryId={lastEntryId} active={workspaceMode} />
					) : null}
				</div>
			</div>
		</div>
	);
}

function SidebarTitlebar({ homeTo }: { homeTo?: string }) {
	const { t } = useTranslation();
	const { mode } = useThemeStore();
	const showBrand = homeTo === undefined;
	return (
		<div
			className={cn(
				"window-titlebar window-sidebar-titlebar flex min-h-14 shrink-0 items-center border-b border-border",
				showBrand ? "window-sidebar-brand gap-2.5 px-4" : "px-3",
			)}
		>
			{showBrand ? (
				<>
					<img
						src={mode === "dark" ? "/brand/icon-inverted.svg" : "/brand/icon.svg"}
						alt=""
						className="size-6 shrink-0"
					/>
					<span className="truncate text-base font-medium">One Desktop</span>
				</>
			) : (
				<Button variant="ghost" className="w-full min-w-0 justify-start [&>span]:truncate" asChild>
					<EnvironmentLink to={homeTo}>
						<ArrowLeft />
						<span>
							{t(
								homeTo.startsWith("/projects/")
									? "groups.backToProject"
									: homeTo.startsWith("/workspaces")
										? "groups.allWorkspaces"
										: "groups.allProjects",
							)}
						</span>
					</EnvironmentLink>
				</Button>
			)}
		</div>
	);
}

function GlobalSidebar() {
	const { t } = useTranslation();
	return (
		<>
			<nav className="flex shrink-0 flex-col gap-0.5 p-3" aria-label={t("navigation.global")}>
				<Button variant="navigation" size="navigation-compact" asChild>
					<EnvironmentNavLink to="/" end>
						<House strokeWidth={1.5} />
						<span>{t("sidebar.home")}</span>
					</EnvironmentNavLink>
				</Button>
				<Button variant="navigation" size="navigation-compact" asChild>
					<EnvironmentNavLink to="/gateway" end>
						<Activity strokeWidth={1.5} />
						<span>{t("gateway.monitor")}</span>
					</EnvironmentNavLink>
				</Button>
			</nav>
			<Separator />
			<WorkspaceGroupsSidebar />
			<nav
				className="flex shrink-0 flex-col gap-1 border-t border-border p-3"
				aria-label={t("sidebar.settings")}
			>
				<Button variant="navigation" size="navigation-compact" asChild>
					<EnvironmentNavLink to="/settings">
						<Settings2 strokeWidth={1.5} />
						<span>{t("sidebar.settings")}</span>
					</EnvironmentNavLink>
				</Button>
			</nav>
		</>
	);
}

function SettingsSidebar() {
	const { t } = useTranslation();
	return (
		<>
			<h2 className="shrink-0 px-5 py-4 font-medium">{t("sidebar.settings")}</h2>
			<nav
				className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-3 pb-3"
				aria-label={t("sidebar.settings")}
			>
				<Button variant="navigation" size="navigation-compact" className="min-h-9" asChild>
					<EnvironmentNavLink to="/settings/desktop" end>
						<MonitorCog />
						<span>{t("desktop.settings")}</span>
					</EnvironmentNavLink>
				</Button>
				<Button variant="navigation" size="navigation-compact" className="min-h-9" asChild>
					<EnvironmentNavLink to="/settings/general" end>
						<SlidersHorizontal />
						<span>{t("navigation.generalSettings")}</span>
					</EnvironmentNavLink>
				</Button>
				<Button variant="navigation" size="navigation-compact" className="min-h-9" asChild>
					<EnvironmentNavLink to="/settings/credentials" end>
						<KeyRound />
						<span>{t("global.title")}</span>
					</EnvironmentNavLink>
				</Button>
				<Button variant="navigation" size="navigation-compact" className="min-h-9" asChild>
					<EnvironmentNavLink to="/settings/linear" end>
						<ClipboardList />
						<span>{t("linear.settings")}</span>
					</EnvironmentNavLink>
				</Button>
				<Button variant="navigation" size="navigation-compact" className="min-h-9" asChild>
					<EnvironmentNavLink to="/settings/code-hosts" end>
						<GitBranch />
						<span>{t("codeHosts.settings")}</span>
					</EnvironmentNavLink>
				</Button>
				<Button variant="navigation" size="navigation-compact" className="min-h-9" asChild>
					<EnvironmentNavLink to="/settings/gateway" end>
						<Network />
						<span>{t("gateway.settings")}</span>
					</EnvironmentNavLink>
				</Button>
				<Button variant="navigation" size="navigation-compact" className="min-h-9" asChild>
					<EnvironmentNavLink to="/settings/openviking" end>
						<BookOpen />
						<span>{t("openviking.settings")}</span>
					</EnvironmentNavLink>
				</Button>
			</nav>
		</>
	);
}

function WorkspaceSidebar({
	entryId: logicalEntryId,
	active,
}: {
	entryId: string;
	active: boolean;
}) {
	const { t } = useTranslation();
	const { projectName } = useParams();
	const { search } = useLocation();
	const navigate = useNavigate();
	const { workspace, overview, available, contextEntryId: entryId } = useWorkspace(logicalEntryId);
	const projects = overview.data?.projects ?? [];
	const [createOpen, setCreateOpen] = useState(false);
	const [pendingAction, setPendingAction] = useState<(() => void) | null>(null);
	const dirty = useEnvironmentDirtyStore((state) => state.dirty);
	const discardAll = useEnvironmentDirtyStore((state) => state.discardAll);
	const readOnly = workspace?.status === "identity-conflict";
	function requestAction(action: () => void) {
		if (dirty) setPendingAction(() => action);
		else action();
	}
	return (
		<>
			<div className="flex shrink-0 flex-col gap-1 px-5 py-4">
				<h2 className="truncate text-base font-semibold" title={workspace?.name}>
					{workspace?.name ?? t("workspaces.workspaceLabel")}
				</h2>
				<p
					className="truncate font-mono text-xs leading-5 text-muted-foreground"
					title={workspace?.root}
				>
					{workspace?.root}
				</p>
			</div>
			<nav
				className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-3 pb-3"
				aria-label={t("navigation.workspace")}
			>
				{available && !overview.data && !overview.error && <Skeleton className="h-24" />}
				<Button variant="navigation" size="navigation-compact" asChild>
					<EnvironmentNavLink
						to={
							new URLSearchParams(search).get("group")
								? `/projects/${encodeURIComponent(new URLSearchParams(search).get("group")!)}/issues`
								: `/workspaces/${encodeURIComponent(logicalEntryId)}/issues`
						}
					>
						<ClipboardList />
						<span>{t("linear.issues")}</span>
					</EnvironmentNavLink>
				</Button>
				{overview.error && (
					<Button variant="outline" onClick={() => void overview.mutate()}>
						{t("workspaces.retry")}
					</Button>
				)}
				<Button variant="navigation" size="navigation-compact" asChild>
					<EnvironmentNavLink to={`${workspacePath(entryId)}/git/worktrees`}>
						<GitBranch />
						<span>{t("git.title")}</span>
					</EnvironmentNavLink>
				</Button>
				<Button variant="navigation" size="navigation-compact" asChild>
					<EnvironmentNavLink to={workspaceVariablesPath(entryId, projectName)}>
						<KeyRound />
						<span>{t("navigation.environmentVariables")}</span>
					</EnvironmentNavLink>
				</Button>
				<Button variant="navigation" size="navigation-compact" asChild>
					<EnvironmentNavLink to={workspaceKnowledgePath(entryId)}>
						<BookOpen />
						<span>{t("openviking.title")}</span>
					</EnvironmentNavLink>
				</Button>
			</nav>
			{available && (
				<div className="flex shrink-0 flex-col gap-2 border-t border-border p-3">
					<Button
						disabled={readOnly || !overview.data}
						onClick={() => requestAction(() => setCreateOpen(true))}
					>
						<Plus />
						{t("projectCreate.title")}
					</Button>
				</div>
			)}
			{active && createOpen && !readOnly && (
				<CreateProjectDialog
					entryId={entryId}
					projectNames={projects.map((project) => project.name)}
					onClose={() => setCreateOpen(false)}
					onCreated={(name) => {
						setCreateOpen(false);
						navigate(preserveEnvironment(workspaceVariablesPath(entryId, name), search));
					}}
				/>
			)}
			<DiscardDialog
				open={active && pendingAction !== null}
				onOpenChange={(open) => {
					if (!open) setPendingAction(null);
				}}
				onDiscard={() => {
					const action = pendingAction;
					setPendingAction(null);
					discardAll();
					action?.();
				}}
			/>
		</>
	);
}
