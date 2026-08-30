import { AlertTriangle, ArrowUpRight, FolderGit2, FolderPlus } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { getWorkspaces, workspacesKey } from "@/api/workspaces";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { cn } from "@/lib/utils";
import type { WorkspaceRegistryEntry, WorkspaceRegistryStatus } from "@/types/api";

const STATUS_BADGE_CLASS: Record<WorkspaceRegistryStatus, string> = {
	ready: "border-success-border bg-success-surface text-success-foreground",
	missing: "border-border bg-muted text-muted-foreground",
	invalid: "border-error-border bg-error-surface text-error-foreground",
	"identity-missing": "border-warning-border bg-warning-surface text-warning-foreground",
	"identity-conflict": "border-warning-border bg-warning-surface text-warning-foreground",
};

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
	current: boolean;
}> = ({ workspace, current }) => {
	const { t, i18n } = useTranslation();
	const locale = i18n.resolvedLanguage ?? i18n.language;
	const projectCountUnavailable =
		(workspace.status === "missing" || workspace.status === "invalid") &&
		workspace.projectCount === 0;

	return (
		<EnvironmentLink
			to={`/workspace/${encodeURIComponent(workspace.entryId)}`}
			className="group rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
		>
			<Card
				className={cn(
					"relative h-full overflow-hidden transition-[border-color,box-shadow] group-hover:border-primary/40 group-hover:shadow-md",
					current && "border-primary/30",
				)}
			>
				{current ? (
					<span className="absolute inset-y-0 left-0 w-1 bg-primary" aria-hidden="true" />
				) : null}
				<CardHeader className="gap-3 pb-3">
					<div className="flex items-start justify-between gap-4">
						<div className="min-w-0 space-y-1.5">
							<div className="flex flex-wrap items-center gap-2">
								<CardTitle className="truncate">{workspace.name}</CardTitle>
								{current ? (
									<Badge className="uppercase tracking-[0.08em]">
										{t("workspaces.home.currentSession", {
											defaultValue: "This one serve session",
										})}
									</Badge>
								) : null}
							</div>
							<p
								className="truncate font-mono text-[11px] text-muted-foreground"
								title={workspace.root}
							>
								{workspace.root}
							</p>
						</div>
						<div className="flex shrink-0 items-center gap-2">
							<Badge variant="outline" className={STATUS_BADGE_CLASS[workspace.status]}>
								{t(`workspaces.status.${workspace.status}`, { defaultValue: workspace.status })}
							</Badge>
							<ArrowUpRight className="h-4 w-4 text-muted-foreground transition-colors group-hover:text-primary" />
						</div>
					</div>
				</CardHeader>
				<CardContent>
					<dl className="grid grid-cols-[minmax(0,1fr)_5rem_minmax(9rem,auto)] gap-4 border-t border-border/70 pt-3">
						<div className="min-w-0">
							<dt className="text-[10px] font-medium tracking-[0.12em] text-muted-foreground uppercase">
								{t("workspaces.home.workspaceId", { defaultValue: "Workspace ID" })}
							</dt>
							<dd className="mt-1 truncate font-mono text-xs" title={workspace.id}>
								{workspace.id ??
									t("workspaces.home.idUnavailable", { defaultValue: "Not available" })}
							</dd>
						</div>
						<div>
							<dt className="text-[10px] font-medium tracking-[0.12em] text-muted-foreground uppercase">
								{t("workspaces.home.projects", { defaultValue: "Projects" })}
							</dt>
							<dd className="mt-1 font-mono text-xs font-semibold">
								{projectCountUnavailable
									? t("workspaces.home.projectCountUnavailable", {
											defaultValue: "Unavailable",
										})
									: workspace.projectCount}
							</dd>
						</div>
						<div>
							<dt className="text-[10px] font-medium tracking-[0.12em] text-muted-foreground uppercase">
								{t("workspaces.home.lastDetected", { defaultValue: "Last detected" })}
							</dt>
							<dd className="mt-1 text-xs">
								<time dateTime={workspace.lastSeenAt}>
									{formatLastSeen(workspace.lastSeenAt, locale)}
								</time>
							</dd>
						</div>
					</dl>
				</CardContent>
			</Card>
		</EnvironmentLink>
	);
};

export const WorkspaceHome: React.FC = () => {
	const { t } = useTranslation();
	const registry = useSWR(workspacesKey, getWorkspaces);

	if (registry.isLoading) {
		return (
			<div role="status" className="space-y-4">
				<span className="sr-only">
					{t("workspaces.home.loading", { defaultValue: "Loading Workspaces…" })}
				</span>
				<Skeleton className="h-20 w-full rounded-lg" />
				<div className="grid gap-4 xl:grid-cols-2">
					<Skeleton className="h-40 w-full rounded-lg" />
					<Skeleton className="h-40 w-full rounded-lg opacity-70" />
				</div>
			</div>
		);
	}
	if (registry.error) {
		const message = (registry.error as { message?: string }).message;
		return (
			<Alert variant="destructive">
				<AlertTriangle className="h-4 w-4" />
				<div>
					<AlertTitle>
						{t("workspaces.home.loadFailedTitle", {
							defaultValue: "Could not load Workspaces",
						})}
					</AlertTitle>
					<AlertDescription className="mt-1">
						<p>
							{message ??
								t("workspaces.home.loadFailedDescription", {
									defaultValue: "The local Workspace registry could not be read.",
								})}
						</p>
						<Button
							type="button"
							variant="outline"
							size="sm"
							className="mt-3"
							onClick={() => void registry.mutate()}
						>
							{t("workspaces.home.retry", { defaultValue: "Retry" })}
						</Button>
					</AlertDescription>
				</div>
			</Alert>
		);
	}

	const workspaces = registry.data?.workspaces ?? [];
	const projectCount = workspaces.reduce((total, workspace) => total + workspace.projectCount, 0);

	return (
		<div className="space-y-7">
			<header className="flex items-end justify-between gap-8 border-b border-border/70 pb-5">
				<div className="space-y-1">
					<div className="flex items-center gap-2">
						<FolderGit2 className="h-5 w-5 text-primary" />
						<h1 className="text-xl font-semibold tracking-tight">
							{t("workspaces.home.title", { defaultValue: "Workspaces" })}
						</h1>
					</div>
					<p className="max-w-2xl text-sm text-muted-foreground">
						{t("workspaces.home.description", {
							defaultValue: "Open any Workspace registered on this machine.",
						})}
					</p>
				</div>
				<div className="flex shrink-0 items-center gap-4 rounded-md border border-border bg-muted/35 px-4 py-2 text-xs">
					<span className="font-medium">
						{t("workspaces.home.registryCount", {
							count: workspaces.length,
							defaultValue: "{{count}} registered Workspaces",
						})}
					</span>
					<Separator orientation="vertical" className="h-4" />
					<span className="font-medium">
						{t("workspaces.home.projectCount", {
							count: projectCount,
							defaultValue: "{{count}} Projects",
						})}
					</span>
				</div>
			</header>

			{workspaces.length === 0 ? (
				<Empty className="min-h-64 border border-dashed border-border bg-muted/20 px-8 py-14">
					<EmptyHeader>
						<EmptyMedia variant="icon">
							<FolderPlus className="h-5 w-5 text-primary" />
						</EmptyMedia>
						<EmptyTitle>
							<h2>{t("workspaces.home.emptyTitle", { defaultValue: "No Workspaces yet" })}</h2>
						</EmptyTitle>
						<EmptyDescription className="max-w-lg leading-6">
							{t("workspaces.home.emptyDescription", {
								defaultValue:
									"Run one create, or run one serve inside a Workspace, to register it here.",
							})}
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			) : (
				<section
					aria-label={t("workspaces.home.listLabel", {
						defaultValue: "Registered Workspaces",
					})}
					className="grid gap-4 xl:grid-cols-2"
				>
					{workspaces.map((workspace) => (
						<WorkspaceCard
							key={workspace.entryId}
							workspace={workspace}
							current={workspace.entryId === registry.data?.currentEntryId}
						/>
					))}
				</section>
			)}
		</div>
	);
};
