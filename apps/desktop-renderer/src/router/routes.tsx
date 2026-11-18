import { DesktopSettings } from "@/lib/desktop/DesktopSettings";
import { AlertTriangle, ArrowLeft, FolderX, RefreshCw } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import {
	Navigate,
	type RouteObject,
	useLocation,
	useMatch,
	useParams,
	useNavigate,
} from "react-router-dom";
import { Button } from "@/components/ui/button";
import { PageContent, PageHeader, StatePanel } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { OpenVikingSettings } from "@/features/openviking/OpenVikingSettings";
import { KnowledgePage } from "@/features/openviking/KnowledgePage";
import { Overview } from "@/pages/Overview";
import { App } from "@/App";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { useWorkspace } from "@/features/workspace-navigation/useWorkspace";
import { WorkspaceOverview } from "@/features/workspace-navigation/WorkspaceOverview";
import {
	legacyWorkspaceLocation,
	workspaceContext,
	workspacePath,
	workspaceVariablesPath,
} from "./paths";
import { AccountSettings } from "@/features/infisical-session/AccountSettings";
import { GlobalVariables } from "@/features/global-variables/GlobalVariables";
import { CodeHostSettings } from "@/features/code-hosts/CodeHostSettings";
import { GatewaySettings } from "@/features/gateway/GatewaySettings";
import { GatewayMonitor } from "@/features/gateway/GatewayMonitor";
import { LinearSettings } from "@/features/linear/LinearSettings";
import { IssuesPage } from "@/features/linear/IssuesPage";

import { AddWorkspaceDialog } from "@/features/workspace-navigation/AddWorkspaceDialog";
import { useState } from "react";
import { Worktrees } from "@/features/git/Worktrees";
import { preserveEnvironment } from "@/features/environment-context/environment";
import { WorkspaceGroupsPage } from "@/features/workspace-groups/WorkspaceGroupsPage";
import { WorkspaceHome } from "@/pages/WorkspaceHome";
import type { WorkspaceRegistryEntry } from "@/types/api";

const NotFoundRoute: React.FC = () => {
	const { t } = useTranslation();
	const workspaceRoute = useMatch("/workspaces/:entryId/*");
	return (
		<PageContent scroll={Boolean(workspaceRoute)}>
			<StatePanel
				framed
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
		</PageContent>
	);
};

const LegacySettingsRedirect: React.FC = () => {
	const { search } = useLocation();
	return <Navigate replace to={{ pathname: "/settings/general", search }} />;
};

const LegacyWorkspaceRedirect: React.FC = () => {
	const { entryId = "", worktreeId, projectName } = useParams();
	const { search } = useLocation();
	const context = workspaceContext(entryId, worktreeId);
	const location = legacyWorkspaceLocation(context, search);
	return (
		<Navigate
			replace
			to={
				projectName
					? { ...location, pathname: workspaceVariablesPath(context, projectName) }
					: location
			}
		/>
	);
};

const LegacyGlobalRedirect: React.FC<{ to: string }> = ({ to }) => {
	const { search } = useLocation();
	return <Navigate replace to={{ pathname: to, search }} />;
};

const WorkspaceRoute: React.FC<{ view?: "overview" | "variables" | "knowledge" }> = ({
	view = "overview",
}) => {
	const { entryId = "", projectName } = useParams();
	const { search } = useLocation();
	const { registry, workspace, overview, contextEntryId, available } = useWorkspace(entryId);

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
	if (view === "overview" && (available || workspace.status === "uninitialized")) {
		const params = new URLSearchParams(search);
		if (available && (params.has("project") || params.has("tab")))
			return <LegacyWorkspaceRedirect />;
		return <WorkspaceOverview key={contextEntryId} workspace={workspace} />;
	}
	if (workspace.status === "uninitialized") {
		return <Navigate replace to={preserveEnvironment(workspacePath(contextEntryId), search)} />;
	}
	if (!available)
		return (
			<WorkspaceStateLayout>
				<WorkspaceStatusPage workspace={workspace} />
			</WorkspaceStateLayout>
		);
	if (view === "knowledge") {
		if (workspace.status === "identity-conflict") {
			return (
				<WorkspaceStateLayout>
					<WorkspaceStatusPage workspace={workspace} />
				</WorkspaceStateLayout>
			);
		}
		return <KnowledgePage key={contextEntryId} entryId={contextEntryId} />;
	}
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
	const project = projectName ? projects.find((item) => item.name === projectName) : projects[0];
	if (projectName && !project)
		return <Navigate replace to={{ pathname: workspaceVariablesPath(contextEntryId), search }} />;

	return (
		<Overview
			key={contextEntryId}
			project={project}
			projects={projects}
			workspaceEntryId={contextEntryId}
			readOnly={workspace.status === "identity-conflict"}
		/>
	);
};

const WorkspaceStateLayout: React.FC<React.PropsWithChildren> = ({ children }) => (
	<PageContent scroll>{children}</PageContent>
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
			framed
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
		<>
			<PageHeader
				title={workspace.name}
				description={<code className="font-mono text-xs wrap-anywhere">{workspace.root}</code>}
			/>
			<StatePanel
				framed
				icon={FolderX}
				title={t(`workspaces.state.${workspace.status}.title`)}
				description={t(`workspaces.state.${workspace.status}.description`)}
			>
				<p className="w-full text-xs text-muted-foreground">{t("workspaces.forget.pageHint")}</p>
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
		</>
	);
};

const WorkspaceLoadError: React.FC<{ workspace: WorkspaceRegistryEntry; onRetry(): void }> = ({
	workspace,
	onRetry,
}) => {
	const { t } = useTranslation();
	return (
		<StatePanel
			framed
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
			framed
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
			{ path: "projects/:groupId/issues", element: <IssuesPage /> },
			{ path: "workspaces", element: <WorkspaceHome /> },
			{ path: "gateway", element: <GatewayMonitor /> },
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
					{ path: "linear", element: <LinearSettings /> },
					{ path: "gateway", element: <GatewaySettings /> },
					{ path: "openviking", element: <OpenVikingSettings /> },
					{ path: "code-hosts", element: <CodeHostSettings /> },
					{ path: ":domain/:backend", element: <LegacySettingsRedirect /> },
				],
			},
			{
				path: "workspaces/:entryId",
				children: [
					{ index: true, element: <WorkspaceRoute /> },
					{ path: "projects/:projectName", element: <LegacyWorkspaceRedirect /> },
					{
						path: "environment-variables/:projectName?",
						element: <WorkspaceRoute view="variables" />,
					},
					{ path: "git/worktrees", element: <Worktrees /> },
					{ path: "knowledge", element: <WorkspaceRoute view="knowledge" /> },
					{ path: "issues", element: <IssuesPage /> },
					{ path: "worktrees/:worktreeId", element: <WorkspaceRoute /> },
					{
						path: "worktrees/:worktreeId/projects/:projectName",
						element: <LegacyWorkspaceRedirect />,
					},
					{
						path: "worktrees/:worktreeId/environment-variables/:projectName?",
						element: <WorkspaceRoute view="variables" />,
					},
					{ path: "worktrees/:worktreeId/git/worktrees", element: <Worktrees /> },
					{ path: "worktrees/:worktreeId/knowledge", element: <WorkspaceRoute view="knowledge" /> },
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
	if (registry.isLoading || !workspace)
		return (
			<PageContent>{registry.isLoading ? <WorkspaceLoading /> : <UnknownWorkspace />}</PageContent>
		);
	return <Navigate replace to={`${workspacePath(contextEntryId)}/git/worktrees`} />;
}
