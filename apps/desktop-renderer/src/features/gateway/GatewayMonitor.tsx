import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import useSWR, { useSWRConfig } from "swr";
import { Activity, Plus, RefreshCw, Send, Settings2, SlidersHorizontal } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ErrorNotice, PageHeader, StatePanel } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { ConnectionBadge } from "./ConnectionBadge";
import { GatewayExecutions } from "./GatewayExecutions";
import { useWorkers } from "./executions";
import { GatewayIssueDialog, type GatewayIssueAction } from "./GatewayIssueDialog";
import { GatewayProjectDialog } from "./GatewayProjectDialog";
import { GatewayTaskList } from "./GatewayTaskList";
import {
	getProjectBindings,
	projectBindingsKey,
	repositoryLabel,
	useGatewayJobs,
	useGatewayStatus,
	type GatewaySnapshot,
	type ProjectSummary,
} from "./api";

const displayedCounts = ["queued", "active", "review", "completed", "failed"] as const;

export function GatewayMonitor() {
	const { t, i18n } = useTranslation();
	const status = useGatewayStatus();
	const { mutate } = useSWRConfig();
	const [configure, setConfigure] = useState(false);
	const [configured, setConfigured] = useState(false);
	const [page, setPage] = useState(1);
	const [issueAction, setIssueAction] = useState<GatewayIssueAction>();
	const [search, setSearch] = useSearchParams();
	const selectedExecution = search.get("execution") ?? undefined;
	const connection = status.error ? undefined : status.data;
	const snapshot = connection?.snapshot;
	const mappedProjects = snapshot ? (snapshot.mappings ?? []) : undefined;
	const workers = useWorkers(snapshot?.executions ? connection?.url : undefined);
	const machines = workers.data ?? [];
	const openExecution = (id?: string) => {
		setSearch(
			(current) => {
				const next = new URLSearchParams(current);
				if (id) next.set("execution", id);
				else next.delete("execution");
				return next;
			},
			{ replace: true },
		);
		if (id && connection) void mutate(["/gateway/executions", connection.url]);
	};
	const jobs = useGatewayJobs(connection, page);
	const locale = i18n.resolvedLanguage;
	const formatTime = (value: string) => new Date(value).toLocaleString(locale);

	return (
		<div className="@container mx-auto flex w-full max-w-[1120px] flex-col gap-6">
			<PageHeader
				title={t("gateway.monitor")}
				actions={
					<>
						<Button variant="outline" size="sm" asChild>
							<EnvironmentLink to="/settings/gateway">
								<Settings2 />
								{t("gateway.settings")}
							</EnvironmentLink>
						</Button>
						<Button
							variant="outline"
							size="sm"
							disabled={status.isValidating || jobs.isValidating}
							onClick={() => {
								void status.mutate();
								void jobs.mutate();
							}}
						>
							<RefreshCw />
							{t("gateway.refresh")}
						</Button>
					</>
				}
			/>
			{status.error && <ErrorNotice>{t("gateway.statusError")}</ErrorNotice>}
			{status.isLoading && (
				<div aria-busy="true" className="space-y-4">
					<Skeleton className="h-28 w-full" />
					<Skeleton className="h-20 w-full" />
				</div>
			)}
			{connection && (
				<Card>
					<CardContent className="space-y-4 p-5">
						<div className="flex flex-wrap items-center justify-between gap-2">
							<h2 className="flex items-center gap-2 text-base font-medium">
								<Activity aria-hidden="true" className="size-4 text-muted-foreground" />
								{t("gateway.connection")}
							</h2>
							<ConnectionBadge state={connection.state} />
						</div>
						<dl className="grid gap-4 text-sm @min-[42rem]:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)]">
							{connection.url && (
								<div className="min-w-0 space-y-1">
									<dt className="text-muted-foreground">{t("gateway.address")}</dt>
									<dd className="break-all font-mono text-xs">{connection.url}</dd>
								</div>
							)}
							{snapshot && (
								<div className="min-w-0 space-y-1">
									<dt className="text-muted-foreground">{t("gateway.startedAt")}</dt>
									<dd>{formatTime(snapshot.startedAt)}</dd>
								</div>
							)}
							<div className="min-w-0 space-y-1">
								<dt className="text-muted-foreground">{t("gateway.checkedAt")}</dt>
								<dd>{formatTime(connection.checkedAt)}</dd>
							</div>
						</dl>
					</CardContent>
				</Card>
			)}
			{snapshot ? (
				<>
					<ProjectMappings
						snapshot={snapshot}
						configured={configured}
						onConfigure={() => {
							setConfigured(false);
							setConfigure(true);
						}}
					/>
				</>
			) : connection ? (
				<StatePanel
					icon={Activity}
					title={t("gateway.snapshotUnavailable")}
					description={t(
						connection.state === "unconfigured" || connection.state === "invalid"
							? "gateway.connectHint"
							: "gateway.offlineHint",
					)}
				/>
			) : null}
			{connection && snapshot?.executions && (
				<GatewayExecutions
					key={connection.url}
					connection={connection.url}
					canInvite={!!snapshot.workerUrl}
					selected={selectedExecution}
					onSelect={openExecution}
				/>
			)}

			<section className="space-y-3" aria-labelledby="gateway-queue-heading">
				<div className="flex flex-wrap items-center justify-between gap-3">
					<h2 id="gateway-queue-heading" className="text-base font-medium">
						{t("gateway.tasks")}
					</h2>
					{snapshot && (
						<div className="flex flex-wrap gap-2">
							<Button variant="outline" size="sm" onClick={() => setIssueAction("new")}>
								<Plus />
								{t("gateway.issueActions.new")}
							</Button>
							<Button size="sm" onClick={() => setIssueAction("dispatch")}>
								<Send data-icon="inline-start" />
								{t("gateway.issueActions.dispatch")}
							</Button>
						</div>
					)}
				</div>
				{snapshot && <QueueSummary snapshot={snapshot} />}
			</section>
			{snapshot && (
				<GatewayTaskList
					connection={connection?.url}
					data={jobs.data}
					error={jobs.error}
					isLoading={jobs.isLoading}
					onPageChange={setPage}
				/>
			)}
			{issueAction && (
				<GatewayIssueDialog
					action={issueAction}
					mappedProjects={mappedProjects}
					machines={snapshot?.executions ? machines : undefined}
					onClose={() => setIssueAction(undefined)}
					onDispatched={(id) => {
						if (id) openExecution(id);
						if (page === 1) void jobs.mutate();
						setPage(1);
						void status.mutate();
					}}
				/>
			)}

			{configure && (
				<GatewayProjectDialog
					onClose={() => setConfigure(false)}
					onSaved={() => {
						setConfigured(true);
						void status.mutate();
					}}
				/>
			)}
		</div>
	);
}

function ProjectMappings({
	snapshot,
	configured,
	onConfigure,
}: {
	snapshot: GatewaySnapshot;
	configured: boolean;
	onConfigure(): void;
}) {
	const { t } = useTranslation();
	return (
		<section className="space-y-3" aria-labelledby="gateway-projects-heading">
			<div className="flex flex-wrap items-center justify-between gap-3">
				<div className="space-y-1">
					<h2 id="gateway-projects-heading" className="text-base font-medium">
						{t("gateway.projectMappings")}
					</h2>
					<p className="text-sm text-muted-foreground">
						{t("gateway.projects", { count: snapshot.projects })}
					</p>
				</div>
				<Button variant="outline" size="sm" onClick={onConfigure}>
					<SlidersHorizontal />
					{t("gateway.configureProject")}
				</Button>
			</div>
			{configured && (
				<p role="status" className="text-sm text-success-foreground">
					{t("gateway.projectSaved")}
				</p>
			)}
			{snapshot.mappings?.length ? (
				<MappingTable mappings={snapshot.mappings} />
			) : snapshot.projects > 0 && !snapshot.mappings ? (
				<p className="text-sm text-muted-foreground">{t("gateway.mappingListUnavailable")}</p>
			) : null}
		</section>
	);
}

function MappingTable({ mappings }: { mappings: ProjectSummary[] }) {
	const { t } = useTranslation();
	const bindings = useSWR(projectBindingsKey, getProjectBindings);
	const names = new Map(
		bindings.data?.map(({ binding }) => [binding.project.id, binding.project.name]),
	);
	return (
		<div className="overflow-hidden rounded-md border border-border bg-card">
			<Table aria-labelledby="gateway-projects-heading" className="table-fixed">
				<TableHeader>
					<TableRow>
						<TableHead scope="col" className="w-[36%] whitespace-normal">
							{t("gateway.linearProject")}
						</TableHead>
						<TableHead scope="col" className="w-[42%] whitespace-normal">
							{t("gateway.mappedRepository")}
						</TableHead>
						<TableHead scope="col" className="whitespace-normal">
							{t("gateway.baseBranch")}
						</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody>
					{mappings.map((mapping) => (
						<TableRow key={mapping.projectId}>
							<TableCell className="break-all whitespace-normal">
								{names.get(mapping.projectId) || mapping.projectId}
							</TableCell>
							<TableCell className="break-all whitespace-normal">
								{(mapping.repositories ?? [mapping]).map((repo) => (
									<div key={repositoryLabel(repo)}>{repositoryLabel(repo)}</div>
								))}
							</TableCell>
							<TableCell className="break-all whitespace-normal">
								{(mapping.repositories ?? [mapping]).map((repo) => (
									<div key={repositoryLabel(repo)}>{repo.base || t("gateway.defaultBranch")}</div>
								))}
							</TableCell>
						</TableRow>
					))}
				</TableBody>
			</Table>
		</div>
	);
}

function QueueSummary({ snapshot }: { snapshot: GatewaySnapshot }) {
	const { t, i18n } = useTranslation();
	const format = new Intl.NumberFormat(i18n.resolvedLanguage);
	return (
		<dl className="grid grid-cols-2 gap-px overflow-hidden rounded-md border border-border bg-border @min-[42rem]:grid-cols-5">
			{displayedCounts.map((count) => (
				<div
					key={count}
					className="min-w-0 bg-card p-4 last:col-span-2 @min-[42rem]:last:col-span-1"
				>
					<dt className="break-words text-sm text-muted-foreground">
						{t(`gateway.counts.${count}`)}
					</dt>
					<dd className="mt-1 text-2xl font-medium tabular-nums">
						{format.format(snapshot.jobs[count])}
					</dd>
				</div>
			))}
		</dl>
	);
}
