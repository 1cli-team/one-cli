import { AlertTriangle, FolderX, RefreshCw } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { Navigate, type RouteObject, useLocation, useParams, useRoutes } from "react-router-dom";
import useSWR from "swr";
import { getOverview, overviewKeyFor } from "@/api/workspace";
import { getWorkspaces, workspacesKey } from "@/api/workspaces";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Empty, EmptyDescription, EmptyHeader } from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import {
	environmentFromSearch,
	preserveEnvironment,
} from "@/features/environment-context/environment";
import { Overview } from "@/pages/Overview";
import { SectionDetail } from "@/pages/SectionDetail";
import { SectionsHome } from "@/pages/SectionsHome";
import { WorkspaceHome } from "@/pages/WorkspaceHome";
import type { WorkspaceRegistryEntry } from "@/types/api";

const NotFoundRoute: React.FC = () => {
	const { t } = useTranslation();
	return (
		<Empty className="min-h-80 border border-dashed border-border">
			<EmptyHeader>
				<EmptyDescription>{t("notFound.message")}</EmptyDescription>
			</EmptyHeader>
		</Empty>
	);
};

const LegacySectionRedirect: React.FC = () => {
	const { search } = useLocation();
	const { domain = "", backend = "" } = useParams<{
		domain: string;
		backend: string;
	}>();
	return (
		<Navigate
			replace
			to={preserveEnvironment(
				`/settings/${encodeURIComponent(domain)}/${encodeURIComponent(backend)}`,
				search,
			)}
		/>
	);
};

const LegacyProfileRedirect: React.FC = () => {
	const { search } = useLocation();
	return <Navigate replace to={preserveEnvironment("/settings", search)} />;
};

const WorkspaceRoute: React.FC = () => {
	const { entryId = "" } = useParams<{ entryId: string }>();
	const { search } = useLocation();
	const environment = environmentFromSearch(search);
	const registry = useSWR(workspacesKey, getWorkspaces);
	const workspace = registry.data?.workspaces.find((entry) => entry.entryId === entryId);
	const canLoadOverview =
		workspace?.status === "ready" || workspace?.status === "identity-conflict";
	const overviewKey = entryId && canLoadOverview ? overviewKeyFor(entryId, environment) : null;
	const overview = useSWR(overviewKey, () => getOverview(entryId, environment), {
		shouldRetryOnError: false,
	});

	if (registry.isLoading && !registry.data) return <WorkspaceLoading />;
	if (registry.error) return <WorkspaceRegistryError onRetry={() => void registry.mutate()} />;
	if (!workspace) return <UnknownWorkspace />;
	if (workspace.status !== "ready" && workspace.status !== "identity-conflict") {
		return <WorkspaceStatusPage workspace={workspace} />;
	}
	if (overview.error) {
		return <WorkspaceLoadError workspace={workspace} onRetry={() => void overview.mutate()} />;
	}
	if (overview.isLoading || !overview.data) return <WorkspaceLoading />;

	return (
		<Overview
			data={overview.data}
			workspaceEntryId={workspace.entryId}
			readOnly={workspace.status === "identity-conflict"}
		/>
	);
};

const WorkspaceLoading: React.FC = () => {
	const { t } = useTranslation();
	return (
		<div className="w-full space-y-3" role="status" aria-label={t("workspaces.loading")}>
			<Skeleton className="h-11 w-full rounded-[6px]" />
			<Skeleton className="h-64 w-full rounded-[6px] opacity-75" />
		</div>
	);
};

const WorkspaceRegistryError: React.FC<{ onRetry(): void }> = ({ onRetry }) => {
	const { t } = useTranslation();
	return (
		<Card className="w-full rounded-[6px] border-error-border">
			<CardContent className="grid min-h-80 place-items-center p-6 text-center">
				<div className="max-w-md">
					<AlertTriangle className="mx-auto h-8 w-8 text-error-foreground" />
					<h1 className="mt-4 text-lg font-semibold">{t("workspaces.registryError.title")}</h1>
					<p className="mt-2 text-sm leading-relaxed text-muted-foreground">
						{t("workspaces.registryError.description")}
					</p>
					<div className="mt-5 flex justify-center gap-2">
						<Button variant="outline" onClick={onRetry}>
							<RefreshCw />
							{t("workspaces.retry")}
						</Button>
						<Button asChild>
							<EnvironmentLink to="/settings">{t("workspaces.openProfiles")}</EnvironmentLink>
						</Button>
					</div>
				</div>
			</CardContent>
		</Card>
	);
};

const WorkspaceStatusPage: React.FC<{ workspace: WorkspaceRegistryEntry }> = ({ workspace }) => {
	const { t } = useTranslation();
	return (
		<Card className="w-full rounded-[6px]">
			<CardContent className="grid min-h-80 place-items-center p-6 text-center">
				<div className="max-w-lg">
					<div className="mx-auto grid h-12 w-12 place-items-center border border-warning-border bg-warning-surface text-warning-foreground">
						<FolderX className="h-5 w-5" />
					</div>
					<p className="mt-4 text-[10px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
						{t("workspaces.workspaceLabel")}
					</p>
					<h1 className="mt-1 text-xl font-semibold">{workspace.name}</h1>
					<p className="mt-2 truncate rounded-md bg-muted/45 px-3 py-2 font-mono text-[11px] text-muted-foreground">
						{workspace.root}
					</p>
					<h2 className="mt-6 text-sm font-semibold">
						{t(`workspaces.state.${workspace.status}.title`)}
					</h2>
					<p className="mt-2 text-sm leading-relaxed text-muted-foreground">
						{t(`workspaces.state.${workspace.status}.description`)}
					</p>
					<p className="mt-5 text-xs text-muted-foreground">{t("workspaces.forget.pageHint")}</p>
				</div>
			</CardContent>
		</Card>
	);
};

const WorkspaceLoadError: React.FC<{
	workspace: WorkspaceRegistryEntry;
	onRetry(): void;
}> = ({ workspace, onRetry }) => {
	const { t } = useTranslation();
	return (
		<Card className="w-full rounded-[6px] border-error-border">
			<CardContent className="grid min-h-80 place-items-center p-6 text-center">
				<div className="max-w-md">
					<AlertTriangle className="mx-auto h-8 w-8 text-error-foreground" />
					<h1 className="mt-4 text-lg font-semibold">
						{t("workspaces.overviewError.title", { name: workspace.name })}
					</h1>
					<p className="mt-2 text-sm text-muted-foreground">
						{t("workspaces.overviewError.description")}
					</p>
					<Button variant="outline" className="mt-5" onClick={onRetry}>
						<RefreshCw />
						{t("workspaces.retry")}
					</Button>
				</div>
			</CardContent>
		</Card>
	);
};

const UnknownWorkspace: React.FC = () => {
	const { t } = useTranslation();
	return (
		<Card className="w-full rounded-[6px]">
			<CardContent className="grid min-h-80 place-items-center p-6 text-center">
				<div>
					<FolderX className="mx-auto h-8 w-8 text-muted-foreground" />
					<h1 className="mt-4 text-lg font-semibold">{t("workspaces.unknown.title")}</h1>
					<p className="mt-2 text-sm text-muted-foreground">
						{t("workspaces.unknown.description")}
					</p>
					<Button asChild variant="outline" className="mt-5">
						<EnvironmentLink to="/">{t("workspaces.unknown.back")}</EnvironmentLink>
					</Button>
				</div>
			</CardContent>
		</Card>
	);
};

const routes: RouteObject[] = [
	{ path: "/", element: <WorkspaceHome /> },
	{ path: "/workspace/:entryId", element: <WorkspaceRoute /> },
	{ path: "/settings", element: <SectionsHome /> },
	{ path: "/settings/:domain/:backend", element: <SectionDetail /> },
	{ path: "/profile", element: <LegacyProfileRedirect /> },
	{ path: "/section/:domain/:backend", element: <LegacySectionRedirect /> },
	{ path: "*", element: <NotFoundRoute /> },
];

export const AppRoutes: React.FC = () => useRoutes(routes);
