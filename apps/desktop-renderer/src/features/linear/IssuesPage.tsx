import { useEffect, useState, type ReactNode } from "react";
import { useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { ArrowLeft, ClipboardList, FolderPlus, Plus, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
	ErrorNotice,
	PageContent,
	PageHeader,
	PageToolbar,
	SearchInput,
	StatePanel,
} from "@/components/ui/page-layout";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { useWorkspaceGroups } from "@/features/workspace-groups/useWorkspaceGroups";
import { BindingForm } from "./BindingForm";
import { Choice } from "./Choice";
import { IssueDialog } from "./IssueDialog";
import { CreateIssueWorktreeDialog } from "./CreateIssueWorktreeDialog";
import { workflowStateLabel } from "./display-labels";
import {
	errorText,
	fetchJSON,
	gatewayJobsKey,
	projectKey,
	teamKey,
	scoped,
	unbind,
	type Binding,
	type GatewayJobs,
	type Issue,
	type IssuePage,
	type Operation,
	type Project,
	type Scope,
	type Team,
} from "./api";

export function IssuesPage() {
	const { groupId, entryId } = useParams();
	const scope: Scope = groupId
		? { kind: "group", id: groupId }
		: { kind: "workspace", id: entryId || "" };
	return <ProjectIssues key={`${scope.kind}:${scope.id}`} scope={scope} />;
}

function ProjectIssues({ scope }: { scope: Scope }) {
	const { t } = useTranslation();
	const projects = useWorkspaceGroups();
	const binding = useSWR<{ binding: Binding | null }>(scoped(scope, "binding"), fetchJSON);
	const savedBinding = binding.data?.binding;
	const project = useSWR<Project>(
		savedBinding ? projectKey(savedBinding.connectionId, savedBinding.project.id) : null,
		fetchJSON,
	);
	const currentBinding = savedBinding
		? { ...savedBinding, project: project.data || savedBinding.project }
		: null;
	const entryId = scope.id.split("~")[0];
	const name =
		scope.kind === "group"
			? projects.data?.groups.find((group) => group.id === scope.id)?.name
			: projects.data?.workspaces.find(
					(workspace) =>
						workspace.entryId === entryId ||
						workspace.aliases?.some((alias) => alias.entryId === entryId),
				)?.name;
	const backAction = (
		<Button asChild variant="ghost" className="min-w-0 max-w-full">
			<EnvironmentLink
				to={
					scope.kind === "group"
						? `/projects/${encodeURIComponent(scope.id)}`
						: `/workspaces/${encodeURIComponent(scope.id)}`
				}
			>
				<ArrowLeft />
				<span className="truncate">{name || t("linear.back")}</span>
			</EnvironmentLink>
		</Button>
	);
	const notices = (
		<>
			{binding.error && (
				<ErrorNotice
					action={
						<Button variant="outline" onClick={() => void binding.mutate()}>
							{t("workspaces.retry")}
						</Button>
					}
				>
					{errorText(binding.error)}
				</ErrorNotice>
			)}
			{project.error && <ErrorNotice>{errorText(project.error)}</ErrorNotice>}
		</>
	);
	return (
		<PageContent scroll={scope.kind === "workspace"}>
			{currentBinding ? (
				<IssueList
					key={`${currentBinding.connectionId}:${currentBinding.project.id}`}
					scope={scope}
					binding={currentBinding}
					backAction={backAction}
					notices={notices}
					onRefreshProject={() => project.mutate()}
					onUnbound={() => void binding.mutate()}
				/>
			) : (
				<>
					<PageHeader title={t("linear.issues")} actions={backAction} />
					{notices}
					{!binding.data && !binding.error && <p role="status">{t("linear.loading")}</p>}
					{binding.data && (
						<BindingForm
							scope={scope}
							projectName={name || ""}
							onSaved={(saved) => void binding.mutate({ binding: saved }, { revalidate: false })}
						/>
					)}
				</>
			)}
		</PageContent>
	);
}

function IssueList({
	scope,
	binding,
	backAction,
	notices,
	onUnbound,
	onRefreshProject,
}: {
	scope: Scope;
	binding: Binding;
	backAction: ReactNode;
	notices: ReactNode;
	onUnbound(): void;
	onRefreshProject(): Promise<unknown>;
}) {
	const { t } = useTranslation();
	const [query, setQuery] = useState("");
	const [filter, setFilter] = useState({ q: "", team: "", state: "", assignee: "", priority: "" });
	const [cursors, setCursors] = useState<string[]>([""]);
	const [selected, setSelected] = useState<string | null>(null);
	const [worktreeIssue, setWorktreeIssue] = useState<Issue | null>(null);
	const [error, setError] = useState("");
	const [busy, setBusy] = useState(false);
	const [refreshing, setRefreshing] = useState(false);
	const operations = useSWR<Operation[]>(scoped(scope, "operations"), fetchJSON);
	const jobs = useSWR<GatewayJobs>(gatewayJobsKey(scope), fetchJSON, {
		refreshInterval: (data) => (data?.enabled ? 5000 : 0),
	});
	const team = useSWR<Team>(
		filter.team ? teamKey(binding.connectionId, filter.team) : null,
		fetchJSON,
	);
	const list = useSWR<IssuePage>(
		scoped(scope, "issues", { ...filter, after: cursors.at(-1)! }),
		fetchJSON,
		{ revalidateOnFocus: true },
	);
	async function refresh() {
		setRefreshing(true);
		setError("");
		try {
			await Promise.all([
				onRefreshProject(),
				list.mutate(),
				team.mutate(),
				operations.mutate(),
				jobs.mutate(),
			]);
		} catch (error) {
			setError(errorText(error));
		} finally {
			setRefreshing(false);
		}
	}
	useEffect(() => {
		if (filter.team && !binding.project.teams.some((team) => team.id === filter.team)) {
			setFilter((current) => ({ ...current, team: "", state: "", assignee: "" }));
			setCursors([""]);
		}
	}, [binding.project.teams, filter.team]);
	function change(key: keyof typeof filter, value: string) {
		setFilter((current) => ({
			...current,
			[key]: value,
			...(key === "team" ? { state: "", assignee: "" } : {}),
		}));
		setCursors([""]);
	}
	return (
		<>
			<PageHeader
				title={t("linear.issues")}
				description={binding.project.name}
				actions={
					<>
						{backAction}
						<Button
							variant="ghost"
							disabled={busy}
							onClick={() => {
								setBusy(true);
								setError("");
								void unbind(scope)
									.then(onUnbound)
									.catch((e) => setError(errorText(e)))
									.finally(() => setBusy(false));
							}}
						>
							{t("linear.unbind")}
						</Button>
						<Button
							variant="outline"
							onClick={() => void refresh()}
							disabled={refreshing || list.isValidating}
						>
							<RefreshCw />
							{t("linear.refresh")}
						</Button>
						<Button onClick={() => setSelected("new")}>
							<Plus />
							{t("linear.create")}
						</Button>
					</>
				}
			/>
			{notices}
			{operations.data?.map((operation) => (
				<div
					key={operation.id}
					className="flex flex-wrap items-center gap-3 rounded-md border border-border bg-muted p-3 text-sm"
				>
					<span className="min-w-0 flex-1">
						{t("linear.pendingOperation")}{" "}
						<span className="break-all font-mono text-xs">{operation.id}</span>
					</span>
					<Button
						variant="outline"
						disabled={busy}
						onClick={() => {
							setBusy(true);
							setError("");
							void fetchJSON<Operation>(scoped(scope, `operations/${operation.id}`))
								.then(async (result) => {
									if (result.status === "confirmed") {
										await operations.mutate();
										await list.mutate();
									} else setError(t("linear.stillUnknown"));
								})
								.catch((e) => setError(errorText(e)))
								.finally(() => setBusy(false));
						}}
					>
						{t("linear.reconcile")}
					</Button>
				</div>
			))}
			<PageToolbar className="grid items-end @min-[56rem]:grid-cols-[minmax(15rem,1fr)_minmax(0,2.6fr)]">
				<form
					className="flex min-w-0 gap-2"
					onSubmit={(e) => {
						e.preventDefault();
						change("q", query);
					}}
				>
					<SearchInput
						groupClassName="h-9"
						aria-label={t("linear.search")}
						placeholder={t("linear.search")}
						value={query}
						onChange={(e) => setQuery(e.target.value)}
					/>
					<Button type="submit" variant="outline" className="h-9">
						{t("linear.searchAction")}
					</Button>
				</form>
				<div className="grid min-w-0 grid-cols-2 gap-3 @min-[36rem]:grid-cols-4">
					<Choice
						label={t("linear.team")}
						value={filter.team}
						onChange={(v) => change("team", v)}
						options={binding.project.teams}
						placeholder={t("linear.allTeams")}
					/>
					<Choice
						label={t("linear.state")}
						value={filter.state}
						onChange={(v) => change("state", v)}
						options={
							team.data?.states.map(({ id, name }) => ({
								id,
								name: workflowStateLabel(name, t),
							})) || []
						}
						disabled={!filter.team || !team.data}
						placeholder={t("linear.allStates")}
					/>
					<Choice
						label={t("linear.assignee")}
						value={filter.assignee}
						onChange={(v) => change("assignee", v)}
						options={team.data?.members || []}
						disabled={!filter.team || !team.data}
						placeholder={t("linear.allAssignees")}
					/>
					<Choice
						label={t("linear.priority")}
						value={filter.priority}
						onChange={(v) => change("priority", v)}
						options={[0, 1, 2, 3, 4].map((n) => ({
							id: String(n),
							name: t(`linear.priorities.${n}`),
						}))}
						placeholder={t("linear.allPriorities")}
					/>
				</div>
				<div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground @min-[56rem]:col-span-2">
					<span className="font-medium">
						{t("linear.boundTeam", {
							name:
								binding.project.teams.find((team) => team.id === binding.teamId)?.name ||
								t("linear.chooseTeam"),
						})}
					</span>
					<span>{t("linear.projectTeamsHint")}</span>
				</div>
			</PageToolbar>
			{(error || team.error || list.error || operations.error) && (
				<ErrorNotice>
					{error || errorText(team.error || list.error || operations.error)}
				</ErrorNotice>
			)}
			{list.data?.stale && (
				<p className="rounded-md bg-muted p-3 text-sm" role="status">
					{t("linear.stale", { time: new Date(list.data.fetchedAt).toLocaleString() })}
				</p>
			)}
			{!list.data && !list.error && <p role="status">{t("linear.loading")}</p>}
			{list.data && (
				<div
					className="overflow-hidden rounded-lg border border-border bg-card"
					aria-busy={list.isValidating}
				>
					{!list.data.nodes.length && <StatePanel icon={ClipboardList} title={t("linear.empty")} />}
					{list.data.nodes.map((issue) => (
						<div
							key={issue.id}
							className="flex items-center border-b border-border last:border-0 hover:bg-muted/50"
						>
							<Button
								variant="ghost"
								type="button"
								className="h-auto min-w-0 flex-1 justify-start gap-3 whitespace-normal rounded-none px-4 py-2.5 text-left font-normal hover:bg-transparent focus-visible:ring-inset focus-visible:ring-offset-0"
								onClick={() => setSelected(issue.id)}
							>
								<span className="min-w-12 shrink-0 font-mono text-xs text-muted-foreground">
									{issue.identifier}
								</span>
								<span className="min-w-0 flex-1">
									<span className="block truncate text-sm font-medium">{issue.title}</span>
									<span className="text-xs text-muted-foreground">
										{workflowStateLabel(issue.state.name, t)} · {issue.team.name}
										{jobs.data?.issues[issue.id] === "processing" && (
											<span> · {t("gateway.processing")}</span>
										)}
									</span>
								</span>
								<span className="hidden text-xs text-muted-foreground ud-md:block">
									{t(`linear.priorities.${issue.priority}`)}
								</span>
								<span className="hidden w-24 truncate text-right text-xs text-muted-foreground ud-md:block">
									{issue.assignee?.name || t("linear.unassigned")}
								</span>
							</Button>
							<Button
								type="button"
								variant="ghost"
								size="icon"
								className="mr-3 shrink-0"
								title={t("git.create")}
								aria-label={t("linear.createWorktreeFor", { identifier: issue.identifier })}
								onClick={() => setWorktreeIssue(issue)}
							>
								<FolderPlus />
							</Button>
						</div>
					))}
				</div>
			)}
			<div className="flex items-center justify-end gap-2">
				{cursors.length > 1 && (
					<Button
						variant="outline"
						disabled={list.isValidating}
						onClick={() => setCursors((c) => c.slice(0, -1))}
					>
						{t("linear.previous")}
					</Button>
				)}
				{list.data?.pageInfo.hasNextPage && (
					<Button
						variant="outline"
						disabled={list.isValidating}
						onClick={() => setCursors((c) => [...c, list.data!.pageInfo.endCursor])}
					>
						{t("linear.next")}
					</Button>
				)}
			</div>
			{worktreeIssue && (
				<CreateIssueWorktreeDialog
					scope={scope}
					issue={worktreeIssue}
					onClose={() => setWorktreeIssue(null)}
				/>
			)}
			{selected && (
				<IssueDialog
					key={selected}
					scope={scope}
					binding={binding}
					issueId={selected}
					onClose={() => {
						setSelected(null);
						void operations.mutate();
					}}
					onSaved={() => {
						void list.mutate();
						void operations.mutate();
					}}
				/>
			)}
		</>
	);
}
