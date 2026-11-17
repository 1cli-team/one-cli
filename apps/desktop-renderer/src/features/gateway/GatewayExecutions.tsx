import { useState } from "react";
import { useTranslation } from "react-i18next";
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
import { Choice } from "@/features/linear/Choice";
import type { HttpError } from "@/types/api";
import { CodexResultMessage } from "./GatewayMarkdown";
import { GatewayProgress } from "./GatewayProgress";
import { GatewayQuestions } from "./GatewayQuestions";
import { openExternal } from "@/lib/desktop/runtime";
import Toast from "@/lib/toast";
import {
	executionLogText,
	executionStates,
	inviteWorker,
	repositoryKey,
	useExecution,
	useExecutions,
	useWorkers,
	type ExecutionState,
	type ExecutionWorker,
	type WorkerInvite,
} from "./executions";

export function GatewayExecutions({
	connection,
	canInvite,
	selected,
	onSelect,
}: {
	connection: string;
	canInvite: boolean;
	selected?: string;
	onSelect(id?: string): void;
}) {
	const { t, i18n } = useTranslation();
	const workers = useWorkers(connection);
	const executions = useExecutions(connection);
	const [workerFilter, setWorkerFilter] = useState("");
	const [stateFilter, setStateFilter] = useState("");
	const [sourceFilter, setSourceFilter] = useState("");
	const [invite, setInvite] = useState<WorkerInvite>();
	const [inviting, setInviting] = useState(false);
	const [inviteError, setInviteError] = useState("");
	const machines = workers.data ?? [];
	const jobs = (executions.data ?? []).filter(
		(job) =>
			(!workerFilter || job.workerId === workerFilter) &&
			(!stateFilter || job.state === stateFilter) &&
			(!sourceFilter || job.source === sourceFilter),
	);

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
				<div className="flex flex-wrap items-center justify-between gap-2">
					<h2 id="gateway-workers-heading" className="text-base font-semibold">
						{t("gateway.execution.workers")}
					</h2>
					{canInvite && (
						<Button variant="outline" size="sm" disabled={inviting} onClick={() => void pair()}>
							{t("gateway.execution.addWorker")}
						</Button>
					)}
				</div>
				{(workers.error || inviteError) && (
					<p role="alert" className="text-sm text-destructive">
						{inviteError || t("gateway.execution.loadError")}
					</p>
				)}
				{workers.isLoading ? (
					<Skeleton className="h-12 w-full" />
				) : (
					<div className="rounded-lg border">
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>{t("gateway.execution.worker")}</TableHead>
									<TableHead>{t("gateway.execution.status")}</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{machines.map((worker) => (
									<TableRow key={worker.id}>
										<TableCell className="font-medium">{worker.name}</TableCell>
										<TableCell>
											<Badge variant="outline">
												{t(
													`gateway.execution.workerState.${worker.online ? (worker.busy ? "busy" : "available") : "offline"}`,
												)}
											</Badge>
										</TableCell>
									</TableRow>
								))}
								{machines.length === 0 && (
									<TableRow>
										<TableCell colSpan={2}>{t("gateway.execution.noWorkers")}</TableCell>
									</TableRow>
								)}
							</TableBody>
						</Table>
					</div>
				)}
			</section>

			<section className="flex flex-col gap-3" aria-labelledby="gateway-executions-heading">
				<h2 id="gateway-executions-heading" className="text-base font-semibold">
					{t("gateway.execution.jobs")}
				</h2>
				<div className="grid gap-3 sm:grid-cols-3">
					<Choice
						label={t("gateway.execution.worker")}
						value={workerFilter}
						options={machines.map(({ id, name }) => ({ id, name }))}
						placeholder={t("gateway.execution.all")}
						onChange={setWorkerFilter}
					/>
					<Choice
						label={t("gateway.execution.status")}
						value={stateFilter}
						options={executionStates.map((id) => ({
							id,
							name: t(`gateway.execution.state.${id}`),
						}))}
						placeholder={t("gateway.execution.all")}
						onChange={setStateFilter}
					/>
					<Choice
						label={t("gateway.execution.sourceLabel")}
						value={sourceFilter}
						options={["desktop", "cli"].map((id) => ({
							id,
							name: t(`gateway.execution.source.${id}`),
						}))}
						placeholder={t("gateway.execution.all")}
						onChange={setSourceFilter}
					/>
				</div>
				{executions.error && (
					<p role="alert" className="text-sm text-destructive">
						{t("gateway.execution.loadError")}
					</p>
				)}
				{executions.isLoading ? (
					<Skeleton className="h-12 w-full" />
				) : (
					<div className="rounded-lg border">
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>{t("gateway.execution.task")}</TableHead>
									<TableHead>{t("gateway.execution.worker")}</TableHead>
									<TableHead>{t("gateway.execution.sourceLabel")}</TableHead>
									<TableHead>{t("gateway.execution.status")}</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{jobs.map((job) => (
									<TableRow key={job.id}>
										<TableCell>
											<Button
												variant="link"
												size="sm"
												className="h-auto p-0"
												onClick={() => onSelect(job.id)}
											>
												{job.input.kind === "linear-workflow"
													? job.input.linear.identifier + " " + job.input.linear.title
													: job.input.task || t("gateway.dispatch")}
											</Button>
											{job.input.kind !== "linear-workflow" && (
												<span className="block text-xs text-muted-foreground">
													{repositoryKey(job.input.repository)}
												</span>
											)}
										</TableCell>
										<TableCell>
											{machines.find((worker) => worker.id === job.workerId)?.name || job.workerId}
										</TableCell>
										<TableCell>{t(`gateway.execution.source.${job.source}`)}</TableCell>
										<TableCell>
											<ExecutionBadge state={job.state} />
										</TableCell>
									</TableRow>
								))}
								{jobs.length === 0 && (
									<TableRow>
										<TableCell colSpan={4}>{t("gateway.execution.noJobs")}</TableCell>
									</TableRow>
								)}
							</TableBody>
						</Table>
					</div>
				)}
			</section>
			{selected && (
				<ExecutionDetails
					connection={connection}
					id={selected}
					workers={machines}
					onClose={() => onSelect()}
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

function ExecutionDetails({
	connection,
	id,
	workers,
	onClose,
}: {
	connection: string;
	id: string;
	workers: ExecutionWorker[];
	onClose(): void;
}) {
	const { t } = useTranslation();
	const { job, logs } = useExecution(connection, id);
	const result = job.data?.result;
	const issue = job.data?.input.kind === "linear-workflow" ? job.data.input.linear : undefined;
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
