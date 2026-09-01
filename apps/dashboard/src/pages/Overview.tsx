import { CheckCircle2, FolderKanban, Gauge, KeyRound, Layers3, Settings2 } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router-dom";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { environmentFromSearch } from "@/features/environment-context/environment";
import {
	ProjectInspector,
	type ProjectInspectorTarget,
} from "@/features/project-settings/ProjectInspector";
import { ProjectMatrix } from "@/features/project-settings/ProjectMatrix";
import { SecretsManager } from "@/features/secrets/SecretsManager";
import { WorkspaceActionCenter } from "@/features/workspace-overview/WorkspaceActionCenter";
import { WorkspaceEnvironmentSettings } from "@/features/workspace-settings/WorkspaceEnvironmentSettings";
import type { Overview as OverviewPayload, OverviewIssue } from "@/types/api";

function jumpToWorkspaceSection(event: React.MouseEvent<HTMLAnchorElement>, href: string) {
	event.preventDefault();
	document.querySelector(href)?.scrollIntoView({ behavior: "smooth", block: "start" });
	window.history.replaceState(null, "", href);
}

export const Overview: React.FC<{
	data: OverviewPayload;
	workspaceEntryId?: string;
	readOnly?: boolean;
}> = ({ data, workspaceEntryId, readOnly }) => {
	const { t } = useTranslation();
	const { search } = useLocation();
	const environment = environmentFromSearch(search);
	const workspace = data.workspace;
	const projects = data.projects ?? [];
	const workspaceIssues = data.issues ?? [];
	const [inspector, setInspector] = useState<ProjectInspectorTarget | null>(null);
	const healthyProjects = projects.filter((project) => (project.issues?.length ?? 0) === 0).length;
	const projectIssues = projects.reduce(
		(count, project) => count + (project.issues?.length ?? 0),
		0,
	);
	const attentionIssues = projectIssues + workspaceIssues.length;
	const issueMessage = (issue: OverviewIssue) =>
		issue.reason === "profile"
			? t("overview.issue.missingProfile", {
					section: issue.section ?? `${issue.domain}/${issue.backend ?? ""}`,
					profile: issue.profile ? ` "${issue.profile}"` : "",
					defaultValue: issue.message,
				})
			: t(`overview.issue.${issue.severity}.${issue.domain}`, {
					defaultValue: issue.message,
				});

	return (
		<div className="space-y-5">
			<Card className="relative overflow-hidden rounded-xl border-slate-300/80 shadow-sm dark:border-slate-700">
				<div className="pointer-events-none absolute -right-16 -top-24 h-64 w-64 rounded-full bg-primary/8 blur-3xl" />
				<CardContent className="relative flex items-start justify-between gap-8 px-6 py-5">
					<div className="min-w-0">
						<div className="flex items-center gap-3">
							<div className="grid h-10 w-10 place-items-center rounded-lg bg-primary text-primary-foreground shadow-sm">
								<FolderKanban className="h-4 w-4" />
							</div>
							<div>
								<div className="flex items-center gap-2">
									<h1 className="text-xl font-semibold tracking-tight">
										{workspace?.name || t("overview.untitledWorkspace")}
									</h1>
									{workspace?.id ? (
										<Badge variant="outline" className="font-mono text-xs">
											{workspace.id}
										</Badge>
									) : null}
								</div>
								<p className="mt-1 max-w-2xl truncate font-mono text-xs text-muted-foreground">
									{data.root}
								</p>
							</div>
						</div>
					</div>

					<div className="grid shrink-0 grid-cols-3 divide-x divide-border overflow-hidden rounded-lg border border-border bg-background/75">
						<WorkspaceMetric
							label={t("overview.metrics.projects")}
							value={projects.length}
							primary
						/>
						<WorkspaceMetric
							label={t("overview.metrics.healthy")}
							value={healthyProjects}
							positive={healthyProjects === projects.length}
						/>
						<WorkspaceMetric
							label={t("overview.metrics.attention")}
							value={attentionIssues}
							warning={attentionIssues > 0}
						/>
					</div>
				</CardContent>
			</Card>

			{readOnly ? (
				<Alert>
					<Layers3 className="h-4 w-4" />
					<AlertTitle>{t("workspaces.conflict.title")}</AlertTitle>
					<AlertDescription>{t("workspaces.conflict.description")}</AlertDescription>
				</Alert>
			) : null}

			<WorkspaceJumpNav
				attentionIssues={attentionIssues}
				projectCount={projects.length}
				secretsEnabled={workspace?.domains?.env === "infisical"}
			/>

			<section id="workspace-overview" className="scroll-mt-20">
				<WorkspaceActionCenter
					projects={projects}
					workspaceIssues={workspaceIssues}
					readOnly={readOnly}
					issueMessage={issueMessage}
					onInspect={(project, tab) => setInspector({ project, tab })}
				/>
			</section>

			<section id="workspace-projects" className="scroll-mt-20">
				<ProjectMatrix
					projects={projects}
					workspaceEnvironment={workspace?.domains?.env}
					readOnly={readOnly}
					onInspect={(project, tab) => setInspector({ project, tab })}
				/>
			</section>

			<section id="workspace-secrets" className="scroll-mt-20">
				{workspace?.domains?.env === "infisical" ? (
					<SecretsManager
						workspaceEntryId={workspaceEntryId}
						environment={environment}
						projects={projects}
						readOnly={readOnly}
					/>
				) : (
					<Card className="overflow-hidden rounded-xl border-dashed shadow-none">
						<CardContent className="flex items-center justify-between gap-5 px-5 py-4">
							<div className="flex items-center gap-3">
								<div className="grid h-9 w-9 place-items-center rounded-lg bg-muted text-muted-foreground">
									<KeyRound className="h-4 w-4" />
								</div>
								<div>
									<h2 className="text-sm font-semibold">{t("secrets.unavailableTitle")}</h2>
									<p className="mt-0.5 text-xs text-muted-foreground">
										{t("secrets.unavailableDescription")}
									</p>
								</div>
							</div>
							<Button asChild variant="outline" size="sm">
								<a
									href="#workspace-settings"
									onClick={(event) => jumpToWorkspaceSection(event, "#workspace-settings")}
								>
									{t("secrets.configureBackend")}
								</a>
							</Button>
						</CardContent>
					</Card>
				)}
			</section>

			<section id="workspace-settings" className="scroll-mt-20">
				<WorkspaceEnvironmentSettings
					key={`${workspaceEntryId ?? "current"}:${environment}:${workspace?.domains?.env ?? ""}`}
					currentBackend={workspace?.domains?.env}
					environment={environment}
					workspaceEntryId={workspaceEntryId}
					readOnly={readOnly}
				/>
			</section>

			<Card className="border-dashed shadow-none">
				<CardContent className="flex items-center justify-between px-4 py-3 text-xs text-muted-foreground">
					<span className="inline-flex items-center gap-2">
						<Layers3 className="h-3.5 w-3.5" />
						{t("overview.profileSafety")}
					</span>
					<Button asChild variant="link" size="sm" className="h-auto p-0">
						<EnvironmentLink to="/settings">{t("overview.manageProfilesCta")}</EnvironmentLink>
					</Button>
				</CardContent>
			</Card>

			<ProjectInspector
				target={inspector}
				environment={environment}
				workspaceEntryId={workspaceEntryId}
				readOnly={readOnly}
				onOpenChange={(open) => {
					if (!open) setInspector(null);
				}}
			/>
		</div>
	);
};

const WorkspaceJumpNav: React.FC<{
	attentionIssues: number;
	projectCount: number;
	secretsEnabled: boolean;
}> = ({ attentionIssues, projectCount, secretsEnabled }) => {
	const { t } = useTranslation();
	const items = [
		{
			href: "#workspace-overview",
			icon: Gauge,
			label: t("overview.navigation.overview"),
			meta:
				attentionIssues > 0
					? t("overview.navigation.attention", { count: attentionIssues })
					: t("overview.navigation.ready"),
			attention: attentionIssues > 0,
		},
		{
			href: "#workspace-projects",
			icon: FolderKanban,
			label: t("overview.navigation.projects"),
			meta: t("overview.navigation.projectCount", { count: projectCount }),
		},
		{
			href: "#workspace-secrets",
			icon: KeyRound,
			label: t("overview.navigation.secrets"),
			meta: secretsEnabled
				? t("overview.navigation.infisical")
				: t("overview.navigation.notEnabled"),
		},
		{
			href: "#workspace-settings",
			icon: Settings2,
			label: t("overview.navigation.settings"),
			meta: t("overview.navigation.backendProfile"),
		},
	];
	return (
		<nav
			aria-label={t("overview.navigation.label")}
			className="sticky top-0 z-20 grid grid-cols-4 overflow-hidden rounded-xl border border-border bg-card/95 shadow-sm backdrop-blur"
		>
			{items.map(({ href, icon: Icon, label, meta, attention }) => (
				<a
					key={href}
					href={href}
					onClick={(event) => jumpToWorkspaceSection(event, href)}
					className="group flex min-w-0 items-center gap-3 border-r border-border px-4 py-3 transition-colors last:border-r-0 hover:bg-accent/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
				>
					<span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-muted text-muted-foreground transition-colors group-hover:bg-primary/10 group-hover:text-primary">
						<Icon className="h-4 w-4" />
					</span>
					<span className="min-w-0">
						<span className="block text-sm font-semibold">{label}</span>
						<span
							className={
								attention
									? "block truncate text-xs text-error-foreground"
									: "block truncate text-xs text-muted-foreground"
							}
						>
							{meta}
						</span>
					</span>
				</a>
			))}
		</nav>
	);
};

const WorkspaceMetric: React.FC<{
	label: string;
	value: number;
	positive?: boolean;
	warning?: boolean;
	primary?: boolean;
}> = ({ label, value, positive, warning, primary }) => (
	<div aria-label={`${label}: ${value}`} className="min-w-24 px-4 py-3 text-center">
		<div className="flex items-center justify-center gap-1.5">
			{positive ? <CheckCircle2 className="h-3.5 w-3.5 text-success-foreground" /> : null}
			<span
				className={
					primary
						? "text-3xl font-semibold leading-none text-primary"
						: warning
							? "text-xl font-semibold text-warning-foreground"
							: "text-xl font-semibold"
				}
			>
				{value}
			</span>
		</div>
		<p className="mt-0.5 whitespace-nowrap text-xs text-muted-foreground">{label}</p>
	</div>
);
