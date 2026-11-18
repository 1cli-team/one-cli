import { GatewaySessionLink } from "@/features/openviking/GatewaySessionLink";
import { useState } from "react";
import { ChevronDown, ClipboardList, ExternalLink } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ErrorNotice, PageToolbar, StatePanel } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { Choice } from "@/features/linear/Choice";
import type { HttpError } from "@/types/api";
import {
	useGatewayJobCodexResult,
	type GatewayTask,
	type GatewayTaskFilters,
	type GatewayTaskPage,
	type JobRepository,
} from "./api";
import { CodexResultMessage, GatewayMarkdown, openGatewayLink } from "./GatewayMarkdown";
import { GatewaySubtasks } from "./GatewaySubtasks";
import { GatewayQuestions } from "./GatewayQuestions";
import { GatewayProgress } from "./GatewayProgress";
import type { ExecutionWorker } from "./executions";

const taskStages = [
	"queued",
	"running",
	"preparing_subtask",
	"committing_subtask",
	"waiting_for_answer",
	"waiting_to_continue",
	"revising",
	"verifying",
	"publishing",
	"opening",
	"to_review",
	"review",
	"to_done",
	"done",
	"succeeded",
	"failed",
	"outcome_unknown",
];
const sources = ["desktop", "cli"] as const;

const stageColors = {
	queued: "muted",
	review: "warning",
	done: "success",
	failed: "error",
} as const;

export function GatewayTaskList({
	data,
	error,
	isLoading,
	connection,
	filters,
	machines,
	onFilterChange,
	onPageChange,
	onOpenRun,
	onRunUpdated,
}: {
	data?: GatewayTaskPage;
	error?: HttpError;
	isLoading: boolean;
	connection?: string;
	filters: GatewayTaskFilters;
	machines: ExecutionWorker[];
	onFilterChange(field: keyof GatewayTaskFilters, value: string): void;
	onPageChange(page: number): void;
	onOpenRun(id: string): void;
	onRunUpdated(): void;
}) {
	const { t } = useTranslation();
	return (
		<div className="space-y-3">
			<PageToolbar className="grid @min-[32rem]:grid-cols-3">
				<Choice
					label={t("gateway.execution.worker")}
					value={filters.worker}
					options={machines.map(({ id, name }) => ({ id, name }))}
					placeholder={t("gateway.execution.all")}
					onChange={(value) => onFilterChange("worker", value)}
				/>
				<Choice
					label={t("gateway.execution.status")}
					value={filters.stage}
					options={taskStages.map((id) => ({ id, name: t(`gateway.taskList.stages.${id}`) }))}
					placeholder={t("gateway.execution.all")}
					onChange={(value) => onFilterChange("stage", value)}
				/>
				<Choice
					label={t("gateway.execution.sourceLabel")}
					value={filters.source}
					options={sources.map((id) => ({ id, name: t(`gateway.execution.source.${id}`) }))}
					placeholder={t("gateway.execution.all")}
					onChange={(value) => onFilterChange("source", value)}
				/>
			</PageToolbar>
			{error && (
				<ErrorNotice>
					{t(error.status === 404 ? "gateway.taskList.updateRequired" : "gateway.taskList.error")}
				</ErrorNotice>
			)}
			{isLoading && <Skeleton aria-busy="true" className="h-20 w-full" />}
			{data?.partial && <ErrorNotice>{t("gateway.taskList.partial")}</ErrorNotice>}
			{data && (
				<>
					{data.items.length > 0 ? (
						<ul className="divide-y divide-border overflow-hidden rounded-lg border border-border bg-card">
							{data.items.map((task) => (
								<GatewayTaskRow
									key={task.id}
									task={task}
									connection={connection}
									onOpenRun={onOpenRun}
									onRunUpdated={onRunUpdated}
								/>
							))}
						</ul>
					) : !data.partial ? (
						<StatePanel
							icon={ClipboardList}
							framed
							title={t(
								filters.worker || filters.stage || filters.source
									? "gateway.taskList.noMatching"
									: "gateway.taskList.empty",
							)}
						/>
					) : null}
					{data.pageCount > 1 && (
						<nav
							aria-label={t("gateway.taskList.pagination")}
							className="flex flex-wrap items-center justify-end gap-2"
						>
							<span className="mr-1 text-sm text-muted-foreground tabular-nums">
								{t("gateway.taskList.page", { page: data.page, total: data.pageCount })}
							</span>
							<Button
								variant="outline"
								size="sm"
								disabled={data.page === 1}
								onClick={() => onPageChange(data.page - 1)}
							>
								{t("gateway.taskList.previous")}
							</Button>
							<Button
								variant="outline"
								size="sm"
								disabled={data.page === data.pageCount}
								onClick={() => onPageChange(data.page + 1)}
							>
								{t("gateway.taskList.next")}
							</Button>
						</nav>
					)}
				</>
			)}
		</div>
	);
}

function GatewayTaskRow({
	task,
	connection,
	onOpenRun,
	onRunUpdated,
}: {
	task: GatewayTask;
	connection?: string;
	onOpenRun(id: string): void;
	onRunUpdated(): void;
}) {
	const { t, i18n } = useTranslation();
	const [expanded, setExpanded] = useState(false);
	const showResult = expanded && !!task.localResultJobId;
	const result = useGatewayJobCodexResult(
		connection,
		showResult ? task.localResultJobId : undefined,
	);
	const updated = new Date(task.updatedAt).toLocaleString(i18n.resolvedLanguage);
	const variant = stageColors[task.stage as keyof typeof stageColors] ?? "secondary";
	const fallbackPRs = task.pullRequests?.filter(
		(pr) =>
			!task.repositories?.some((repo) => repo.pr?.url === pr.url && repo.pr?.number === pr.number),
	);
	return (
		<li>
			<details className="group" onToggle={(event) => setExpanded(event.currentTarget.open)}>
				<summary className="flex cursor-pointer list-none flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3 text-sm transition-colors duration-150 ease-ui hover:bg-muted/50 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring group-open:bg-muted/45 [&::-webkit-details-marker]:hidden">
					<span className="min-w-0 flex-1 basis-[12rem] space-y-1">
						<span className="block font-medium wrap-anywhere">
							{task.identifier && (
								<span className="mr-2 font-mono text-xs text-muted-foreground">
									{task.identifier}
								</span>
							)}
							{task.title || task.repository?.slug || t("gateway.dispatch")}
						</span>
						<span className="flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
							{task.projectName && <span className="wrap-anywhere">{task.projectName}</span>}
							{task.workerName && <span className="wrap-anywhere">{task.workerName}</span>}
							<time dateTime={task.updatedAt}>{updated}</time>
						</span>
					</span>
					<Badge variant={variant}>
						{t(`gateway.taskList.stages.${task.stage}`, { defaultValue: task.stage })}
					</Badge>
					<ChevronDown
						aria-hidden="true"
						className="size-4 shrink-0 text-muted-foreground motion-safe:transition-transform motion-safe:duration-150 motion-safe:ease-ui group-open:rotate-180"
					/>
				</summary>
				<div className="space-y-3 border-t border-border px-4 py-3 text-sm">
					{task.lastError && (
						<ErrorNotice>
							<p className="font-medium">{t("gateway.taskList.lastError")}</p>
							<p className="mt-1 whitespace-pre-wrap wrap-anywhere">{task.lastError}</p>
						</ErrorNotice>
					)}
					{task.codex && (
						<dl className="grid gap-2 rounded-md border border-border p-3 sm:grid-cols-2">
							<div className="min-w-0">
								<dt className="text-xs text-muted-foreground">{t("gateway.codex.model")}</dt>
								<dd className="wrap-anywhere">{task.codex.model || t("gateway.codex.default")}</dd>
							</div>
							<div className="min-w-0">
								<dt className="text-xs text-muted-foreground">{t("gateway.codex.effort")}</dt>
								<dd className="wrap-anywhere">{task.codex.effort || t("gateway.codex.default")}</dd>
							</div>
						</dl>
					)}
					{expanded && task.description && <GatewayMarkdown value={task.description} />}
					{showResult && (
						<CodexResultMessage isLoading={result.isLoading} value={result.data?.response} />
					)}
					{expanded && task.openViking && <GatewaySessionLink session={task.openViking} />}
					{expanded && <GatewaySubtasks subtasks={task.subtasks} />}
					{!!task.repositories?.length && (
						<ul className="divide-y divide-border rounded-md border border-border">
							{task.repositories.map((repo) => (
								<RepositoryResult
									key={`${repo.repository.host}/${repo.repository.slug}`}
									repo={repo}
								/>
							))}
						</ul>
					)}
					{!!fallbackPRs?.length && (
						<ul className="divide-y divide-border rounded-md border border-border">
							{fallbackPRs.map((pr) => (
								<li key={pr.url || pr.number} className="p-3">
									<PullRequestResult pr={pr} />
								</li>
							))}
						</ul>
					)}
					{expanded && !!task.runs?.length && (
						<section className="space-y-2">
							<h3 className="font-medium">{t("gateway.taskList.runs")}</h3>
							<ul className="divide-y divide-border rounded-md border border-border">
								{task.runs.map((run) => (
									<li key={run.id} className="space-y-2 p-3">
										<div className="flex flex-wrap items-center gap-x-3 gap-y-1">
											<span className="font-medium">
												{t(
													run.revision
														? "gateway.execution.revision"
														: "gateway.taskList.initialRun",
												)}
											</span>
											<span className="text-xs text-muted-foreground">{run.workerName}</span>
											<span className="text-xs text-muted-foreground">
												{t(`gateway.execution.source.${run.source}`)}
											</span>
											<time className="text-xs text-muted-foreground" dateTime={run.createdAt}>
												{new Date(run.createdAt).toLocaleString(i18n.resolvedLanguage)}
											</time>
											<Badge variant="outline">{t(`gateway.execution.state.${run.state}`)}</Badge>
											<Button
												size="xs"
												variant="link"
												className="ml-auto"
												onClick={() => onOpenRun(run.id)}
											>
												{t("gateway.execution.details")}
											</Button>
										</div>
										{run.question && (
											<GatewayQuestions
												job={{ ...run, questions: [run.question] }}
												onAnswered={onRunUpdated}
											/>
										)}
										{run.progress && (
											<GatewayProgress
												key={run.progress.id}
												jobId={run.id}
												progress={run.progress}
												state={run.state}
												onContinued={onRunUpdated}
											/>
										)}
									</li>
								))}
							</ul>
						</section>
					)}
					{task.url && <TaskLink url={task.url}>{t("gateway.taskList.openLinear")}</TaskLink>}
				</div>
			</details>
		</li>
	);
}

function RepositoryResult({ repo }: { repo: JobRepository }) {
	const { t } = useTranslation();
	const pr = repo.pr;
	return (
		<li className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2 p-3">
			<div className="min-w-0 space-y-1">
				<p className="font-medium wrap-anywhere">
					{repo.repository.host}/{repo.repository.slug}
				</p>
				{repo.branch && (
					<p className="font-mono text-xs text-muted-foreground wrap-anywhere">
						{repo.base
							? t("gateway.taskList.branchToBase", { branch: repo.branch, base: repo.base })
							: repo.branch}
					</p>
				)}
			</div>
			{pr ? (
				<PullRequestResult pr={pr} />
			) : repo.unchanged ? (
				<Badge variant="muted">{t("gateway.taskList.unchanged")}</Badge>
			) : null}
		</li>
	);
}

function PullRequestResult({ pr }: { pr: NonNullable<JobRepository["pr"]> }) {
	const { t } = useTranslation();
	const label = t("gateway.taskList.pr", { number: pr.number });
	return (
		<div className="flex items-center gap-2">
			{pr.url ? <TaskLink url={pr.url}>{label}</TaskLink> : <span>{label}</span>}
			{pr.merged ? (
				<Badge variant="success">{t("gateway.taskList.merged")}</Badge>
			) : pr.state === "closed" ? (
				<Badge variant="muted">{t("gateway.taskList.closed")}</Badge>
			) : null}
		</div>
	);
}

function TaskLink({ url, children }: { url: string; children: string }) {
	return (
		<Button
			variant="link"
			size="xs"
			className="h-auto px-0 has-[>svg]:px-0"
			onClick={() => openGatewayLink(url)}
		>
			{children}
			<ExternalLink aria-hidden="true" />
		</Button>
	);
}
