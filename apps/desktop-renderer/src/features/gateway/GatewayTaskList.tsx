import { lazy, Suspense, useState } from "react";
import { ChevronDown, ExternalLink } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { openExternal } from "@/lib/desktop/runtime";
import i18n from "@/lib/i18n";
import Toast from "@/lib/toast";
import type { HttpError } from "@/types/api";
import type { GatewayJob, GatewayJobPage, JobRepository } from "./api";

const MarkdownPreview = lazy(() =>
	import("@/features/linear/IssueMarkdownEditor").then(({ IssueMarkdownPreview }) => ({
		default: IssueMarkdownPreview,
	})),
);

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
	onPageChange,
}: {
	data?: GatewayJobPage;
	error?: HttpError;
	isLoading: boolean;
	onPageChange(page: number): void;
}) {
	const { t } = useTranslation();
	return (
		<section className="space-y-3" aria-labelledby="gateway-task-list-heading">
			<h2 id="gateway-task-list-heading" className="text-base font-medium">
				{t("gateway.taskList.heading")}
			</h2>
			{error && (
				<ErrorNotice>
					{t(error.status === 404 ? "gateway.taskList.updateRequired" : "gateway.taskList.error")}
				</ErrorNotice>
			)}
			{isLoading && <Skeleton aria-busy="true" className="h-20 w-full" />}
			{data && (
				<>
					{data.items.length > 0 ? (
						<ul className="divide-y divide-border overflow-hidden rounded-md border border-border bg-card">
							{data.items.map((job) => (
								<GatewayTaskRow key={job.id} job={job} />
							))}
						</ul>
					) : (
						<p className="rounded-md border border-border bg-card p-4 text-sm text-muted-foreground">
							{t("gateway.taskList.empty")}
						</p>
					)}
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
		</section>
	);
}

function GatewayTaskRow({ job }: { job: GatewayJob }) {
	const { t, i18n } = useTranslation();
	const [expanded, setExpanded] = useState(false);
	const created = new Date(job.createdAt).toLocaleString(i18n.resolvedLanguage);
	const variant = stageColors[job.stage as keyof typeof stageColors] ?? "secondary";
	return (
		<li>
			<details className="group" onToggle={(event) => setExpanded(event.currentTarget.open)}>
				<summary className="flex cursor-pointer list-none flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3 text-sm hover:bg-muted/50 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
					<span className="min-w-0 flex-1 basis-[12rem] space-y-1">
						<span className="block font-medium wrap-anywhere">
							<span className="mr-2 font-mono text-xs text-muted-foreground">{job.identifier}</span>
							{job.title}
						</span>
						<span className="flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
							{job.projectName && <span className="wrap-anywhere">{job.projectName}</span>}
							<time dateTime={job.createdAt}>{created}</time>
						</span>
					</span>
					<Badge variant={variant}>
						{t(`gateway.taskList.stages.${job.stage}`, { defaultValue: job.stage })}
					</Badge>
					<ChevronDown
						aria-hidden="true"
						className="size-4 shrink-0 text-muted-foreground group-open:rotate-180"
					/>
				</summary>
				<div className="space-y-3 border-t border-border px-4 py-3 text-sm">
					{job.lastError && (
						<ErrorNotice>
							<p className="font-medium">{t("gateway.taskList.lastError")}</p>
							<p className="mt-1 whitespace-pre-wrap wrap-anywhere">{job.lastError}</p>
						</ErrorNotice>
					)}
					{job.codex && (
						<dl className="grid gap-2 rounded-md border border-border p-3 sm:grid-cols-2">
							<div className="min-w-0">
								<dt className="text-xs text-muted-foreground">{t("gateway.codex.model")}</dt>
								<dd className="wrap-anywhere">{job.codex.model || t("gateway.codex.default")}</dd>
							</div>
							<div className="min-w-0">
								<dt className="text-xs text-muted-foreground">{t("gateway.codex.effort")}</dt>
								<dd className="wrap-anywhere">{job.codex.effort || t("gateway.codex.default")}</dd>
							</div>
						</dl>
					)}
					{expanded && job.description && (
						<Suspense
							fallback={<p className="whitespace-pre-wrap wrap-anywhere">{job.description}</p>}
						>
							<MarkdownPreview value={job.description} onOpenLink={openTaskLink} />
						</Suspense>
					)}
					{job.repositories.length > 0 && (
						<ul className="divide-y divide-border rounded-md border border-border">
							{job.repositories.map((repo) => (
								<RepositoryResult
									key={`${repo.repository.host}/${repo.repository.slug}`}
									repo={repo}
								/>
							))}
						</ul>
					)}
					{job.url && <TaskLink url={job.url}>{t("gateway.taskList.openLinear")}</TaskLink>}
				</div>
			</details>
		</li>
	);
}

function RepositoryResult({ repo }: { repo: JobRepository }) {
	const { t } = useTranslation();
	const pr = repo.pr;
	const prLabel = pr && t("gateway.taskList.pr", { number: pr.number });
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
			{pr && prLabel ? (
				<div className="flex items-center gap-2">
					{pr.url ? <TaskLink url={pr.url}>{prLabel}</TaskLink> : <span>{prLabel}</span>}
					{pr.merged ? (
						<Badge variant="success">{t("gateway.taskList.merged")}</Badge>
					) : pr.state === "closed" ? (
						<Badge variant="muted">{t("gateway.taskList.closed")}</Badge>
					) : null}
				</div>
			) : repo.unchanged ? (
				<Badge variant="muted">{t("gateway.taskList.unchanged")}</Badge>
			) : null}
		</li>
	);
}

function TaskLink({ url, children }: { url: string; children: string }) {
	return (
		<Button
			variant="link"
			size="xs"
			className="h-auto px-0 has-[>svg]:px-0"
			onClick={() => openTaskLink(url)}
		>
			{children}
			<ExternalLink aria-hidden="true" />
		</Button>
	);
}

function openTaskLink(url: string) {
	void openExternal(url).catch(() => Toast.error(i18n.t("gateway.taskList.linkError")));
}
