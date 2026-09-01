import { AlertCircle, ArrowRight, CheckCircle2, FolderCog, ListChecks } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import type { ProjectInspectorTab } from "@/features/project-settings/ProjectMatrix";
import type { OverviewIssue, OverviewProject } from "@/types/api";

interface WorkspaceActionCenterProps {
	projects: OverviewProject[];
	workspaceIssues: OverviewIssue[];
	readOnly?: boolean;
	issueMessage(issue: OverviewIssue): string;
	onInspect(project: OverviewProject, tab: ProjectInspectorTab): void;
}

function issueKey(issue: OverviewIssue): string {
	return [issue.domain, issue.reason ?? "backend", issue.backend ?? "", issue.profile ?? ""].join(
		":",
	);
}

function issueTab(issue: OverviewIssue): ProjectInspectorTab {
	if (issue.domain === "env") return "environment";
	return issue.domain;
}

function profileIssueSettingsPath(issue: OverviewIssue): string {
	const section = issue.section ?? (issue.backend ? `${issue.domain}/${issue.backend}` : "");
	const [domain, backend, extra] = section.split("/");
	if (!domain || !backend || extra) return "/settings";
	return `/settings/${encodeURIComponent(domain)}/${encodeURIComponent(backend)}`;
}

export const WorkspaceActionCenter: React.FC<WorkspaceActionCenterProps> = ({
	projects,
	workspaceIssues,
	readOnly,
	issueMessage,
	onInspect,
}) => {
	const { t } = useTranslation();
	function jumpToSection(event: React.MouseEvent<HTMLAnchorElement>, href: string) {
		event.preventDefault();
		document.querySelector(href)?.scrollIntoView({ behavior: "smooth", block: "start" });
		window.history.replaceState(null, "", href);
	}
	const projectActions = projects.flatMap((project) =>
		(project.issues ?? []).map((issue) => ({ project, issue })),
	);
	const total = workspaceIssues.length + projectActions.length;

	return (
		<Card
			role="region"
			aria-labelledby="workspace-action-center-title"
			className="overflow-hidden rounded-xl border-slate-300/80 shadow-sm dark:border-slate-700"
		>
			<div className="grid grid-cols-[minmax(260px,0.72fr)_minmax(0,1.6fr)]">
				<div className="relative border-r border-border bg-slate-950 px-6 py-6 text-white dark:bg-slate-950">
					<div className="absolute inset-y-0 left-0 w-1 bg-primary" aria-hidden="true" />
					<div className="flex items-center gap-2 text-orange-300">
						<ListChecks className="h-4 w-4" />
						<span className="text-xs font-semibold uppercase tracking-[0.14em]">
							{t("overview.actionCenter.eyebrow")}
						</span>
					</div>
					<div className="mt-7 flex items-end gap-3">
						<span className="text-5xl font-semibold leading-none tabular-nums">{total}</span>
						<span className="pb-1 text-sm text-slate-300">
							{total > 0 ? t("overview.actionCenter.pending") : t("overview.actionCenter.clear")}
						</span>
					</div>
					<p className="mt-4 max-w-xs text-sm leading-6 text-slate-300">
						{total > 0
							? t("overview.actionCenter.description")
							: t("overview.actionCenter.clearDescription")}
					</p>
				</div>

				<div className="min-w-0 bg-card">
					{total === 0 ? (
						<div className="flex min-h-56 items-center justify-center px-8 py-10 text-center">
							<div>
								<div className="mx-auto grid h-11 w-11 place-items-center rounded-full border border-success-border bg-success-surface text-success-foreground">
									<CheckCircle2 className="h-5 w-5" />
								</div>
								<h2 id="workspace-action-center-title" className="mt-3 text-base font-semibold">
									{t("overview.actionCenter.allGood")}
								</h2>
								<p className="mt-1 text-sm text-muted-foreground">
									{t("overview.actionCenter.allGoodDescription")}
								</p>
							</div>
						</div>
					) : (
						<>
							<div className="flex items-center justify-between border-b border-border px-5 py-3.5">
								<div>
									<h2 id="workspace-action-center-title" className="text-sm font-semibold">
										{t("overview.actionCenter.title")}
									</h2>
									<p className="mt-0.5 text-xs text-muted-foreground">
										{t("overview.actionCenter.hint")}
									</p>
								</div>
								<a
									href="#workspace-projects"
									onClick={(event) => jumpToSection(event, "#workspace-projects")}
									className="text-xs font-medium text-primary hover:underline"
								>
									{t("overview.actionCenter.viewProjects")}
								</a>
							</div>
							<div className="divide-y divide-border">
								{workspaceIssues.map((issue) => (
									<div
										key={`workspace:${issueKey(issue)}`}
										className="flex min-h-16 items-center gap-3 px-5 py-3"
									>
										<span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-warning-border bg-warning-surface text-warning-foreground">
											<FolderCog className="h-4 w-4" />
										</span>
										<div className="min-w-0 flex-1">
											<p className="text-xs font-semibold text-muted-foreground">
												{t("overview.actionCenter.workspaceLabel")}
											</p>
											<p className="mt-0.5 text-sm leading-5">{issueMessage(issue)}</p>
										</div>
										<Button asChild size="sm" variant="outline">
											{issue.reason === "profile" ? (
												<EnvironmentLink to={profileIssueSettingsPath(issue)}>
													{t("overview.fix.profileCta")}
													<ArrowRight />
												</EnvironmentLink>
											) : (
												<a
													href="#workspace-settings"
													onClick={(event) => jumpToSection(event, "#workspace-settings")}
												>
													{t("overview.actionCenter.configureWorkspace")}
													<ArrowRight />
												</a>
											)}
										</Button>
									</div>
								))}

								{projectActions.map(({ project, issue }) => (
									<div
										key={`${project.name}:${issueKey(issue)}`}
										className="flex min-h-16 items-center gap-3 px-5 py-3"
									>
										<span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-error-border bg-error-surface text-error-foreground">
											<AlertCircle className="h-4 w-4" />
										</span>
										<div className="min-w-0 flex-1">
											<p className="truncate text-xs font-semibold text-muted-foreground">
												{project.name}
												<span className="ml-2 font-mono font-normal">{project.relativeDir}</span>
											</p>
											<p className="mt-0.5 text-sm leading-5">{issueMessage(issue)}</p>
										</div>
										<Button
											type="button"
											size="sm"
											variant="outline"
											disabled={readOnly}
											onClick={() => onInspect(project, issueTab(issue))}
										>
											{t("overview.actionCenter.resolve")}
											<ArrowRight />
										</Button>
									</div>
								))}
							</div>
						</>
					)}
				</div>
			</div>
		</Card>
	);
};
