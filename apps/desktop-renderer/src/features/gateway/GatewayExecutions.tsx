import { GatewaySessionLink } from "@/features/openviking/GatewaySessionLink";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { LinearComments } from "@/features/linear/LinearComments";
import type { HttpError } from "@/types/api";
import { CodexResultMessage } from "./GatewayMarkdown";
import { GatewayProgress } from "./GatewayProgress";
import { GatewayOpenVikingSyncDialog } from "./GatewayOpenVikingSyncDialog";
import { GatewayQuestions } from "./GatewayQuestions";
import { GatewaySubtasks } from "./GatewaySubtasks";
import { openExternal } from "@/lib/desktop/runtime";
import Toast from "@/lib/toast";
import { getProjectBindings, projectBindingsKey } from "./api";
import {
	executionLogText,
	inviteWorker,
	repositoryKey,
	useExecution,
	type ExecutionState,
	type ExecutionWorker,
	type WorkerInvite,
} from "./executions";

export function GatewayWorkers({
	canInvite,
	connection,
	machines,
	isLoading,
	hasError,
}: {
	canInvite: boolean;
	connection: string;
	machines: ExecutionWorker[];
	isLoading: boolean;
	hasError: boolean;
}) {
	const { t, i18n } = useTranslation();
	const [invite, setInvite] = useState<WorkerInvite>();
	const [inviting, setInviting] = useState(false);
	const [inviteError, setInviteError] = useState("");
	const [syncTarget, setSyncTarget] = useState<ExecutionWorker>();

	async function pair() {
		setInviting(true);
		setInviteError("");
		try {
			setInvite(await inviteWorker());
		} catch (error) {
			setInviteError((error as HttpError).message || t("gateway.execution.loadError"));
		} finally {
			setInviting(false);
		}
	}

	return (
		<div className="flex flex-col gap-6">
			<section className="flex flex-col gap-3" aria-labelledby="gateway-workers-heading">
				<h2 id="gateway-workers-heading" className="sr-only">
					{t("gateway.execution.workers")}
				</h2>
				{canInvite && (
					<div className="flex justify-end">
						<Button variant="outline" size="sm" disabled={inviting} onClick={() => void pair()}>
							{t("gateway.execution.addWorker")}
						</Button>
					</div>
				)}
				{(hasError || inviteError) && (
					<p role="alert" className="text-sm text-destructive">
						{inviteError || t("gateway.execution.loadError")}
					</p>
				)}
				{isLoading ? (
					<Skeleton className="h-12 w-full" />
				) : (
					<div className="overflow-hidden rounded-lg border border-border bg-card">
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>{t("gateway.execution.worker")}</TableHead>
									<TableHead>{t("gateway.execution.status")}</TableHead>
									<TableHead className="text-right">
										{t("gateway.execution.workerActions")}
									</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{machines.map((worker) => (
									<TableRow key={worker.id}>
										<TableCell className="font-mono text-xs font-medium">{worker.name}</TableCell>
										<TableCell>
											<Badge
												variant={worker.online ? (worker.busy ? "warning" : "success") : "muted"}
											>
												<span aria-hidden="true" className="size-1.5 rounded-full bg-current" />
												{t(
													`gateway.execution.workerState.${worker.online ? (worker.busy ? "busy" : "available") : "offline"}`,
												)}
											</Badge>
										</TableCell>
										<TableCell className="text-right">
											{worker.remoteSetup ? (
												<Button
													variant="outline"
													size="sm"
													disabled={!worker.online}
													onClick={() => setSyncTarget(worker)}
													aria-label={t("gateway.execution.openViking.actionFor", {
														worker: worker.name,
													})}
												>
													{t("gateway.execution.openViking.title")}
												</Button>
											) : (
												<span className="text-xs text-muted-foreground">
													{t("gateway.execution.openViking.httpsRequired")}
												</span>
											)}
										</TableCell>
									</TableRow>
								))}
								{machines.length === 0 && (
									<TableRow>
										<TableCell colSpan={3}>{t("gateway.execution.noWorkers")}</TableCell>
									</TableRow>
								)}
							</TableBody>
						</Table>
					</div>
				)}
			</section>
			{syncTarget && (
				<GatewayOpenVikingSyncDialog
					connection={connection}
					worker={syncTarget}
					onClose={() => setSyncTarget(undefined)}
				/>
			)}

			{invite && (
				<Dialog
					open
					onOpenChange={(open) => {
						if (!open) setInvite(undefined);
					}}
				>
					<DialogContent>
						<DialogHeader>
							<DialogTitle>{t("gateway.execution.addWorker")}</DialogTitle>
							<DialogDescription>
								{t("gateway.execution.inviteDescription", {
									time: new Date(invite.expiresAt).toLocaleTimeString(i18n.resolvedLanguage),
								})}
							</DialogDescription>
						</DialogHeader>
						<pre className="overflow-x-auto rounded-md bg-muted p-3 text-sm">{`one-gateway worker join ${invite.url}\none-gateway worker run`}</pre>
						<code
							className="select-all break-all rounded-md bg-muted p-3 text-sm"
							aria-label={t("gateway.execution.inviteCode")}
						>
							{invite.code}
						</code>
					</DialogContent>
				</Dialog>
			)}
		</div>
	);
}

function ExecutionBadge({ state }: { state: ExecutionState }) {
	const { t } = useTranslation();
	return (
		<Badge variant={state === "failed" ? "destructive" : "outline"}>
			{t(`gateway.execution.state.${state}`)}
		</Badge>
	);
}

export function GatewayExecutionDetails({
	connection,
	id,
	workers,
	onClose,
	onRevised,
}: {
	connection: string;
	id: string;
	workers: ExecutionWorker[];
	onClose(): void;
	onRevised(id: string): void;
}) {
	const { t } = useTranslation();
	const { job, logs } = useExecution(connection, id);
	const result = job.data?.result;
	const session = result?.openViking ?? job.data?.progress?.openViking;
	const issue = job.data?.input.kind === "linear-workflow" ? job.data.input.linear : undefined;
	const commentIssueId = result?.revisionIssueId ?? issue?.issueId;
	const bindings = useSWR(issue ? projectBindingsKey : null, getProjectBindings);
	const binding = bindings.data?.find(
		({ binding }) => binding.project.id === issue?.project.id,
	)?.binding;
	const machine = workers.find(({ id }) => id === job.data?.workerId);
	const output = logs.data?.data ? executionLogText(logs.data.data) : "";
	return (
		<Dialog
			open
			onOpenChange={(open) => {
				if (!open) onClose();
			}}
		>
			<DialogContent className="ud-sm:max-w-3xl">
				<DialogHeader>
					<DialogTitle>{t("gateway.execution.details")}</DialogTitle>
					<DialogDescription className="break-all">{id}</DialogDescription>
				</DialogHeader>
				{job.isLoading && <Skeleton className="h-12 w-full" />}
				{job.error && (
					<p role="alert" className="text-sm text-destructive">
						{t("gateway.execution.loadError")}
					</p>
				)}
				{job.data && (
					<div className="flex flex-col gap-3">
						<div className="flex flex-wrap items-center gap-2">
							<ExecutionBadge state={job.data.state} />
							{issue?.revision && (
								<Badge variant="secondary">{t("gateway.execution.revision")}</Badge>
							)}
							<span className="text-sm font-medium">{machine?.name ?? job.data.workerId}</span>
							<span className="text-sm text-muted-foreground">
								{job.data.input.kind === "linear-workflow"
									? job.data.input.linear.identifier + " " + job.data.input.linear.title
									: repositoryKey(job.data.input.repository)}
							</span>
						</div>
						{issue?.url && (
							<Button
								variant="link"
								className="w-fit p-0"
								onClick={() => {
									void openExternal(issue.url).catch(() =>
										Toast.error(t("gateway.taskList.linkError")),
									);
								}}
							>
								{t("gateway.taskList.openLinear")}
							</Button>
						)}

						{job.data.reason && (
							<p className="text-sm text-muted-foreground">
								{t(`gateway.execution.reason.${job.data.reason}`)}
							</p>
						)}
						{job.data.state === "queued" && !job.data.reason && machine?.busy && (
							<p className="text-sm text-muted-foreground">
								{t("gateway.execution.waiting", { name: machine.name })}
							</p>
						)}
						<GatewayQuestions
							job={job.data}
							onAnswered={(answer) => {
								void job.mutate(
									(current) =>
										current && {
											...current,
											questions: current.questions?.map((question) =>
												question.id === answer.id ? answer : question,
											),
										},
								);
							}}
						/>
						{job.data.progress && (
							<GatewayProgress
								key={job.data.progress.id}
								jobId={job.data.id}
								progress={job.data.progress}
								state={job.data.state}
								onContinued={(progress) => {
									void job.mutate((current) =>
										current?.progress?.id === progress.id
											? {
													...current,
													progress,
													state: current.state === "waiting_to_continue" ? "queued" : current.state,
												}
											: current,
									);
								}}
							/>
						)}
						{result?.error && (
							<p role="alert" className="break-words text-sm text-destructive">
								{result.error}
							</p>
						)}
						{result?.exitCode !== undefined && (
							<p className="text-sm">
								{t("gateway.execution.exitCode", { code: result.exitCode })}
							</p>
						)}
						{result?.commitSha && <code className="break-all text-sm">{result.commitSha}</code>}
						{result?.ref && <code className="break-all text-sm">{result.ref}</code>}
						{result?.response && <CodexResultMessage value={result.response} />}
						{session && <GatewaySessionLink session={session} />}
						<GatewaySubtasks subtasks={job.data.subtasks ?? result?.subtasks} />
						{result?.pullRequests?.map(({ number, url }) => (
							<Button
								key={url}
								variant="link"
								className="w-fit p-0"
								onClick={() => {
									void openExternal(url).catch(() => Toast.error(t("gateway.taskList.linkError")));
								}}
							>
								{t("gateway.taskList.pr", { number })}
							</Button>
						))}
						{commentIssueId && binding && (
							<LinearComments
								key={commentIssueId}
								scope={binding.scope}
								issueId={commentIssueId}
								revisionJobId={job.data.canRevise ? id : undefined}
								selectedCommentId={issue?.revision?.commentId}
								onRevised={onRevised}
							/>
						)}
					</div>
				)}
				<div className="flex flex-col gap-2">
					<h3 className="text-sm font-medium">{t("gateway.execution.logs")}</h3>
					{logs.error && (
						<p role="alert" className="text-sm text-destructive">
							{t("gateway.execution.loadError")}
						</p>
					)}
					{logs.isLoading ? (
						<Skeleton className="h-16 w-full" />
					) : (
						<pre className="max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted p-3 text-xs">
							{output || t("gateway.execution.noLogs")}
						</pre>
					)}
					{logs.data?.truncated && (
						<p className="text-xs text-muted-foreground">{t("gateway.execution.truncated")}</p>
					)}
				</div>
			</DialogContent>
		</Dialog>
	);
}
