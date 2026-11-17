import { useState } from "react";
import { useTranslation } from "react-i18next";
import useSWRInfinite from "swr/infinite";
import { Button } from "@/components/ui/button";
import { ErrorNotice } from "@/components/ui/page-layout";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { reviseExecution } from "@/features/gateway/executions";
import { IssueMarkdownField } from "./IssueMarkdownField";
import {
	createComment,
	errorCode,
	errorText,
	fetchJSON,
	scoped,
	type Comment,
	type Operation,
	type Page,
	type Scope,
} from "./api";

export function LinearComments({
	scope,
	issueId,
	revisionJobId,
	selectedCommentId,
	onPosted,
	onRevised,
}: {
	scope: Scope;
	issueId: string;
	revisionJobId?: string;
	selectedCommentId?: string;
	onPosted?(): void;
	onRevised?(id: string): void;
}) {
	const { t, i18n } = useTranslation();
	const [body, setBody] = useState("");
	const [posted, setPosted] = useState<Comment[]>([]);
	const [pendingId, setPendingId] = useState("");
	const [revisionId, setRevisionId] = useState("");
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const pages = useSWRInfinite<Page<Comment>>((index, previous: Page<Comment> | null) => {
		if (index > 0 && !previous?.pageInfo.hasNextPage) return null;
		return scoped(scope, `issues/${encodeURIComponent(issueId)}/comments`, {
			after: previous?.pageInfo.endCursor ?? "",
		});
	}, fetchJSON);
	const comments = [
		...new Map(
			[...(pages.data?.flatMap((page) => page.nodes ?? []) ?? []), ...posted].map(
				(comment) => [comment.id, comment] as const,
			),
		).values(),
	];

	async function post() {
		const id = crypto.randomUUID();
		setBusy(true);
		setError("");
		try {
			const added = await createComment(scope, issueId, body, id);
			setPosted((current) => [...current, added]);
			setBody("");
			void pages.mutate();
		} catch (e) {
			setError(errorText(e));
			if (errorCode(e) === "outcome_unknown" || !errorCode(e)) setPendingId(id);
		} finally {
			setBusy(false);
			onPosted?.();
		}
	}

	async function reconcile() {
		setBusy(true);
		setError("");
		try {
			const result = await fetchJSON<Operation>(scoped(scope, `operations/${pendingId}`));
			if (result.status === "confirmed") {
				setPendingId("");
				setBody("");
				await pages.mutate();
				onPosted?.();
			} else setError(t("linear.stillUnknown"));
		} catch (e) {
			setError(errorText(e));
		} finally {
			setBusy(false);
		}
	}

	async function revise(commentId: string) {
		if (!revisionJobId) return;
		setBusy(true);
		setError("");
		try {
			const job = await reviseExecution(revisionJobId, commentId);
			setRevisionId(job.id);
			onRevised?.(job.id);
		} catch (e) {
			setError(errorText(e));
		} finally {
			setBusy(false);
		}
	}

	return (
		<section className="space-y-3 border-t border-border pt-4">
			<div className="flex items-center justify-between gap-2">
				<h3 className="text-sm font-medium">{t("linear.comments")}</h3>
				<Button
					variant="ghost"
					size="sm"
					disabled={pages.isValidating}
					onClick={() => void pages.mutate()}
				>
					{t("gateway.refresh")}
				</Button>
			</div>
			{(error || pages.error) && <ErrorNotice>{error || errorText(pages.error)}</ErrorNotice>}
			{revisionId && (
				<EnvironmentLink
					className="text-sm text-primary underline"
					to={`/gateway?execution=${revisionId}`}
				>
					{t("gateway.execution.viewRevision")}
				</EnvironmentLink>
			)}
			{pendingId && (
				<div
					role="status"
					className="space-y-2 rounded-md border border-border bg-muted p-3 text-sm"
				>
					<p>{t("linear.unknown")}</p>
					<Button
						type="button"
						variant="outline"
						size="sm"
						disabled={busy}
						onClick={() => void reconcile()}
					>
						{t("linear.reconcile")}
					</Button>
				</div>
			)}
			{pages.isLoading && (
				<p role="status" className="text-sm text-muted-foreground">
					{t("linear.loading")}
				</p>
			)}
			{pages.data && comments.length === 0 && (
				<p className="text-sm text-muted-foreground">{t("linear.noComments")}</p>
			)}
			{comments.map((comment) => (
				<article key={comment.id} className="space-y-1">
					<div className="flex flex-wrap items-center justify-between gap-2">
						<p className="text-xs text-muted-foreground">
							{comment.user?.name || t("linear.unknownUser")} ·{" "}
							{new Date(comment.createdAt).toLocaleString(i18n.resolvedLanguage)}
							{comment.id === selectedCommentId && (
								<span className="ml-2 font-medium text-foreground">
									{t("gateway.execution.revisionComment")}
								</span>
							)}
						</p>
						{revisionJobId && (
							<Button
								type="button"
								variant="outline"
								size="sm"
								disabled={busy}
								onClick={() => void revise(comment.id)}
							>
								{t("gateway.execution.reviseFromComment")}
							</Button>
						)}
					</div>
					<p className="whitespace-pre-wrap wrap-anywhere text-sm">{comment.body}</p>
				</article>
			))}
			{pages.data?.at(-1)?.pageInfo.hasNextPage && (
				<Button
					type="button"
					variant="outline"
					size="sm"
					disabled={pages.isValidating}
					onClick={() => void pages.setSize(pages.size + 1)}
				>
					{t("linear.moreComments")}
				</Button>
			)}
			<form
				className="space-y-2"
				onSubmit={(event) => {
					event.preventDefault();
					void post();
				}}
			>
				<IssueMarkdownField
					label={t("linear.addComment")}
					value={body}
					onChange={setBody}
					disabled={busy || !!pendingId}
					compact
				/>
				<Button type="submit" disabled={busy || !!pendingId || !body.trim()}>
					{t("linear.sendComment")}
				</Button>
			</form>
		</section>
	);
}
