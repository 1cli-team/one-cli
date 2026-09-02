import { FolderGit2, Trash2 } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useMatch, useNavigate } from "react-router-dom";
import useSWR from "swr";
import { forgetWorkspace, getWorkspaces, workspacesKey } from "@/api/workspaces";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { preserveEnvironment } from "@/features/environment-context/environment";
import { useToast } from "@/hooks/useToast";
import { cn } from "@/lib/utils";
import type { WorkspaceRegistryEntry, WorkspaceRegistryStatus } from "@/types/api";

const STATUS_DOT_CLASS: Record<WorkspaceRegistryStatus, string> = {
	ready: "bg-success-500",
	missing: "bg-gray-400",
	invalid: "bg-error-500",
	"identity-missing": "bg-warning-500",
	"identity-conflict": "bg-warning-500",
};

const WorkspaceRailItem: React.FC<{
	workspace: WorkspaceRegistryEntry;
	active: boolean;
	forgetting: boolean;
	onForget(): void;
}> = ({ workspace, active, forgetting, onForget }) => {
	const { t } = useTranslation();

	return (
		<div
			className={cn(
				"group relative rounded-md border border-transparent transition-colors",
				active ? "border-primary/15 bg-primary/9" : "hover:bg-accent/60",
			)}
		>
			{active ? (
				<span className="absolute inset-y-2 -left-1.5 w-0.5 rounded-full bg-primary" aria-hidden />
			) : null}
			<EnvironmentLink
				to={`/workspace/${encodeURIComponent(workspace.entryId)}`}
				className="block min-w-0 px-2 py-2.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
				aria-current={active ? "page" : undefined}
				title={`${workspace.name}\n${workspace.root}`}
			>
				<span className="flex min-w-0 items-center gap-2">
					<span
						className={cn("h-2 w-2 shrink-0 rounded-full", STATUS_DOT_CLASS[workspace.status])}
						aria-label={t(`workspaces.status.${workspace.status}`)}
						title={t(`workspaces.status.${workspace.status}`)}
					/>
					<span
						className={cn("min-w-0 flex-1 truncate text-xs font-medium", active && "text-primary")}
					>
						{workspace.name}
					</span>
					<span
						className="shrink-0 rounded bg-muted px-1.5 py-0.5 font-mono text-[9px] text-muted-foreground transition-opacity group-hover:opacity-0 group-focus-within:opacity-0"
						title={`${workspace.projectCount} ${t("overview.metrics.projects")}`}
					>
						{workspace.projectCount}
					</span>
				</span>
				<span className="mt-1 block break-all pl-4 font-mono text-[9px] leading-[1.45] text-muted-foreground">
					{workspace.root}
				</span>
			</EnvironmentLink>
			<Tooltip>
				<TooltipTrigger asChild>
					<Button
						type="button"
						variant="ghost"
						size="icon"
						className="absolute right-1 top-1.5 h-6 w-6 text-muted-foreground opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
						onClick={onForget}
						disabled={forgetting}
						aria-label={t("workspaces.forget.action", { name: workspace.name })}
					>
						<Trash2 className="h-3 w-3" />
					</Button>
				</TooltipTrigger>
				<TooltipContent side="right">{t("workspaces.forget.short")}</TooltipContent>
			</Tooltip>
		</div>
	);
};

export const WorkspaceRail: React.FC = () => {
	const { t } = useTranslation();
	const toast = useToast();
	const navigate = useNavigate();
	const { search } = useLocation();
	const workspaceMatch = useMatch("/workspace/:entryId");
	const activeEntryId = workspaceMatch?.params.entryId;
	const registry = useSWR(workspacesKey, getWorkspaces);
	const [forgetting, setForgetting] = useState<string | null>(null);
	const [workspaceToForget, setWorkspaceToForget] = useState<WorkspaceRegistryEntry | null>(null);
	const workspaces = registry.data?.workspaces ?? [];

	async function handleForget(workspace: WorkspaceRegistryEntry) {
		setForgetting(workspace.entryId);
		try {
			await forgetWorkspace(workspace.entryId);
			await registry.mutate(
				(current) =>
					current
						? {
								...current,
								currentEntryId:
									current.currentEntryId === workspace.entryId ? undefined : current.currentEntryId,
								workspaces: current.workspaces.filter(
									(entry) => entry.entryId !== workspace.entryId,
								),
							}
						: current,
				{ revalidate: false },
			);
			if (activeEntryId === workspace.entryId) {
				navigate(preserveEnvironment("/", search), { replace: true });
			}
			toast.success(t("workspaces.forget.done", { name: workspace.name }));
			setWorkspaceToForget(null);
		} catch (error) {
			const failure = error as { message?: string };
			toast.error(t("workspaces.forget.failed"), {
				description: failure.message ?? String(error),
			});
		} finally {
			setForgetting(null);
		}
	}

	return (
		<>
			<section className="flex min-h-0 flex-1 flex-col border-t border-border/70 px-3 py-4">
				<div className="mb-2 flex items-center justify-between px-2">
					<div className="flex items-center gap-1.5">
						<FolderGit2 className="h-3.5 w-3.5 text-primary" />
						<h2 className="text-[10px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
							{t("workspaces.rail.title")}
						</h2>
					</div>
					{workspaces.length > 0 ? (
						<span className="font-mono text-[9px] text-muted-foreground">{workspaces.length}</span>
					) : null}
				</div>

				<ScrollArea className="min-h-0 flex-1">
					<div className="space-y-1.5 px-0.5 py-0.5">
						{registry.isLoading ? <WorkspaceRailLoading /> : null}
						{registry.error ? (
							<p className="px-2 py-3 text-[10px] leading-relaxed text-error-foreground">
								{t("workspaces.rail.loadFailed")}
							</p>
						) : null}
						{!registry.isLoading && !registry.error && workspaces.length === 0 ? (
							<p className="px-2 py-3 text-[10px] leading-relaxed text-muted-foreground">
								{t("workspaces.rail.empty")}
							</p>
						) : null}
						{workspaces.map((workspace) => (
							<WorkspaceRailItem
								key={workspace.entryId}
								workspace={workspace}
								active={workspace.entryId === activeEntryId}
								forgetting={workspace.entryId === forgetting}
								onForget={() => setWorkspaceToForget(workspace)}
							/>
						))}
					</div>
				</ScrollArea>
			</section>

			<AlertDialog
				open={workspaceToForget !== null}
				onOpenChange={(open) => {
					if (!open && !forgetting) setWorkspaceToForget(null);
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>{t("workspaces.forget.short")}</AlertDialogTitle>
						<AlertDialogDescription>
							{workspaceToForget
								? t("workspaces.forget.confirm", { name: workspaceToForget.name })
								: ""}
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={Boolean(forgetting)}>{t("form.cancel")}</AlertDialogCancel>
						<AlertDialogAction
							variant="destructive"
							disabled={Boolean(forgetting)}
							onClick={(event) => {
								event.preventDefault();
								if (workspaceToForget) void handleForget(workspaceToForget);
							}}
						>
							{t("workspaces.forget.short")}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</>
	);
};

const WorkspaceRailLoading: React.FC = () => {
	const { t } = useTranslation();
	return (
		<div className="space-y-2 px-2 py-1" aria-label={t("workspaces.loading")}>
			<Skeleton className="h-10 w-full rounded-md" />
			<Skeleton className="h-10 w-full rounded-md opacity-70" />
		</div>
	);
};
