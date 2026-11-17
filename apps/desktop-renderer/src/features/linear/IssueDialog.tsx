import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import useSWRInfinite from "swr/infinite";
import { ExternalLink, FolderPlus, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
	DialogDescription,
} from "@/components/ui/dialog";
import { ErrorNotice } from "@/components/ui/page-layout";
import { openExternal } from "@/lib/desktop/runtime";
import { useGatewayStatus } from "@/features/gateway/api";
import { useExecutions } from "@/features/gateway/executions";
import { Choice } from "./Choice";
import { CreateIssueWorktreeDialog } from "./CreateIssueWorktreeDialog";
import { IssueMarkdownField } from "./IssueMarkdownField";
import { LinearComments } from "./LinearComments";
import { issueLabelName, workflowStateLabel } from "./display-labels";
import {
	createIssue,
	errorCode,
	errorText,
	fetchJSON,
	gatewayJobsKey,
	scoped,
	teamKey,
	updateIssue,
	type Binding,
	type GatewayJobs,
	type Issue,
	type IssueInput,
	type IssuePage,
	type Operation,
	type Scope,
	type Team,
} from "./api";

interface IssueDialogProps {
	scope: Scope;
	binding: Binding;
	issueId: string;
	onClose(): void;
	onSaved(): void;
}

export function IssueDialog(props: IssueDialogProps) {
	const [current, setCurrent] = useState(props.issueId);
	return <IssueEditor key={current} {...props} issueId={current} onNavigate={setCurrent} />;
}

function IssueEditor({
	scope,
	binding,
	issueId,
	parent,
	onClose,
	onSaved,
	onNavigate,
}: IssueDialogProps & {
	parent?: Issue;
	onNavigate(id: string): void;
}) {
	const { t } = useTranslation();
	const formId = useId();
	const isNew = issueId === "new";
	const detail = useSWR<Issue>(
		!isNew ? scoped(scope, `issues/${encodeURIComponent(issueId)}`) : null,
		fetchJSON,
		{ revalidateOnFocus: false },
	);
	const jobs = useSWR<GatewayJobs>(!isNew ? gatewayJobsKey(scope) : null, fetchJSON);
	const gatewayStatus = jobs.data?.issues[issueId];
	const gateway = useGatewayStatus();
	const executions = useExecutions(
		!isNew && gateway.data?.snapshot?.executions ? gateway.data.url : undefined,
	);
	const revisionJob = executions.data?.find((job) => {
		const issue = detail.data;
		if (!issue || !job.canRevise || job.input.kind !== "linear-workflow") return false;
		const task = job.input.linear;
		return (
			!task.revision &&
			task.project.id === binding.project.id &&
			(task.issueId === issue.id || task.subtaskOrder?.includes(issue.id))
		);
	});
	const [input, setInput] = useState<IssueInput>({
		parentId: parent?.id,
		title: "",
		description: "",
		teamId:
			parent?.team.id ??
			(binding.project.teams.some((team) => team.id === binding.teamId) ? binding.teamId : ""),
		stateId: "",
		assigneeId: "",
		priority: parent?.priority ?? 0,
		labelIds: [],
		operationId: crypto.randomUUID(),
	});
	const [loaded, setLoaded] = useState(isNew);
	const [open, setOpen] = useState(true);
	const [creatingWorktree, setCreatingWorktree] = useState(false);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [pending, setPending] = useState<string>();
	const team = useSWR<Team>(
		input.teamId ? teamKey(binding.connectionId, input.teamId) : null,
		fetchJSON,
	);
	useEffect(() => {
		if (detail.data && !loaded) {
			const issue = detail.data;
			setInput({
				title: issue.title,
				description: issue.description || "",
				teamId: issue.team.id,
				stateId: issue.state.id,
				assigneeId: issue.assignee?.id || "",
				priority: issue.priority,
				labelIds: issue.labels.map((l) => l.id),
				expectedUpdatedAt: issue.updatedAt,
			});
			setLoaded(true);
		}
	}, [detail.data, loaded]);
	function field<K extends keyof IssueInput>(key: K, value: IssueInput[K]) {
		setInput((current) => ({ ...current, [key]: value }));
	}
	async function save() {
		setBusy(true);
		setError("");
		try {
			const saved = isNew
				? await createIssue(scope, input)
				: await updateIssue(scope, issueId, input);
			onSaved();
			if (isNew) setOpen(false);
			else {
				setInput((current) => ({ ...current, expectedUpdatedAt: saved.updatedAt }));
				await detail.mutate();
			}
		} catch (e) {
			setError(errorText(e));
			if (isNew && (errorCode(e) === "outcome_unknown" || !errorCode(e)))
				setPending(input.operationId);
		} finally {
			setBusy(false);
		}
	}
	async function reconcile() {
		if (!pending) return;
		setBusy(true);
		setError("");
		try {
			const result = await fetchJSON<Operation>(scoped(scope, `operations/${pending}`));
			if (result.status === "confirmed") {
				onSaved();
				setOpen(false);
			} else setError(t("linear.stillUnknown"));
		} catch (e) {
			setError(errorText(e));
		} finally {
			setBusy(false);
		}
	}
	return (
		<Dialog
			open={open}
			onOpenChange={(next) => {
				if (!next && !busy) setOpen(false);
			}}
		>
			<DialogContent
				placement="right"
				className="max-w-[640px]"
				showCloseButton={!busy}
				onCloseAutoFocus={() => {
					if (!open) onClose();
				}}
			>
				<DialogHeader>
					<DialogTitle>
						{isNew
							? t(parent ? "linear.createSubtask" : "linear.create")
							: detail.data?.identifier || t("linear.issue")}
					</DialogTitle>
					<DialogDescription>
						{parent ? t("linear.parentIssue", { issue: parent.identifier }) : binding.project.name}
					</DialogDescription>
				</DialogHeader>
				{detail.data?.parent && (
					<Button
						type="button"
						variant="ghost"
						className="w-fit"
						disabled={busy || !!pending}
						onClick={() => onNavigate(detail.data!.parent!.id)}
					>
						{t("linear.parentIssue", { issue: detail.data.parent.identifier })}
					</Button>
				)}
				{!isNew && detail.data && (
					<div className="flex flex-wrap items-center gap-2">
						<Button
							type="button"
							variant="outline"
							disabled={busy || !!pending}
							onClick={() => setCreatingWorktree(true)}
						>
							<FolderPlus />
							{t("git.create")}
						</Button>
						<Button
							type="button"
							variant="ghost"
							size="sm"
							onClick={() =>
								void openExternal(detail.data!.url).catch((e) => setError(errorText(e)))
							}
						>
							<ExternalLink />
							{t("linear.openLinear")}
						</Button>
						{gatewayStatus === "processing" && (
							<span role="status" className="text-sm text-muted-foreground">
								{t("gateway.processing")}
							</span>
						)}
					</div>
				)}
				{creatingWorktree && detail.data && (
					<CreateIssueWorktreeDialog
						scope={scope}
						issue={detail.data}
						onClose={() => setCreatingWorktree(false)}
					/>
				)}
				{(error || detail.error || team.error) && (
					<ErrorNotice>{error || errorText(detail.error || team.error)}</ErrorNotice>
				)}
				{pending && (
					<div
						className="space-y-2 rounded-md border border-border bg-muted p-3 text-sm"
						role="status"
					>
						<p>{t("linear.unknown")}</p>
						<p className="break-all font-mono text-xs">{pending}</p>
						<Button variant="outline" disabled={busy} onClick={() => void reconcile()}>
							{t("linear.reconcile")}
						</Button>
					</div>
				)}
				{!loaded && !detail.error && (
					<p role="status" className="text-sm text-muted-foreground">
						{t("linear.loading")}
					</p>
				)}
				{loaded && (
					<>
						<form
							className="space-y-4"
							onSubmit={(e) => {
								e.preventDefault();
								void save();
							}}
						>
							<fieldset disabled={busy || !!pending} className="space-y-4 disabled:opacity-60">
								<div className="space-y-2">
									<Label htmlFor={`${formId}-title`}>{t("linear.title")}</Label>
									<Input
										id={`${formId}-title`}
										value={input.title}
										onChange={(e) => field("title", e.target.value)}
										required
										autoFocus={isNew}
									/>
								</div>
								<IssueMarkdownField
									label={t("linear.description")}
									value={input.description}
									onChange={(value) => field("description", value)}
									disabled={busy || !!pending}
								/>
								<div className="grid grid-cols-2 gap-4">
									<Choice
										label={t("linear.team")}
										value={input.teamId}
										options={binding.project.teams}
										disabled={!isNew || !!parent}
										placeholder={isNew ? t("linear.chooseTeam") : undefined}
										onChange={(teamId) =>
											setInput((current) => ({
												...current,
												teamId,
												stateId: "",
												assigneeId: "",
												labelIds: [],
											}))
										}
									/>
									<Choice
										label={t("linear.state")}
										value={input.stateId}
										options={
											team.data?.states.map(({ id, name }) => ({
												id,
												name: workflowStateLabel(name, t),
											})) || []
										}
										placeholder={t("linear.defaultState")}
										onChange={(v) => field("stateId", v)}
									/>
									<Choice
										label={t("linear.assignee")}
										value={input.assigneeId}
										options={team.data?.members || []}
										placeholder={t("linear.unassigned")}
										onChange={(v) => field("assigneeId", v)}
									/>
									<Choice
										label={t("linear.priority")}
										value={String(input.priority)}
										options={[0, 1, 2, 3, 4].map((n) => ({
											id: String(n),
											name: t(`linear.priorities.${n}`),
										}))}
										onChange={(v) => field("priority", Number(v))}
									/>
								</div>
								<fieldset className="space-y-2">
									<legend className="mb-2 text-sm font-medium">{t("linear.labels")}</legend>
									<div className="flex flex-wrap gap-3">
										{team.data?.labels.map((label) => (
											<Label
												key={label.id}
												htmlFor={`${formId}-label-${label.id}`}
												className="flex items-center gap-2 text-sm font-normal"
											>
												<Checkbox
													id={`${formId}-label-${label.id}`}
													checked={input.labelIds.includes(label.id)}
													onCheckedChange={(checked) =>
														field(
															"labelIds",
															checked === true
																? [...input.labelIds, label.id]
																: input.labelIds.filter((id) => id !== label.id),
														)
													}
												/>
												{issueLabelName(label.name, t)}
											</Label>
										))}
									</div>
									{!team.data?.labels.length && (
										<p className="text-sm text-muted-foreground">{t("linear.noLabels")}</p>
									)}
								</fieldset>
								<div className="flex justify-end gap-2">
									{!isNew && (
										<Button
											type="button"
											variant="outline"
											onClick={() => {
												setLoaded(false);
												void detail.mutate();
											}}
										>
											{t("linear.reload")}
										</Button>
									)}
									<Button type="submit" disabled={!input.title.trim() || !team.data}>
										{busy
											? t("git.working")
											: t(
													isNew
														? parent
															? "linear.createSubtask"
															: "linear.create"
														: "groups.save",
												)}
									</Button>
								</div>
							</fieldset>
						</form>
						{detail.data && (
							<IssueSubtasks
								issue={detail.data}
								scope={scope}
								binding={binding}
								onSaved={onSaved}
								onNavigate={onNavigate}
								disabled={busy || !!pending}
							/>
						)}
						{detail.data && (
							<LinearComments
								scope={scope}
								issueId={detail.data.id}
								revisionJobId={revisionJob?.id}
								onPosted={onSaved}
							/>
						)}
					</>
				)}
			</DialogContent>
		</Dialog>
	);
}

function IssueSubtasks({
	issue,
	scope,
	binding,
	onSaved,
	onNavigate,
	disabled,
}: {
	issue: Issue;
	scope: Scope;
	binding: Binding;
	onSaved(): void;
	onNavigate(id: string): void;
	disabled: boolean;
}) {
	const { t } = useTranslation();
	const [creating, setCreating] = useState(false);
	const pages = useSWRInfinite<IssuePage>((index, previous: IssuePage | null) => {
		if (index > 0 && !previous?.pageInfo.hasNextPage) return null;
		return scoped(scope, "issues", { parent: issue.id, after: previous?.pageInfo.endCursor ?? "" });
	}, fetchJSON);
	const children = [
		...new Map(
			pages.data?.flatMap((page) => page.nodes.map((child) => [child.id, child] as const)),
		).values(),
	];
	return (
		<section className="space-y-3 border-t border-border pt-5">
			<div className="flex flex-wrap items-center justify-between gap-2">
				<h3 className="text-sm font-medium">{t("linear.subtasks")}</h3>
				<Button
					type="button"
					variant="outline"
					size="sm"
					disabled={disabled}
					onClick={() => setCreating(true)}
				>
					<Plus />
					{t("linear.createSubtask")}
				</Button>
			</div>
			{pages.error && <ErrorNotice>{errorText(pages.error)}</ErrorNotice>}
			{pages.isLoading && (
				<p role="status" className="text-sm text-muted-foreground">
					{t("linear.loading")}
				</p>
			)}
			{pages.data && children.length === 0 && (
				<p className="text-sm text-muted-foreground">{t("linear.noSubtasks")}</p>
			)}
			{children.length > 0 && (
				<ul className="divide-y divide-border rounded-md border border-border">
					{children.map((child) => (
						<li key={child.id}>
							<button
								type="button"
								disabled={disabled}
								onClick={() => onNavigate(child.id)}
								className="flex w-full flex-wrap items-baseline justify-between gap-x-4 gap-y-1 px-3 py-2 text-left text-sm hover:bg-muted"
							>
								<span className="min-w-0 wrap-break-word">
									<span className="font-mono text-muted-foreground">{child.identifier}</span>{" "}
									{child.title}
								</span>
								<span className="text-xs text-muted-foreground">
									{workflowStateLabel(child.state.name, t)}
								</span>
							</button>
						</li>
					))}
				</ul>
			)}
			{pages.data?.at(-1)?.pageInfo.hasNextPage && (
				<Button
					type="button"
					variant="outline"
					size="sm"
					disabled={pages.isValidating}
					onClick={() => void pages.setSize(pages.size + 1)}
				>
					{t("linear.moreSubtasks")}
				</Button>
			)}
			{creating && (
				<IssueEditor
					scope={scope}
					binding={binding}
					issueId="new"
					parent={issue}
					onClose={() => setCreating(false)}
					onNavigate={onNavigate}
					onSaved={() => {
						void pages.mutate();
						onSaved();
					}}
				/>
			)}
		</section>
	);
}
