import { DesktopSettings } from "@/lib/desktop/DesktopSettings";
import { AlertTriangle, ArrowLeft, FolderX, RefreshCw } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { Navigate, type RouteObject, useLocation, useParams, useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { PageHeader, StatePanel } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { Overview } from "@/pages/Overview";
import { App } from "@/App";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { useWorkspace } from "@/features/workspace-navigation/useWorkspace";
import { legacyWorkspaceLocation, projectPath, workspacePath } from "./paths";
import { AccountSettings } from "@/features/infisical-session/AccountSettings";
import { GlobalVariables } from "@/features/global-variables/GlobalVariables";

import { InitializeWorkspace } from "@/features/workspace-navigation/InitializeWorkspace";
import { AddWorkspaceDialog } from "@/features/workspace-navigation/AddWorkspaceDialog";
import { useState } from "react";
import { Worktrees } from "@/features/git/Worktrees";
import { preserveEnvironment } from "@/features/environment-context/environment";
import { WorkspaceGroupsPage } from "@/features/workspace-groups/WorkspaceGroupsPage";
import { WorkspaceHome } from "@/pages/WorkspaceHome";
import type { WorkspaceRegistryEntry } from "@/types/api";

const NotFoundRoute: React.FC = () => {
	const { t } = useTranslation();
	return (
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
	);
};

const LegacySettingsRedirect: React.FC = () => {
	const { search } = useLocation();
	return <Navigate replace to={{ pathname: "/settings/general", search }} />;
};

const LegacyWorkspaceRedirect: React.FC = () => {
	const { entryId = "" } = useParams();
	const { search } = useLocation();
	return <Navigate replace to={legacyWorkspaceLocation(entryId, search)} />;
};

const LegacyGlobalRedirect: React.FC<{ to: string }> = ({ to }) => {
	const { search } = useLocation();
	return <Navigate replace to={{ pathname: to, search }} />;
};

const WorkspaceRoute: React.FC = () => {
	const { entryId = "", projectName } = useParams();
	const { search } = useLocation();
	const { registry, workspace, overview, contextEntryId } = useWorkspace(entryId);

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
	if (workspace.status !== "ready" && workspace.status !== "identity-conflict")
		return (
			<WorkspaceStateLayout>
				<WorkspaceStatusPage workspace={workspace} />
			</WorkspaceStateLayout>
		);
	if (overview.error)
		return (
			<WorkspaceStateLayout>
				<WorkspaceLoadError workspace={workspace} onRetry={() => void overview.mutate()} />
			</WorkspaceStateLayout>
		);
	if (!overview.data)
		return (
			<WorkspaceStateLayout>
				<WorkspaceLoading />
			</WorkspaceStateLayout>
		);

	const projects = overview.data.projects ?? [];
	if (!projectName && projects.length) {
		const requested = new URLSearchParams(search).get("project");
		const location = legacyWorkspaceLocation(contextEntryId, search);
		return (
			<Navigate
				replace
				to={{ ...location, pathname: projectPath(contextEntryId, requested || projects[0].name) }}
			/>
		);
	}
	const project = projects.find((item) => item.name === projectName);
	if (projectName && !project)
		return (
			<WorkspaceStateLayout>
				<UnknownProject entryId={contextEntryId} />
			</WorkspaceStateLayout>
		);

	return (
		<Overview
			project={project}
			workspaceEntryId={contextEntryId}
			readOnly={workspace.status === "identity-conflict"}
		/>
	);
};

const UnknownProject: React.FC<{ entryId: string }> = ({ entryId }) => {
	const { t } = useTranslation();
	return (
		<StatePanel
			icon={FolderX}
			heading="h1"
			title={t("navigation.projectMissing")}
			description={t("navigation.projectMissingDescription")}
		>
			<Button asChild>
				<EnvironmentLink to={workspacePath(entryId)}>
					{t("navigation.backToWorkspace")}
				</EnvironmentLink>
			</Button>
		</StatePanel>
	);
};

const WorkspaceStateLayout: React.FC<React.PropsWithChildren> = ({ children }) => (
	<div className="h-full overflow-y-auto p-4 ud-md:p-6">
		<div className="mx-auto max-w-[1120px]">{children}</div>
	</div>
);

const WorkspaceLoading: React.FC = () => {
	const { t } = useTranslation();
	return (
		<div className="flex w-full flex-col gap-4" role="status" aria-label={t("workspaces.loading")}>
			<Skeleton className="h-11 w-full rounded-lg" />
			<Skeleton className="h-64 w-full rounded-lg opacity-75" />
		</div>
	);
};

const WorkspaceRegistryError: React.FC<{ onRetry(): void }> = ({ onRetry }) => {
	const { t } = useTranslation();
	return (
		<StatePanel
			icon={AlertTriangle}
			heading="h1"
			title={t("workspaces.registryError.title")}
			description={t("workspaces.registryError.description")}
		>
			<Button variant="outline" onClick={onRetry}>
				<RefreshCw />
				{t("workspaces.retry")}
			</Button>
			<Button asChild>
				<EnvironmentLink to="/settings">{t("workspaces.openProfiles")}</EnvironmentLink>
			</Button>
		</StatePanel>
	);
};

const WorkspaceStatusPage: React.FC<{ workspace: WorkspaceRegistryEntry }> = ({ workspace }) => {
	const { t } = useTranslation();
	const { search } = useLocation();
	const [relocating, setRelocating] = useState(false);
	const navigate = useNavigate();
	return (
		<div className="flex flex-col gap-6">
			<PageHeader title={workspace.name} />
			<p className="break-all font-mono text-sm text-muted-foreground">{workspace.root}</p>
			<StatePanel
				icon={FolderX}
				title={t(`workspaces.state.${workspace.status}.title`)}
				description={t(`workspaces.state.${workspace.status}.description`)}
			>
				<p className="w-full text-xs text-muted-foreground">{t("workspaces.forget.pageHint")}</p>
				{workspace.status === "uninitialized" && <InitializeWorkspace workspace={workspace} />}
				{workspace.gitCommonDir && (
					<Button variant="outline" asChild>
						<EnvironmentLink to={`${workspacePath(workspace.entryId)}/git/worktrees`}>
							{t("git.title")}
						</EnvironmentLink>
					</Button>
				)}
				{workspace.status === "missing" && (
					<Button variant="outline" onClick={() => setRelocating(true)}>
						{t("git.locateRepository")}
					</Button>
				)}
				{relocating && (
					<AddWorkspaceDialog
						workspace={workspace}
						onClose={() => setRelocating(false)}
						onSaved={(entry) => {
							setRelocating(false);
							navigate(preserveEnvironment(workspacePath(entry.entryId, entry.worktreeId), search));
						}}
					/>
				)}
				<Button asChild variant="outline">
					<EnvironmentLink to="/">
						<ArrowLeft />
						{t("workspaces.unknown.back")}
					</EnvironmentLink>
				</Button>
			</StatePanel>
		</div>
	);
};

const WorkspaceLoadError: React.FC<{ workspace: WorkspaceRegistryEntry; onRetry(): void }> = ({
	workspace,
	onRetry,
}) => {
	const { t } = useTranslation();
	return (
		<StatePanel
			icon={AlertTriangle}
			heading="h1"
			title={t("workspaces.overviewError.title", { name: workspace.name })}
			description={t("workspaces.overviewError.description")}
		>
			<Button variant="outline" onClick={onRetry}>
				<RefreshCw />
				{t("workspaces.retry")}
			</Button>
		</StatePanel>
	);
};

const UnknownWorkspace: React.FC = () => {
	const { t } = useTranslation();
	return (
		<StatePanel
			icon={FolderX}
			heading="h1"
			title={t("workspaces.unknown.title")}
			description={t("workspaces.unknown.description")}
		>
			<Button asChild variant="outline">
				<EnvironmentLink to="/">{t("workspaces.unknown.back")}</EnvironmentLink>
			</Button>
		</StatePanel>
	);
};

export const routes: RouteObject[] = [
	{
		element: (
			<ErrorBoundary>
				<App />
			</ErrorBoundary>
		),
		children: [
			{ index: true, element: <WorkspaceGroupsPage /> },
			{ path: "projects/:groupId", element: <WorkspaceGroupsPage /> },
			{ path: "workspaces", element: <WorkspaceHome /> },
			{ path: "git/repositories", element: <Navigate replace to="/" /> },
			{ path: "git/repositories/:repositoryId", element: <LegacyRepositoryRedirect /> },
			{ path: "credentials", element: <LegacyGlobalRedirect to="/settings/credentials" /> },
			{
				path: "settings",
				children: [
					{ index: true, element: <LegacySettingsRedirect /> },
					{ path: "general", element: <AccountSettings /> },
					{ path: "desktop", element: <DesktopSettings /> },
					{ path: "credentials", element: <GlobalVariables /> },
					{ path: ":domain/:backend", element: <LegacySettingsRedirect /> },
				],
			},
			{
				path: "workspaces/:entryId",
				children: [
					{ index: true, element: <WorkspaceRoute /> },
					{ path: "projects/:projectName", element: <WorkspaceRoute /> },
					{ path: "git/worktrees", element: <Worktrees /> },
					{ path: "worktrees/:worktreeId", element: <WorkspaceRoute /> },
					{ path: "worktrees/:worktreeId/projects/:projectName", element: <WorkspaceRoute /> },
					{ path: "worktrees/:worktreeId/git/worktrees", element: <Worktrees /> },
				],
			},
			{ path: "workspace/:entryId", element: <LegacyWorkspaceRedirect /> },
			{ path: "desktop", element: <LegacyGlobalRedirect to="/settings/desktop" /> },
			{ path: "global", element: <LegacyGlobalRedirect to="/settings/credentials" /> },
			{ path: "profile", element: <LegacySettingsRedirect /> },
			{ path: "section/:domain/:backend", element: <LegacySettingsRedirect /> },
			{ path: "*", element: <NotFoundRoute /> },
		],
	},
];

function LegacyRepositoryRedirect() {
	const { repositoryId = "" } = useParams();
	const { registry, workspace, contextEntryId } = useWorkspace(repositoryId);
	if (registry.isLoading) return <WorkspaceLoading />;
	if (!workspace) return <UnknownWorkspace />;
	return <Navigate replace to={`${workspacePath(contextEntryId)}/git/worktrees`} />;
}
