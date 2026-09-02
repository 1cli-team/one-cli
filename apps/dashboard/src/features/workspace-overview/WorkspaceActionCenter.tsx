import { AlertCircle, ArrowRight, CheckCircle2, FolderCog } from "lucide-react";
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
	return "deploy";
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
		<Card role="region" aria-labelledby="workspace-action-center-title" className="overflow-hidden">
			<div className="flex min-h-16 items-center justify-between border-b border-border px-4 py-3">
				<div>
					<h2 id="workspace-action-center-title" className="text-sm font-semibold">
						{total > 0 ? t("overview.actionCenter.title") : t("overview.actionCenter.allGood")}
					</h2>
					<p className="mt-0.5 text-[10px] text-muted-foreground">
						{total > 0
							? t("overview.actionCenter.hint")
							: t("overview.actionCenter.allGoodDescription")}
					</p>
				</div>
				<span className="grid size-9 place-items-center bg-warning-surface font-mono text-sm font-semibold text-warning-foreground">
					{String(total).padStart(2, "0")}
				</span>
			</div>

			{total === 0 ? (
				<div className="flex min-h-52 items-center justify-center px-6 py-8 text-center">
					<div>
						<div className="mx-auto grid h-10 w-10 place-items-center border border-success-border bg-success-surface text-success-foreground">
							<CheckCircle2 className="h-5 w-5" />
						</div>
						<p className="mt-3 text-xs leading-5 text-muted-foreground">
							{t("overview.actionCenter.clearDescription")}
						</p>
					</div>
				</div>
			) : (
				<>
					<div className="divide-y divide-border">
						{workspaceIssues.map((issue) => (
							<div
								key={`workspace:${issueKey(issue)}`}
								className="flex min-h-16 items-center gap-2.5 px-3 py-2.5"
							>
								<span className="grid h-7 w-7 shrink-0 place-items-center border border-warning-border bg-warning-surface text-warning-foreground">
									<FolderCog className="h-4 w-4" />
								</span>
								<div className="min-w-0 flex-1">
									<p className="text-[10px] font-semibold text-muted-foreground">
										{t("overview.actionCenter.workspaceLabel")}
									</p>
									<p className="mt-0.5 line-clamp-2 text-xs leading-4">{issueMessage(issue)}</p>
								</div>
								<Button asChild size="icon-sm" variant="ghost" className="text-primary">
									{issue.reason === "profile" ? (
										<EnvironmentLink to={profileIssueSettingsPath(issue)}>
											<span className="sr-only">{t("overview.fix.profileCta")}</span>
											<ArrowRight />
										</EnvironmentLink>
									) : (
										<a
											href="#workspace-settings"
											onClick={(event) => jumpToSection(event, "#workspace-settings")}
										>
											<span className="sr-only">
												{t("overview.actionCenter.configureWorkspace")}
											</span>
											<ArrowRight />
										</a>
									)}
								</Button>
							</div>
						))}

						{projectActions.map(({ project, issue }) => (
							<div
								key={`${project.name}:${issueKey(issue)}`}
								className="flex min-h-16 items-center gap-2.5 px-3 py-2.5"
							>
								<span className="grid h-7 w-7 shrink-0 place-items-center border border-error-border bg-error-surface text-error-foreground">
									<AlertCircle className="h-4 w-4" />
								</span>
								<div className="min-w-0 flex-1">
									<p className="truncate text-[10px] font-semibold text-muted-foreground">
										{project.name}
										<span className="ml-2 font-mono font-normal">{project.relativeDir}</span>
									</p>
									<p className="mt-0.5 line-clamp-2 text-xs leading-4">{issueMessage(issue)}</p>
								</div>
								<Button
									type="button"
									size="icon-sm"
									variant="ghost"
									className="text-primary"
									disabled={readOnly}
									onClick={() => onInspect(project, issueTab(issue))}
								>
									<span className="sr-only">{t("overview.actionCenter.resolve")}</span>
									<ArrowRight />
								</Button>
							</div>
						))}
					</div>
					<a
						href="#workspace-projects"
						onClick={(event) => jumpToSection(event, "#workspace-projects")}
						className="flex h-9 items-center justify-between border-t border-border bg-muted/25 px-3 font-mono text-[9px] uppercase tracking-wide text-primary hover:bg-muted/45"
					>
						{t("overview.actionCenter.viewProjects")}
						<ArrowRight className="size-3.5" />
					</a>
				</>
			)}
		</Card>
	);
};
