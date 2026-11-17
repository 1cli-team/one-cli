import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { ArrowLeft, ClipboardList, FolderPlus, Plus, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ErrorNotice, PageHeader } from "@/components/ui/page-layout";
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
	projectKey,
	teamKey,
	scoped,
	unbind,
	type Binding,
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
	return (
		<div className="mx-auto flex h-full w-full max-w-[1440px] flex-col gap-5 overflow-y-auto p-1">
			<div>
				<Button asChild variant="ghost" size="sm">
					<EnvironmentLink
						to={
							scope.kind === "group"
								? `/projects/${encodeURIComponent(scope.id)}`
								: `/workspaces/${encodeURIComponent(scope.id)}`
						}
					>
						<ArrowLeft />
						{name || t("linear.back")}
					</EnvironmentLink>
				</Button>
			</div>
			<PageHeader title={t("linear.issues")} description={currentBinding?.project.name} />
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
			{!binding.data && !binding.error && <p role="status">{t("linear.loading")}</p>}
			{binding.data && !binding.data.binding && (
				<BindingForm
					scope={scope}
					projectName={name || ""}
					onSaved={(saved) => void binding.mutate({ binding: saved }, { revalidate: false })}
				/>
			)}
			{currentBinding && (
				<IssueList
					key={`${currentBinding.connectionId}:${currentBinding.project.id}`}
					scope={scope}
					binding={currentBinding}
					onRefreshProject={() => project.mutate()}
					onUnbound={() => void binding.mutate()}
				/>
			)}
		</div>
	);
}

function IssueList({
	scope,
	binding,
	onUnbound,
	onRefreshProject,
}: {
	scope: Scope;
	binding: Binding;
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
			await Promise.all([onRefreshProject(), list.mutate(), team.mutate(), operations.mutate()]);
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
			<div className="flex flex-wrap items-center justify-between gap-3">
				<p className="text-sm text-muted-foreground">
					{t("linear.boundTeam", {
						name:
							binding.project.teams.find((team) => team.id === binding.teamId)?.name ||
							t("linear.chooseTeam"),
					})}
				</p>
				<div className="flex flex-wrap gap-2">
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
				</div>
			</div>
			<p className="text-xs text-muted-foreground">{t("linear.projectTeamsHint")}</p>
			<form
				className="flex gap-2"
				onSubmit={(e) => {
					e.preventDefault();
					change("q", query);
				}}
			>
				<Input
					aria-label={t("linear.search")}
					placeholder={t("linear.search")}
					value={query}
					onChange={(e) => setQuery(e.target.value)}
				/>
				<Button type="submit" variant="outline">
					{t("linear.searchAction")}
				</Button>
			</form>
			<div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
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
						team.data?.states.map(({ id, name }) => ({ id, name: workflowStateLabel(name, t) })) ||
						[]
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
					{!list.data.nodes.length && (
						<div className="flex flex-col items-center gap-3 p-12 text-muted-foreground">
							<ClipboardList className="size-7" />
							<p>{t("linear.empty")}</p>
						</div>
					)}
					{list.data.nodes.map((issue) => (
						<div
							key={issue.id}
							className="flex items-center border-b border-border last:border-0 hover:bg-muted/50"
						>
							<Button
								variant="ghost"
								type="button"
								className="h-auto min-w-0 flex-1 justify-start gap-3 whitespace-normal rounded-none px-4 py-3 text-left font-normal hover:bg-transparent"
								onClick={() => setSelected(issue.id)}
							>
								<span className="w-20 shrink-0 font-mono text-xs text-muted-foreground">
									{issue.identifier}
								</span>
								<span className="min-w-0 flex-1">
									<span className="block truncate text-sm font-medium">{issue.title}</span>
									<span className="text-xs text-muted-foreground">
										{workflowStateLabel(issue.state.name, t)} · {issue.team.name}
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
