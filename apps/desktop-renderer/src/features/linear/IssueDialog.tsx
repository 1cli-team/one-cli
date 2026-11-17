import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { ExternalLink, GitBranchPlus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
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
import { Choice } from "./Choice";
import { CreateIssueBranchDialog } from "./CreateIssueBranchDialog";
import {
	createComment,
	createIssue,
	errorCode,
	errorText,
	fetchJSON,
	scoped,
	teamKey,
	updateIssue,
	type Binding,
	type Comment,
	type Issue,
	type IssueInput,
	type Operation,
	type Page,
	type Scope,
	type Team,
} from "./api";

export function IssueDialog({
	scope,
	binding,
	issueId,
	onClose,
	onSaved,
}: {
	scope: Scope;
	binding: Binding;
	issueId: string;
	onClose(): void;
	onSaved(): void;
}) {
	const { t } = useTranslation();
	const isNew = issueId === "new";
	const detail = useSWR<Issue>(
		!isNew ? scoped(scope, `issues/${encodeURIComponent(issueId)}`) : null,
		fetchJSON,
		{ revalidateOnFocus: false },
	);
	const [input, setInput] = useState<IssueInput>({
		title: "",
		description: "",
		teamId: binding.project.teams.some((team) => team.id === binding.teamId) ? binding.teamId : "",
		stateId: "",
		assigneeId: "",
		priority: 0,
		labelIds: [],
		operationId: crypto.randomUUID(),
	});
	const [loaded, setLoaded] = useState(isNew);
	const [creatingBranch, setCreatingBranch] = useState(false);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [pending, setPending] = useState<{ id: string; kind: "issue" | "comment" } | null>(null);
	const [comment, setComment] = useState("");
	const [comments, setComments] = useState<Comment[]>([]);
	const [commentPage, setCommentPage] = useState({ endCursor: "", hasNextPage: false });
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
			setComments(issue.comments);
			setCommentPage(issue.commentsPageInfo);
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
			if (isNew) onClose();
			else {
				setInput((current) => ({ ...current, expectedUpdatedAt: saved.updatedAt }));
				await detail.mutate();
			}
		} catch (e) {
			setError(errorText(e));
			if (isNew && (errorCode(e) === "outcome_unknown" || !errorCode(e)))
				setPending({ id: input.operationId!, kind: "issue" });
		} finally {
			setBusy(false);
		}
	}
	async function addComment() {
		const id = crypto.randomUUID();
		setBusy(true);
		setError("");
		try {
			const added = await createComment(scope, issueId, comment, id);
			setComments((current) => [...current, added]);
			setComment("");
			onSaved();
		} catch (e) {
			setError(errorText(e));
			if (errorCode(e) === "outcome_unknown" || !errorCode(e)) setPending({ id, kind: "comment" });
		} finally {
			setBusy(false);
		}
	}
	async function reconcile() {
		if (!pending) return;
		setBusy(true);
		setError("");
		try {
			const result = await fetchJSON<Operation>(scoped(scope, `operations/${pending.id}`));
			if (result.status === "confirmed") {
				onSaved();
				if (pending.kind === "issue") onClose();
				else {
					setPending(null);
					setComment("");
					setLoaded(false);
					await detail.mutate();
				}
			} else setError(t("linear.stillUnknown"));
		} catch (e) {
			setError(errorText(e));
		} finally {
			setBusy(false);
		}
	}
	return (
		<Dialog
			open
			onOpenChange={(open) => {
				if (!open && !busy) onClose();
			}}
		>
			<DialogContent
				className="fixed top-0 right-0 left-auto h-dvh max-h-dvh w-full max-w-[640px] translate-x-0 translate-y-0 gap-5 overflow-y-auto rounded-none p-6 ud-sm:max-w-[640px]"
				showCloseButton={!busy}
			>
				<DialogHeader>
					<DialogTitle>
						{isNew ? t("linear.create") : detail.data?.identifier || t("linear.issue")}
					</DialogTitle>
					<DialogDescription>{binding.project.name}</DialogDescription>
				</DialogHeader>
				{!isNew && detail.data && (
					<Button
						type="button"
						variant="outline"
						className="justify-self-start"
						disabled={busy || !!pending}
						onClick={() => setCreatingBranch(true)}
					>
						<GitBranchPlus />
						{t("linear.createBranch")}
					</Button>
				)}
				{creatingBranch && detail.data && (
					<CreateIssueBranchDialog
						scope={scope}
						issue={detail.data}
						onClose={() => setCreatingBranch(false)}
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
						<p className="break-all font-mono text-xs">{pending.id}</p>
						<Button variant="outline" disabled={busy} onClick={() => void reconcile()}>
							{t("linear.reconcile")}
						</Button>
					</div>
				)}
				{!loaded && !detail.error && <p role="status">{t("linear.loading")}</p>}
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
									<Label htmlFor="issue-title">{t("linear.title")}</Label>
									<Input
										id="issue-title"
										value={input.title}
										onChange={(e) => field("title", e.target.value)}
										required
										autoFocus={isNew}
									/>
								</div>
								<div className="space-y-2">
									<Label htmlFor="issue-description">{t("linear.description")}</Label>
									<Textarea
										id="issue-description"
										className="min-h-32"
										value={input.description}
										onChange={(e) => field("description", e.target.value)}
									/>
								</div>
								<div className="grid grid-cols-2 gap-4">
									<Choice
										label={t("linear.team")}
										value={input.teamId}
										options={binding.project.teams}
										disabled={!isNew}
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
										options={team.data?.states || []}
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
												htmlFor={`issue-label-${label.id}`}
												className="flex items-center gap-2 text-sm font-normal"
											>
												<Checkbox
													id={`issue-label-${label.id}`}
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
												{label.name}
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
										{busy ? t("git.working") : t(isNew ? "linear.create" : "groups.save")}
									</Button>
								</div>
							</fieldset>
						</form>
						{!isNew && (
							<section className="space-y-4 border-t border-border pt-5">
								<div className="flex items-center justify-between">
									<h3 className="text-sm font-medium">{t("linear.comments")}</h3>
									<Button
										variant="ghost"
										size="sm"
										onClick={() =>
											void openExternal(detail.data!.url).catch((e) => setError(errorText(e)))
										}
									>
										<ExternalLink />
										{t("linear.openLinear")}
									</Button>
								</div>
								<div className="space-y-4">
									{comments.map((item) => (
										<article key={item.id} className="space-y-1">
											<p className="text-xs text-muted-foreground">
												{item.user?.name || t("linear.unknownUser")} ·{" "}
												{new Date(item.createdAt).toLocaleString()}
											</p>
											<p className="whitespace-pre-wrap break-words text-sm">{item.body}</p>
										</article>
									))}
								</div>
								{commentPage.hasNextPage && (
									<Button
										variant="outline"
										disabled={busy}
										onClick={() => {
											setBusy(true);
											void fetchJSON<Page<Comment>>(
												scoped(scope, `issues/${encodeURIComponent(issueId)}/comments`, {
													after: commentPage.endCursor,
												}),
											)
												.then((page) => {
													setComments((current) => [
														...current,
														...page.nodes.filter((c) => !current.some((old) => old.id === c.id)),
													]);
													setCommentPage(page.pageInfo);
												})
												.catch((e) => setError(errorText(e)))
												.finally(() => setBusy(false));
										}}
									>
										{t("linear.moreComments")}
									</Button>
								)}
								<form
									className="space-y-2"
									onSubmit={(e) => {
										e.preventDefault();
										void addComment();
									}}
								>
									<Label htmlFor="issue-comment">{t("linear.addComment")}</Label>
									<Textarea
										id="issue-comment"
										value={comment}
										onChange={(e) => setComment(e.target.value)}
										disabled={busy || !!pending}
									/>
									<Button type="submit" disabled={busy || !!pending || !comment.trim()}>
										{t("linear.sendComment")}
									</Button>
								</form>
							</section>
						)}
					</>
				)}
			</DialogContent>
		</Dialog>
	);
}
