import { AlertTriangle, ArrowLeft, FolderX, RefreshCw } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { Navigate, type RouteObject, useLocation, useParams, useRoutes } from "react-router-dom";
import useSWR from "swr";
import { getOverview, overviewKeyFor } from "@/api/workspace";
import { getWorkspaces, workspacesKey } from "@/api/workspaces";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { StatePanel } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import {
	environmentFromSearch,
	preserveEnvironment,
} from "@/features/environment-context/environment";
import { Overview } from "@/pages/Overview";
import { AccountSettings } from "@/features/infisical-session/AccountSettings";
import { GlobalVariables } from "@/features/global-variables/GlobalVariables";

import { ProjectTemplates } from "@/pages/ProjectTemplates";
import { WorkspaceHome } from "@/pages/WorkspaceHome";
import type { WorkspaceRegistryEntry } from "@/types/api";

const NotFoundRoute: React.FC = () => {
	const { t } = useTranslation();
	return (
		<Card>
			<StatePanel
				icon={FolderX}
				heading="h1"
				title={t("notFound.title")}
				description={t("notFound.description")}
			>
				<Button asChild>
					<EnvironmentLink to="/">
						<ArrowLeft />
						{t("workspaces.unknown.back")}
					</EnvironmentLink>
				</Button>
			</StatePanel>
		</Card>
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

	if (registry.isLoading && !registry.data)
		return (
			<WorkspaceStateLayout>
				<WorkspaceLoading />
			</WorkspaceStateLayout>
		);
	if (registry.error)
		return (
			<WorkspaceStateLayout>
				<WorkspaceRegistryError onRetry={() => void registry.mutate()} />
			</WorkspaceStateLayout>
		);
	if (!workspace)
		return (
			<WorkspaceStateLayout>
				<UnknownWorkspace />
			</WorkspaceStateLayout>
		);
	if (workspace.status !== "ready" && workspace.status !== "identity-conflict") {
		return (
			<WorkspaceStateLayout>
				<WorkspaceStatusPage workspace={workspace} />
			</WorkspaceStateLayout>
		);
	}
	if (overview.error) {
		return (
			<WorkspaceStateLayout>
				<WorkspaceLoadError workspace={workspace} onRetry={() => void overview.mutate()} />
			</WorkspaceStateLayout>
		);
	}
	if (overview.isLoading || !overview.data)
		return (
			<WorkspaceStateLayout>
				<WorkspaceLoading />
			</WorkspaceStateLayout>
		);

	return (
		<Overview
			data={overview.data}
			workspaceEntryId={workspace.entryId}
			readOnly={workspace.status === "identity-conflict"}
		/>
	);
};

const WorkspaceStateLayout: React.FC<React.PropsWithChildren> = ({ children }) => (
	<div className="h-full overflow-y-auto p-4 ud-md:p-6">
		<div className="mx-auto max-w-5xl">{children}</div>
	</div>
);

const WorkspaceLoading: React.FC = () => {
	const { t } = useTranslation();
	return (
		<div className="w-full space-y-3" role="status" aria-label={t("workspaces.loading")}>
			<Skeleton className="h-11 w-full rounded-lg" />
			<Skeleton className="h-64 w-full rounded-lg opacity-75" />
		</div>
	);
};

const WorkspaceRegistryError: React.FC<{ onRetry(): void }> = ({ onRetry }) => {
	const { t } = useTranslation();
	return (
		<Card className="w-full rounded-lg border-error-border">
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
		<Card className="w-full rounded-lg">
			<CardContent className="grid min-h-80 place-items-center p-6 text-center">
				<div className="max-w-lg">
					<div className="mx-auto grid h-12 w-12 place-items-center border border-warning-border bg-warning-surface text-warning-foreground">
						<FolderX className="h-5 w-5" />
					</div>
					<p className="mt-4 text-xs font-medium text-muted-foreground">
						{t("workspaces.workspaceLabel")}
					</p>
					<h1 className="mt-1 text-xl font-semibold">{workspace.name}</h1>
					<p className="mt-2 break-all rounded-md bg-muted/45 px-3 py-2 font-mono text-xs text-muted-foreground">
						{workspace.root}
					</p>
					<h2 className="mt-6 text-sm font-semibold">
						{t(`workspaces.state.${workspace.status}.title`)}
					</h2>
					<p className="mt-2 text-sm leading-relaxed text-muted-foreground">
						{t(`workspaces.state.${workspace.status}.description`)}
					</p>
					<p className="mt-5 text-xs text-muted-foreground">{t("workspaces.forget.pageHint")}</p>
					<Button asChild variant="outline" className="mt-5">
						<EnvironmentLink to="/">
							<ArrowLeft />
							{t("workspaces.unknown.back")}
						</EnvironmentLink>
					</Button>
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
		<Card className="w-full rounded-lg border-error-border">
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
		<Card className="w-full rounded-lg">
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
	{ path: "/templates", element: <ProjectTemplates /> },
	{ path: "/workspace/:entryId", element: <WorkspaceRoute /> },
	{ path: "/settings", element: <AccountSettings /> },
	{ path: "/global", element: <GlobalVariables /> },
	{ path: "/settings/:domain/:backend", element: <LegacyProfileRedirect /> },
	{ path: "/profile", element: <LegacyProfileRedirect /> },
	{ path: "/section/:domain/:backend", element: <LegacySectionRedirect /> },
	{ path: "*", element: <NotFoundRoute /> },
];

export const AppRoutes: React.FC = () => useRoutes(routes);
