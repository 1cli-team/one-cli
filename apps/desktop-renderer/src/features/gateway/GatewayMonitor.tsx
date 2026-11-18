import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import useSWR, { useSWRConfig } from "swr";
import { Activity, Plus, RefreshCw, Send, Settings2, SlidersHorizontal } from "lucide-react";
import { Popover, Tabs } from "radix-ui";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { ErrorNotice, PageContent, PageHeader, StatePanel } from "@/components/ui/page-layout";
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
import { GatewayExecutionDetails, GatewayWorkers } from "./GatewayExecutions";
import { useWorkers } from "./executions";
import { GatewayIssueDialog, type GatewayIssueAction } from "./GatewayIssueDialog";
import { GatewayProjectDialog } from "./GatewayProjectDialog";
import { GatewayTaskList } from "./GatewayTaskList";
import {
	getProjectBindings,
	projectBindingsKey,
	repositoryLabel,
	useGatewayTasks,
	useGatewayStatus,
	type GatewaySnapshot,
	type GatewayStatus,
	type GatewayTaskFilters,
	type ProjectSummary,
} from "./api";

const tabStyle =
	"min-h-9 shrink-0 border-b-2 border-transparent px-3 text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring data-[state=active]:border-primary data-[state=active]:text-primary-text";

export function GatewayMonitor() {
	const { t } = useTranslation();
	const status = useGatewayStatus();
	const { mutate } = useSWRConfig();
	const [configure, setConfigure] = useState(false);
	const [configured, setConfigured] = useState(false);
	const [page, setPage] = useState(1);
	const [filters, setFilters] = useState<GatewayTaskFilters>({ worker: "", stage: "", source: "" });
	const [issueAction, setIssueAction] = useState<GatewayIssueAction>();
	const [search, setSearch] = useSearchParams();
	const selectedExecution = search.get("execution") ?? undefined;
	const connection = status.error ? undefined : status.data;
	const snapshot = connection?.snapshot;
	const requested = search.get("section");
	const section =
		requested === "mappings" || (requested === "workers" && snapshot?.executions)
			? requested
			: "tasks";
	const mappedProjects = snapshot ? (snapshot.mappings ?? []) : undefined;
	const workers = useWorkers(snapshot?.executions ? connection?.url : undefined);
	const machines = workers.data ?? [];
	const tasks = useGatewayTasks(connection, page, filters);
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
		if (id && connection) {
			void mutate(["/gateway/executions", connection.url]);
			void tasks.mutate();
		}
	};

	return (
		<PageContent>
			<PageHeader
				title={t("gateway.monitor")}
				status={connection && <ConnectionBadge state={connection.state} />}
				actions={
					<>
						<GatewayConnectionDetails connection={connection} />
						<Button
							variant="outline"
							disabled={status.isValidating || tasks.isValidating}
							onClick={() => {
								void status.mutate();
								void tasks.mutate();
								void workers.mutate();
							}}
						>
							<RefreshCw />
							{t("gateway.refresh")}
						</Button>
						{snapshot && section === "tasks" && (
							<>
								<Button variant="outline" onClick={() => setIssueAction("new")}>
									<Plus />
									{t("gateway.issueActions.new")}
								</Button>
								<Button onClick={() => setIssueAction("dispatch")}>
									<Send />
									{t("gateway.issueActions.dispatch")}
								</Button>
							</>
						)}
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
			{connection && snapshot ? (
				<Tabs.Root
					value={section}
					onValueChange={(value) => {
						setSearch(
							(current) => {
								const next = new URLSearchParams(current);
								if (value === "tasks") next.delete("section");
								else next.set("section", value);
								return next;
							},
							{ replace: true },
						);
					}}
					className="flex min-w-0 flex-col gap-4"
				>
					<Tabs.List
						aria-label={t("gateway.monitor")}
						className="flex gap-1 overflow-x-auto border-b border-border"
					>
						<Tabs.Trigger value="tasks" className={tabStyle}>
							{t("gateway.tasks")}
						</Tabs.Trigger>
						{snapshot.executions && (
							<Tabs.Trigger value="workers" className={tabStyle}>
								{t("gateway.execution.workers")}
							</Tabs.Trigger>
						)}
						<Tabs.Trigger value="mappings" className={tabStyle}>
							{t("gateway.projectMappings")}
						</Tabs.Trigger>
					</Tabs.List>
					<Tabs.Content value="tasks" forceMount className="min-w-0 data-[state=inactive]:hidden">
						<GatewayTaskList
							connection={connection.url}
							data={tasks.data}
							error={tasks.error}
							isLoading={tasks.isLoading}
							filters={filters}
							machines={machines}
							onFilterChange={(field, value) => {
								setFilters((current) => ({ ...current, [field]: value }));
								setPage(1);
							}}
							onPageChange={setPage}
							onOpenRun={openExecution}
							onRunUpdated={() => void tasks.mutate()}
						/>
					</Tabs.Content>
					{snapshot.executions && (
						<Tabs.Content value="workers" className="min-w-0">
							<GatewayWorkers
								key={connection.url}
								machines={machines}
								isLoading={workers.isLoading}
								hasError={!!workers.error}
								canInvite={!!snapshot.workerUrl}
							/>
						</Tabs.Content>
					)}
					<Tabs.Content value="mappings" className="min-w-0">
						<ProjectMappings
							snapshot={snapshot}
							configured={configured}
							onConfigure={() => {
								setConfigured(false);
								setConfigure(true);
							}}
						/>
					</Tabs.Content>
				</Tabs.Root>
			) : connection ? (
				<StatePanel
					className="rounded-lg border border-border bg-card"
					icon={Activity}
					title={t("gateway.snapshotUnavailable")}
					description={t(
						connection.state === "unconfigured" || connection.state === "invalid"
							? "gateway.connectHint"
							: "gateway.offlineHint",
					)}
				/>
			) : null}
			{connection && snapshot?.executions && selectedExecution && (
				<GatewayExecutionDetails
					key={`${connection.url}:${selectedExecution}`}
					connection={connection.url}
					id={selectedExecution}
					workers={machines}
					onClose={() => openExecution()}
					onRevised={openExecution}
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
						if (page === 1) void tasks.mutate();
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
		</PageContent>
	);
}

function GatewayConnectionDetails({ connection }: { connection?: GatewayStatus }) {
	const { t, i18n } = useTranslation();
	function formatTime(value: string) {
		return new Date(value).toLocaleString(i18n.resolvedLanguage);
	}
	return (
		<Popover.Root>
			<Popover.Trigger asChild>
				<Button variant="outline">
					<Settings2 />
					{t("gateway.connectionDetails")}
				</Button>
			</Popover.Trigger>
			<Popover.Portal>
				<Popover.Content
					align="end"
					sideOffset={8}
					collisionPadding={12}
					aria-label={t("gateway.connectionDetails")}
					className="z-50 w-80 max-w-[var(--radix-popover-content-available-width)] space-y-3 rounded-lg border border-border bg-popover p-4 text-popover-foreground shadow-lg"
				>
					{connection && (
						<dl className="space-y-2 text-sm">
							{connection.url && (
								<div>
									<dt className="text-xs text-muted-foreground">{t("gateway.address")}</dt>
									<dd className="font-mono text-xs wrap-anywhere">{connection.url}</dd>
								</div>
							)}
							{connection.snapshot && (
								<div>
									<dt className="text-xs text-muted-foreground">{t("gateway.startedAt")}</dt>
									<dd>{formatTime(connection.snapshot.startedAt)}</dd>
								</div>
							)}
							<div>
								<dt className="text-xs text-muted-foreground">{t("gateway.checkedAt")}</dt>
								<dd>{formatTime(connection.checkedAt)}</dd>
							</div>
						</dl>
					)}
					<Button variant="outline" size="sm" asChild>
						<EnvironmentLink to="/settings/gateway">
							<Settings2 />
							{t("gateway.settings")}
						</EnvironmentLink>
					</Button>
				</Popover.Content>
			</Popover.Portal>
		</Popover.Root>
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
					<h2 id="gateway-projects-heading" className="sr-only">
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
		<div className="overflow-hidden rounded-lg border border-border bg-card">
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
