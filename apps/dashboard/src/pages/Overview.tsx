import { AlertTriangle, CheckCircle2, Copy, FolderKanban, Layers3 } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router-dom";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { environmentFromSearch } from "@/features/environment-context/environment";
import {
	ProjectInspector,
	type ProjectInspectorTarget,
} from "@/features/project-settings/ProjectInspector";
import { ProjectMatrix } from "@/features/project-settings/ProjectMatrix";
import { WorkspaceEnvironmentSettings } from "@/features/workspace-settings/WorkspaceEnvironmentSettings";
import type { Overview as OverviewPayload, OverviewIssue } from "@/types/api";

function issueKey(issue: OverviewIssue): string {
	return [issue.domain, issue.reason ?? "backend", issue.backend ?? "", issue.profile ?? ""].join(
		":",
	);
}

function profileIssueSettingsPath(issue: OverviewIssue): string {
	const section = issue.section ?? (issue.backend ? `${issue.domain}/${issue.backend}` : "");
	const [domain, backend, extra] = section.split("/");
	if (!domain || !backend || extra) return "/settings";
	return `/settings/${encodeURIComponent(domain)}/${encodeURIComponent(backend)}`;
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
	const [inspector, setInspector] = useState<ProjectInspectorTarget | null>(null);
	const healthyProjects = projects.filter((project) => (project.issues?.length ?? 0) === 0).length;
	const projectIssues = projects.reduce(
		(count, project) => count + (project.issues?.length ?? 0),
		0,
	);
	const attentionIssues = projectIssues + (data.issues?.length ?? 0);
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
		<div className="space-y-6">
			<Card className="relative overflow-hidden rounded-xl">
				<div className="pointer-events-none absolute -right-16 -top-24 h-64 w-64 rounded-full bg-primary/8 blur-3xl" />
				<CardContent className="relative flex items-start justify-between gap-8 px-6 py-5">
					<div className="min-w-0">
						<div className="flex items-center gap-2">
							<div className="grid h-9 w-9 place-items-center rounded-lg bg-primary text-primary-foreground shadow-sm">
								<FolderKanban className="h-4 w-4" />
							</div>
							<div>
								<div className="flex items-center gap-2">
									<h1 className="text-xl font-semibold tracking-tight">
										{workspace?.name || t("overview.untitledWorkspace")}
									</h1>
									{workspace?.id ? (
										<Badge variant="outline" className="font-mono">
											{workspace.id}
										</Badge>
									) : null}
								</div>
								<p className="mt-0.5 max-w-2xl truncate font-mono text-[11px] text-muted-foreground">
									{data.root}
								</p>
							</div>
						</div>
					</div>

					<div className="grid shrink-0 grid-cols-3 divide-x divide-border overflow-hidden rounded-lg border border-border bg-background/70">
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
					<Copy className="h-4 w-4" />
					<AlertTitle>{t("workspaces.conflict.title")}</AlertTitle>
					<AlertDescription>{t("workspaces.conflict.description")}</AlertDescription>
				</Alert>
			) : null}

			<WorkspaceEnvironmentSettings
				key={`${workspaceEntryId ?? "current"}:${environment}:${workspace?.domains?.env ?? ""}`}
				currentBackend={workspace?.domains?.env}
				environment={environment}
				workspaceEntryId={workspaceEntryId}
				readOnly={readOnly}
			/>

			{(data.issues?.length ?? 0) > 0 ? (
				<Alert variant="destructive">
					<AlertTriangle className="h-4 w-4" />
					<AlertTitle>{t("overview.workspaceIssuesTitle")}</AlertTitle>
					<AlertDescription>
						<div className="mt-1 flex flex-wrap gap-x-5 gap-y-2">
							{data.issues?.map((issue) => (
								<div key={issueKey(issue)} className="flex items-center gap-2">
									<span>{issueMessage(issue)}</span>
									{issue.reason === "profile" ? (
										<EnvironmentLink
											to={profileIssueSettingsPath(issue)}
											className="rounded border border-current/30 px-2 py-0.5 text-[11px] font-medium hover:bg-current/10"
										>
											{t("overview.fix.profileCta")}
										</EnvironmentLink>
									) : (
										<span className="text-[11px] font-medium">{t("overview.fix.cliHint")}</span>
									)}
								</div>
							))}
						</div>
					</AlertDescription>
				</Alert>
			) : null}

			<ProjectMatrix
				projects={projects}
				workspaceEnvironment={workspace?.domains?.env}
				readOnly={readOnly}
				onInspect={(project, tab) => setInspector({ project, tab })}
			/>

			<Card className="border-dashed shadow-none">
				<CardContent className="flex items-center justify-between px-4 py-3 text-xs text-muted-foreground">
					<span className="inline-flex items-center gap-2">
						<Layers3 className="h-3.5 w-3.5" />
						{t("overview.profileSafety")}
					</span>
					<EnvironmentLink to="/settings" className="font-medium text-primary hover:underline">
						{t("overview.manageProfilesCta")}
					</EnvironmentLink>
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

const WorkspaceMetric: React.FC<{
	label: string;
	value: number;
	positive?: boolean;
	warning?: boolean;
	primary?: boolean;
}> = ({ label, value, positive, warning, primary }) => (
	<div
		aria-label={`${label}: ${value}`}
		className={
			primary
				? "min-w-32 bg-primary/[0.04] px-5 py-3.5 text-left"
				: "min-w-24 px-4 py-3 text-center"
		}
	>
		<div className={`flex items-center gap-1.5 ${primary ? "justify-start" : "justify-center"}`}>
			{positive ? <CheckCircle2 className="h-3.5 w-3.5 text-success-foreground" /> : null}
			<span
				className={
					primary
						? "text-3xl font-semibold leading-none tabular-nums text-primary"
						: warning
							? "text-lg font-semibold text-warning-foreground"
							: "text-lg font-semibold"
				}
			>
				{value}
			</span>
		</div>
		<p
			className={
				primary
					? "mt-1.5 whitespace-nowrap text-[10px] font-semibold uppercase tracking-[0.16em] text-muted-foreground"
					: "mt-0.5 whitespace-nowrap text-[10px] uppercase tracking-wider text-muted-foreground"
			}
		>
			{label}
		</p>
	</div>
);
