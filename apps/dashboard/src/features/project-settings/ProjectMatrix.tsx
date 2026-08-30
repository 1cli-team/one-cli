import {
	AlertCircle,
	Box,
	Boxes,
	CheckCircle2,
	CloudUpload,
	Code2,
	KeyRound,
	Library,
	Search,
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
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
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

export type ProjectInspectorTab = "overview" | "environment" | "container" | "deploy";

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

function domainIsApplicable(project: OverviewProject, domain: OverviewIssueDomain): boolean {
	if (domain === "env") return true;
	if (project.kind === "package") return false;
	if (domain === "deploy") return (project.compatibleDeployTargets?.length ?? 0) > 0;
	return Boolean(project.domains?.container || issueFor(project, "container"));
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

interface DomainCellProps {
	project: OverviewProject;
	domain: OverviewIssueDomain;
	backend?: string;
	readOnly?: boolean;
	onClick(): void;
}

const DOMAIN_ICON: Record<OverviewIssueDomain, React.ComponentType<{ className?: string }>> = {
	env: KeyRound,
	container: Box,
	deploy: CloudUpload,
};

const DomainCell: React.FC<DomainCellProps> = ({ project, domain, backend, readOnly, onClick }) => {
	const { t } = useTranslation();
	const issue = issueFor(project, domain);
	const applicable = domainIsApplicable(project, domain);
	const Icon = DOMAIN_ICON[domain];

	if (!applicable) {
		return (
			<span className="text-xs text-muted-foreground/55">{t("projects.matrix.notApplicable")}</span>
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
				"group h-auto w-full justify-start gap-2 rounded-md px-2 py-1.5 text-left transition-colors disabled:cursor-default",
				issue ? "hover:bg-error-surface/65" : "hover:bg-accent/70",
				readOnly && "hover:bg-transparent",
			)}
		>
			<span
				className={cn(
					"grid h-7 w-7 shrink-0 place-items-center rounded-md border",
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
				<span className="block truncate text-[10px] text-muted-foreground">
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
	const filtered = useMemo(() => {
		const normalized = query.trim().toLowerCase();
		return projects.filter(
			(project) =>
				(kind === "all" || project.kind === kind) &&
				(!normalized || projectSearchText(project).includes(normalized)),
		);
	}, [kind, projects, query]);

	return (
		<Card
			role="region"
			aria-labelledby="project-matrix-title"
			className="overflow-hidden rounded-xl"
		>
			<div className="flex h-16 items-center justify-between gap-4 border-b border-border px-5">
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
				<div className="flex items-center gap-2">
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
						<NativeSelect
							size="sm"
							value={kind}
							onChange={(event) => setKind(event.target.value as typeof kind)}
							aria-label={t("projects.kindFilter")}
							className="text-xs"
						>
							<NativeSelectOption value="all">{t("projects.kinds.all")}</NativeSelectOption>
							<NativeSelectOption value="app">{t("overview.kinds.app")}</NativeSelectOption>
							<NativeSelectOption value="service">{t("overview.kinds.service")}</NativeSelectOption>
							<NativeSelectOption value="package">{t("overview.kinds.package")}</NativeSelectOption>
						</NativeSelect>
					</div>
				</div>
			</div>

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
					<TableHeader className="bg-muted/35">
						<TableRow className="hover:bg-transparent">
							<TableHead className="w-[30%] pl-5">{t("projects.matrix.project")}</TableHead>
							<TableHead className="w-[18%]">{t("projects.matrix.environment")}</TableHead>
							<TableHead className="w-[18%]">{t("projects.matrix.container")}</TableHead>
							<TableHead className="w-[18%]">{t("projects.matrix.deploy")}</TableHead>
							<TableHead className="w-[16%] pr-5 text-right">
								{t("projects.matrix.status")}
							</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{filtered.map((project) => {
							const Icon = KIND_ICON[project.kind];
							const issueCount = project.issues?.length ?? 0;
							return (
								<TableRow key={project.name} className="h-[76px]">
									<TableCell className="py-2 pl-5">
										<Button
											type="button"
											variant="ghost"
											onClick={() => onInspect(project, "overview")}
											disabled={readOnly}
											className="group h-auto w-full justify-start gap-3 rounded-md p-0 text-left hover:bg-transparent disabled:cursor-default"
										>
											<span className="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-primary/15 bg-primary/8 text-primary transition-colors group-hover:bg-primary/14">
												<Icon className="h-4 w-4" />
											</span>
											<span className="min-w-0">
												<span className="block truncate text-sm font-semibold group-hover:text-primary">
													{project.name}
												</span>
												<span className="mt-0.5 block truncate font-mono text-[10px] text-muted-foreground">
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
											domain="container"
											backend={project.domains?.container}
											readOnly={readOnly}
											onClick={() => onInspect(project, "container")}
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
											<Badge variant="destructive" className="gap-1.5">
												<AlertCircle className="h-3.5 w-3.5" />
												{t("projects.matrix.issueCount", { count: issueCount })}
											</Badge>
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
