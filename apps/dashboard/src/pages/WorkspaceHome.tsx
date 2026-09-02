import { AlertTriangle, FolderGit2, FolderPlus, Trash2 } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { forgetWorkspace, getWorkspaces, workspacesKey } from "@/api/workspaces";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
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
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { useToast } from "@/hooks/useToast";
import type { WorkspaceRegistryEntry } from "@/types/api";

function formatLastSeen(value: string, locale: string): string {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return value;
	return new Intl.DateTimeFormat(locale, {
		dateStyle: "medium",
		timeStyle: "short",
	}).format(date);
}

const WorkspaceCard: React.FC<{
	workspace: WorkspaceRegistryEntry;
	onForget(): void;
}> = ({ workspace, onForget }) => {
	const { t, i18n } = useTranslation();
	const locale = i18n.resolvedLanguage ?? i18n.language;
	const countUnavailable =
		(workspace.status === "missing" || workspace.status === "invalid") &&
		workspace.projectCount === 0;

	return (
		<article className="group relative min-h-52 overflow-hidden rounded-xl border border-border bg-card shadow-sm transition-[border-color,box-shadow] duration-200 hover:border-primary/30 hover:shadow-md">
			<EnvironmentLink
				to={`/workspace/${encodeURIComponent(workspace.entryId)}`}
				className="flex h-full min-h-52 flex-col p-5 pr-14 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
			>
				<div className="flex items-start gap-3.5">
					<span className="grid size-10 shrink-0 place-items-center rounded-lg border border-primary/15 bg-primary/8 text-primary">
						<FolderGit2 className="size-[18px]" />
					</span>
					<div className="min-w-0 flex-1">
						<h2 className="truncate text-base font-semibold tracking-tight">{workspace.name}</h2>
					</div>
				</div>

				<div className="mt-6">
					<div>
						<p className="font-mono text-[10px] font-semibold tracking-[0.1em] text-muted-foreground uppercase">
							{t("workspaces.home.projects")}
						</p>
						<p className="mt-1.5 font-heading text-4xl font-semibold leading-none tracking-tight">
							{countUnavailable ? "-" : workspace.projectCount}
						</p>
					</div>
				</div>

				<div className="mt-auto flex items-center justify-between gap-3 border-t border-border/70 pt-4 text-[11px] text-muted-foreground">
					<time dateTime={workspace.lastSeenAt} className="truncate">
						{t("workspaces.home.lastDetected")} · {formatLastSeen(workspace.lastSeenAt, locale)}
					</time>
				</div>
			</EnvironmentLink>
			<Button
				type="button"
				variant="ghost"
				size="icon-sm"
				className="absolute top-4 right-4 text-muted-foreground opacity-65 hover:bg-error-surface hover:text-error-foreground group-hover:opacity-100"
				aria-label={t("workspaces.forget.action", { name: workspace.name })}
				onClick={onForget}
			>
				<Trash2 />
			</Button>
		</article>
	);
};

export const WorkspaceHome: React.FC = () => {
	const { t } = useTranslation();
	const toast = useToast();
	const registry = useSWR(workspacesKey, getWorkspaces);
	const [workspaceToForget, setWorkspaceToForget] = useState<WorkspaceRegistryEntry | null>(null);
	const [forgetting, setForgetting] = useState(false);

	async function confirmForget() {
		if (!workspaceToForget || forgetting) return;
		setForgetting(true);
		try {
			await forgetWorkspace(workspaceToForget.entryId);
			toast.success(t("workspaces.forget.done", { name: workspaceToForget.name }));
			setWorkspaceToForget(null);
			await registry.mutate();
		} catch (error) {
			toast.error(t("workspaces.forget.failed"), {
				description: (error as { message?: string }).message,
			});
		} finally {
			setForgetting(false);
		}
	}

	if (registry.isLoading) {
		return (
			<div role="status" className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
				<span className="sr-only">{t("workspaces.home.loading")}</span>
				<Skeleton className="h-52 w-full rounded-xl" />
				<Skeleton className="h-52 w-full rounded-xl opacity-80" />
				<Skeleton className="h-52 w-full rounded-xl opacity-60" />
			</div>
		);
	}
	if (registry.error) {
		const message = (registry.error as { message?: string }).message;
		return (
			<Alert variant="destructive" className="w-full rounded-xl">
				<AlertTriangle className="h-4 w-4" />
				<div>
					<AlertTitle>{t("workspaces.home.loadFailedTitle")}</AlertTitle>
					<AlertDescription className="mt-1">
						<p>{message ?? t("workspaces.home.loadFailedDescription")}</p>
						<Button
							type="button"
							variant="outline"
							size="sm"
							className="mt-3"
							onClick={() => void registry.mutate()}
						>
							{t("workspaces.home.retry")}
						</Button>
					</AlertDescription>
				</div>
			</Alert>
		);
	}

	const workspaces = registry.data?.workspaces ?? [];

	return (
		<div className="w-full space-y-6 pb-8">
			<header className="pb-1">
				<h1 className="font-heading text-3xl font-semibold tracking-tight">
					{t("workspaces.home.title")}
				</h1>
			</header>

			{workspaces.length === 0 ? (
				<Empty className="min-h-64 rounded-xl border border-dashed border-border bg-card px-6 py-10 shadow-sm">
					<EmptyHeader>
						<EmptyMedia variant="icon">
							<FolderPlus className="h-5 w-5 text-primary" />
						</EmptyMedia>
						<EmptyTitle>
							<h2>{t("workspaces.home.emptyTitle")}</h2>
						</EmptyTitle>
						<EmptyDescription className="max-w-lg leading-6">
							{t("workspaces.home.emptyDescription")}
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			) : (
				<section aria-label={t("workspaces.home.listLabel")}>
					<div className="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-4">
						{workspaces.map((workspace) => (
							<WorkspaceCard
								key={workspace.entryId}
								workspace={workspace}
								onForget={() => setWorkspaceToForget(workspace)}
							/>
						))}
					</div>
				</section>
			)}

			<AlertDialog
				open={Boolean(workspaceToForget)}
				onOpenChange={(open) => !open && !forgetting && setWorkspaceToForget(null)}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>
							{workspaceToForget
								? t("workspaces.forget.action", { name: workspaceToForget.name })
								: ""}
						</AlertDialogTitle>
						<AlertDialogDescription>
							{workspaceToForget
								? t("workspaces.forget.confirm", { name: workspaceToForget.name })
								: ""}
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={forgetting}>{t("form.cancel")}</AlertDialogCancel>
						<AlertDialogAction
							variant="destructive"
							disabled={forgetting}
							onClick={(event) => {
								event.preventDefault();
								void confirmForget();
							}}
						>
							{forgetting ? <Spinner /> : <Trash2 />}
							{workspaceToForget
								? t("workspaces.forget.action", { name: workspaceToForget.name })
								: ""}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	);
};
