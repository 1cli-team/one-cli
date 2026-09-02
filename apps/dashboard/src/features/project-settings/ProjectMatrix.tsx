import {
	AlertCircle,
	Boxes,
	CheckCircle2,
	CloudUpload,
	Code2,
	KeyRound,
	Library,
	Search,
	SlidersHorizontal,
} from "lucide-react";
import type React from "react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";
import type {
	OverviewIssue,
	OverviewIssueDomain,
	OverviewProject,
	OverviewProjectKind,
} from "@/types/api";

export type ProjectInspectorTab = "overview" | "environment" | "deploy";

type MatrixDomain = Exclude<OverviewIssueDomain, "container">;

interface ProjectMatrixProps {
	projects: OverviewProject[];
	workspaceEnvironment?: string;
	readOnly?: boolean;
	onInspect(project: OverviewProject, tab: ProjectInspectorTab): void;
}

const KIND_ICON: Record<OverviewProjectKind, React.ComponentType<{ className?: string }>> = {
	app: Code2,
	service: Boxes,
	package: Library,
};

function issueFor(
	project: OverviewProject,
	domain: OverviewIssueDomain,
): OverviewIssue | undefined {
	return project.issues?.find((issue) => issue.domain === domain);
}

function domainIsApplicable(project: OverviewProject, domain: MatrixDomain): boolean {
	if (domain === "env") return true;
	if (project.kind === "package") return false;
	return (project.compatibleDeployTargets?.length ?? 0) > 0;
}

function projectSearchText(project: OverviewProject): string {
	return [
		project.name,
		project.relativeDir,
		project.kind,
		project.templateId,
		project.toolchain,
		...Object.values(project.domains ?? {}),
	]
		.filter(Boolean)
		.join(" ")
		.toLowerCase();
}

function inspectorTabForIssue(issue: OverviewIssue): ProjectInspectorTab {
	if (issue.domain === "env") return "environment";
	return "deploy";
}

interface DomainCellProps {
	project: OverviewProject;
	domain: MatrixDomain;
	backend?: string;
	readOnly?: boolean;
	onClick(): void;
}

const DOMAIN_ICON: Record<MatrixDomain, React.ComponentType<{ className?: string }>> = {
	env: KeyRound,
	deploy: CloudUpload,
};

const DomainCell: React.FC<DomainCellProps> = ({ project, domain, backend, readOnly, onClick }) => {
	const { t } = useTranslation();
	const issue = issueFor(project, domain);
	const applicable = domainIsApplicable(project, domain);
	const Icon = DOMAIN_ICON[domain];

	if (!applicable) {
		return (
			<span
				className="pl-3 text-sm text-muted-foreground/55"
				title={t("projects.matrix.notApplicableTitle")}
			>
				{t("projects.matrix.notApplicable")}
			</span>
		);
	}

	return (
		<Button
			type="button"
			variant="ghost"
			onClick={onClick}
			disabled={readOnly}
			aria-label={t("projects.matrix.configureDomain", {
				project: project.name,
				domain,
			})}
			className={cn(
				"group h-auto w-full justify-start gap-2 rounded-none px-2 py-1.5 text-left transition-colors disabled:cursor-default",
				issue ? "hover:bg-error-surface/65" : "hover:bg-accent/70",
				readOnly && "hover:bg-transparent",
			)}
		>
			<span
				className={cn(
					"grid h-7 w-7 shrink-0 place-items-center border",
					issue
						? "border-error-border/60 bg-error-surface text-error-foreground"
						: "border-border bg-background text-muted-foreground group-hover:text-primary",
				)}
			>
				<Icon className="h-3.5 w-3.5" />
			</span>
			<span className="min-w-0">
				<span
					className={cn("block truncate text-xs font-medium", issue && "text-error-foreground")}
				>
					{backend || t("projects.matrix.notConfigured")}
				</span>
				<span className="block truncate text-[11px] text-muted-foreground">
					{issue?.reason === "profile"
						? t("projects.matrix.profileMissing")
						: issue
							? t("projects.matrix.setupRequired")
							: t("projects.matrix.ready")}
				</span>
			</span>
		</Button>
	);
};

export const ProjectMatrix: React.FC<ProjectMatrixProps> = ({
	projects,
	workspaceEnvironment,
	readOnly,
	onInspect,
}) => {
	const { t } = useTranslation();
	const [query, setQuery] = useState("");
	const [kind, setKind] = useState<"all" | OverviewProjectKind>("all");
	const [status, setStatus] = useState<"all" | "attention" | "healthy">("all");
	const [filtersOpen, setFiltersOpen] = useState(false);
	const filtered = useMemo(() => {
		const normalized = query.trim().toLowerCase();
		return projects
			.filter((project) => {
				const hasIssues = (project.issues?.length ?? 0) > 0;
				return (
					(kind === "all" || project.kind === kind) &&
					(status === "all" || (status === "attention" ? hasIssues : !hasIssues)) &&
					(!normalized || projectSearchText(project).includes(normalized))
				);
			})
			.sort(
				(left, right) =>
					Number((right.issues?.length ?? 0) > 0) - Number((left.issues?.length ?? 0) > 0),
			);
	}, [kind, projects, query, status]);

	return (
		<Card role="region" aria-labelledby="project-matrix-title" className="overflow-hidden">
			<div className="flex min-h-16 items-center justify-between gap-4 border-b border-border px-5 py-3">
				<div>
					<div className="flex items-center gap-2">
						<h2 id="project-matrix-title" className="text-sm font-semibold">
							{t("projects.title")}
						</h2>
						<Badge variant="secondary" className="font-mono">
							{projects.length}
						</Badge>
					</div>
					<p className="mt-0.5 text-xs text-muted-foreground">{t("projects.description")}</p>
				</div>
				<Button
					type="button"
					variant="ghost"
					size="sm"
					onClick={() => setFiltersOpen((open) => !open)}
					aria-expanded={filtersOpen}
				>
					<SlidersHorizontal className="size-3.5" />
					{filtersOpen ? t("projects.hideFilters") : t("projects.showFilters")}
				</Button>
			</div>

			{filtersOpen ? (
				<div className="flex items-center justify-end gap-2 border-b border-border bg-muted/20 px-5 py-2.5">
					<div className="w-36 shrink-0">
						<Select value={status} onValueChange={(value) => setStatus(value as typeof status)}>
							<SelectTrigger size="sm" aria-label={t("projects.statusFilter")} className="text-xs">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="all">{t("projects.statuses.all")}</SelectItem>
								<SelectItem value="attention">{t("projects.statuses.attention")}</SelectItem>
								<SelectItem value="healthy">{t("projects.statuses.healthy")}</SelectItem>
							</SelectContent>
						</Select>
					</div>
					<InputGroup className="h-8 w-56">
						<InputGroupAddon>
							<Search className="h-3.5 w-3.5" />
						</InputGroupAddon>
						<InputGroupInput
							value={query}
							onChange={(event) => setQuery(event.target.value)}
							placeholder={t("projects.search")}
							aria-label={t("projects.search")}
							className="text-xs"
						/>
					</InputGroup>
					<div className="w-32 shrink-0">
						<Select value={kind} onValueChange={(value) => setKind(value as typeof kind)}>
							<SelectTrigger size="sm" aria-label={t("projects.kindFilter")} className="text-xs">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="all">{t("projects.kinds.all")}</SelectItem>
								<SelectItem value="app">{t("overview.kinds.app")}</SelectItem>
								<SelectItem value="service">{t("overview.kinds.service")}</SelectItem>
								<SelectItem value="package">{t("overview.kinds.package")}</SelectItem>
							</SelectContent>
						</Select>
					</div>
				</div>
			) : null}

			{filtered.length === 0 ? (
				<Empty className="min-h-52 rounded-none border-0">
					<EmptyHeader>
						<EmptyMedia variant="icon">
							<Search />
						</EmptyMedia>
						<EmptyTitle>{t("projects.empty.title")}</EmptyTitle>
						<EmptyDescription>{t("projects.empty.description")}</EmptyDescription>
					</EmptyHeader>
				</Empty>
			) : (
				<Table className="table-fixed">
					<TableHeader className="bg-muted/55">
						<TableRow className="hover:bg-transparent">
							<TableHead className="w-[34%] pl-5">{t("projects.matrix.project")}</TableHead>
							<TableHead className="w-[22%]">{t("projects.matrix.environment")}</TableHead>
							<TableHead className="w-[24%]">{t("projects.matrix.deploy")}</TableHead>
							<TableHead className="w-[20%] pr-5 text-right">
								{t("projects.matrix.status")}
							</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{filtered.map((project) => {
							const Icon = KIND_ICON[project.kind];
							const issueCount = project.issues?.length ?? 0;
							return (
								<TableRow key={project.name} className="h-[68px]">
									<TableCell className="py-2 pl-5">
										<Button
											type="button"
											variant="ghost"
											onClick={() => onInspect(project, "overview")}
											disabled={readOnly}
											className="group h-auto w-full justify-start gap-3 rounded-none p-0 text-left hover:bg-transparent disabled:cursor-default"
										>
											<span className="grid h-9 w-9 shrink-0 place-items-center border border-primary/15 bg-primary/8 text-primary transition-colors group-hover:bg-primary/14">
												<Icon className="h-4 w-4" />
											</span>
											<span className="min-w-0">
												<span className="block truncate text-sm font-semibold group-hover:text-primary">
													{project.name}
												</span>
												<span className="mt-0.5 block truncate font-mono text-[11px] text-muted-foreground">
													{project.relativeDir}
												</span>
											</span>
										</Button>
									</TableCell>
									<TableCell className="p-2">
										<DomainCell
											project={project}
											domain="env"
											backend={project.domains?.env ?? workspaceEnvironment}
											readOnly={readOnly}
											onClick={() => onInspect(project, "environment")}
										/>
									</TableCell>
									<TableCell className="p-2">
										<DomainCell
											project={project}
											domain="deploy"
											backend={project.domains?.deploy}
											readOnly={readOnly}
											onClick={() => onInspect(project, "deploy")}
										/>
									</TableCell>
									<TableCell className="py-2 pr-5 text-right">
										{issueCount > 0 ? (
											<Button
												type="button"
												variant="outline"
												size="sm"
												disabled={readOnly}
												className="border-error-border bg-error-surface text-error-foreground hover:bg-error-surface/70 hover:text-error-foreground"
												onClick={() => {
													const issue = project.issues?.[0];
													if (issue) onInspect(project, inspectorTabForIssue(issue));
												}}
											>
												<AlertCircle className="h-3.5 w-3.5" />
												{t("projects.matrix.issueCount", { count: issueCount })}
											</Button>
										) : (
											<Badge
												variant="outline"
												className="gap-1.5 border-transparent text-success-foreground"
											>
												<CheckCircle2 className="h-3.5 w-3.5" />
												{t("projects.matrix.healthy")}
											</Badge>
										)}
									</TableCell>
								</TableRow>
							);
						})}
					</TableBody>
				</Table>
			)}
		</Card>
	);
};
