import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR, { useSWRConfig } from "swr";
import { ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Choice } from "@/features/linear/Choice";
import { IssueMarkdownField } from "@/features/linear/IssueMarkdownField";
import {
	createIssue,
	dispatchToGateway,
	errorCode,
	errorText,
	gatewayJobsKey,
	scoped,
	type GatewayJobs,
	type Issue,
} from "@/features/linear/api";
import { openExternal } from "@/lib/desktop/runtime";
import Toast from "@/lib/toast";
import { CodexOptionsFields } from "./CodexOptionsFields";
import { supportsLinear, type ExecutionWorker } from "./executions";
import { GatewayIssuePicker, type IssueSelection } from "./GatewayIssuePicker";
import {
	getProjectBindings,
	projectBindingsKey,
	type CodexOptions,
	type ProjectBinding,
	type ProjectSummary,
} from "./api";

export type GatewayIssueAction = "new" | "dispatch";

function bindingId(project: ProjectBinding) {
	const { kind, id } = project.binding.scope;
	return `${kind}:${id}`;
}

export function GatewayIssueDialog({
	action,
	mappedProjects,
	machines,
	onClose,
	onDispatched,
}: {
	action: GatewayIssueAction;
	mappedProjects?: ProjectSummary[];
	machines?: ExecutionWorker[];
	onClose(): void;
	onDispatched(id?: string): void;
}) {
	const { t } = useTranslation();
	const { mutate } = useSWRConfig();
	const bindings = useSWR(mappedProjects?.length ? projectBindingsKey : null, getProjectBindings);
	const titleId = useId();
	const createFormId = useId();
	const [project, setProject] = useState("");
	const [title, setTitle] = useState("");
	const [description, setDescription] = useState("");
	const [team, setTeam] = useState("");
	const [issue, setIssue] = useState<IssueSelection>();
	const [workerId, setWorkerId] = useState("");
	const [targetOptions, setTargetOptions] = useState<{
		workerId?: string;
		options: CodexOptions;
	}>();
	const [created, setCreated] = useState<Issue>();
	const [pendingCreate, setPendingCreate] = useState("");
	const [busy, setBusy] = useState<"creating" | "dispatching">();
	const [error, setError] = useState("");
	const isNew = action === "new";
	const projects =
		bindings.data?.filter(({ binding }) =>
			mappedProjects?.some((entry) => entry.projectId === binding.project.id),
		) ?? [];
	const selected = isNew
		? (projects.find((entry) => bindingId(entry) === project) ??
			(projects.length === 1 && !project ? projects[0] : undefined))
		: projects.find(({ binding }) => binding.project.id === issue?.projectId);
	const binding = selected?.binding;
	const mapped = mappedProjects?.find((entry) => entry.projectId === binding?.project.id);
	const compatible = mapped
		? (machines ?? []).filter((worker) => supportsLinear(worker, mapped))
		: [];
	const worker =
		compatible.find(({ id }) => id === workerId) ??
		(!workerId && compatible.length === 1 ? compatible[0] : undefined);
	const canSend = machines === undefined || !!worker;
	const options =
		targetOptions && targetOptions.workerId === worker?.id ? targetOptions.options : {};
	const teams = binding?.project.teams ?? [];
	const teamId =
		team ||
		teams.find(({ id }) => id === binding?.teamId)?.id ||
		(teams.length === 1 ? teams[0]?.id : "");
	const canCreate = Boolean(binding && title.trim() && teamId);
	const draftLocked = Boolean(busy || pendingCreate || created);

	async function submit(send: boolean) {
		if (!selected || !binding || busy || (send && !canSend)) return;
		const { scope } = binding;
		let id = isNew ? created?.id : issue?.id;
		if (!id && (!isNew || !canCreate)) return;
		setError("");
		try {
			if (!id) {
				setProject(bindingId(selected));
				setTeam(teamId);
				setBusy("creating");
				const operationId = pendingCreate || crypto.randomUUID();
				try {
					const saved = await createIssue(scope, {
						title,
						description,
						teamId,
						stateId: "",
						assigneeId: "",
						priority: 0,
						labelIds: [],
						operationId,
					});
					setCreated(saved);
					setPendingCreate("");
					id = saved.id;
					const issuesPath = scoped(scope, "issues");
					void mutate(
						(key) =>
							typeof key === "string" && (key === issuesPath || key.startsWith(`${issuesPath}&`)),
					);
				} catch (error) {
					const code = errorCode(error);
					if (code === "outcome_unknown" || !code) setPendingCreate(operationId);
					throw error;
				}
			}
			if (send) {
				const dispatchedId = id;
				setBusy("dispatching");
				const { status, jobId } = await dispatchToGateway(scope, dispatchedId, options, worker?.id);
				void mutate<GatewayJobs>(
					gatewayJobsKey(scope),
					(current) => ({
						enabled: true,
						issues: { ...current?.issues, [dispatchedId]: status },
					}),
					{ revalidate: false },
				);
				onDispatched(jobId);
				if (status === "failed") Toast.error(t("gateway.issueActions.alreadyFailed"));
				else Toast.success(t("gateway.issueActions.dispatched"));
				onClose();
			}
		} catch (error) {
			setError(errorText(error));
		} finally {
			setBusy(undefined);
		}
	}

	return (
		<Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
			<DialogContent showCloseButton={!busy}>
				<DialogHeader>
					<DialogTitle>
						{t(isNew ? "gateway.issueActions.new" : "gateway.issueActions.dispatch")}
					</DialogTitle>
					<DialogDescription>
						{t(
							isNew
								? "gateway.issueActions.newDescription"
								: "gateway.issueActions.dispatchDescription",
						)}
					</DialogDescription>
				</DialogHeader>
				<div className="grid gap-4">
					{bindings.error && !bindings.data && (
						<ErrorNotice>{errorText(bindings.error)}</ErrorNotice>
					)}
					{bindings.isLoading && (
						<p role="status" className="text-sm">
							{t("linear.loading")}
						</p>
					)}
					{!mappedProjects?.length && (
						<p className="text-sm text-muted-foreground">
							{t(mappedProjects ? "gateway.issueActions.noMappedProjects" : "gateway.offlineHint")}
						</p>
					)}
					{Boolean(mappedProjects?.length) && bindings.data && !projects.length && (
						<p className="text-sm text-muted-foreground">
							{t("gateway.issueActions.noMappedBindings")}
						</p>
					)}
					{isNew && projects.length > 0 && (
						<Choice
							label={t("gateway.linearProject")}
							value={selected ? bindingId(selected) : ""}
							options={projects.map((entry) => ({
								id: bindingId(entry),
								name:
									entry.name === entry.binding.project.name
										? entry.name
										: `${entry.binding.project.name} · ${entry.name}`,
							}))}
							placeholder={
								projects.length > 1 ? t("gateway.issueActions.selectProject") : undefined
							}
							disabled={draftLocked}
							onChange={(id) => {
								setProject(id);
								setTeam("");
								setWorkerId("");
								setTargetOptions(undefined);
								setError("");
							}}
						/>
					)}
					{isNew && binding && !created && (
						<form
							id={createFormId}
							className="grid gap-3"
							onSubmit={(event) => {
								event.preventDefault();
								void submit(false);
							}}
						>
							{(teams.length > 1 || !teamId) && (
								<Choice
									label={t("linear.team")}
									value={teamId}
									options={teams}
									disabled={draftLocked || teams.length === 0}
									onChange={setTeam}
								/>
							)}
							{teams.length === 0 && (
								<p className="text-sm text-muted-foreground">{t("linear.projectTeamsHint")}</p>
							)}
							<div className="grid gap-2">
								<Label htmlFor={titleId}>{t("linear.title")}</Label>
								<Input
									id={titleId}
									required
									value={title}
									disabled={draftLocked}
									onChange={(event) => setTitle(event.target.value)}
								/>
							</div>
							<IssueMarkdownField
								label={t("linear.description")}
								value={description}
								onChange={setDescription}
								disabled={draftLocked}
								compact
							/>
						</form>
					)}
					{isNew && created && (
						<div className="flex flex-wrap items-center gap-2">
							<p role="status" className="text-sm text-success-foreground">
								{t("gateway.issueActions.created", { issue: created.identifier })}
							</p>
							<Button
								type="button"
								variant="ghost"
								size="sm"
								onClick={() => {
									void openExternal(created.url).catch((error) => setError(errorText(error)));
								}}
							>
								<ExternalLink />
								{t("linear.openLinear")}
							</Button>
						</div>
					)}
					{!isNew && projects.length > 0 && (
						<GatewayIssuePicker
							projects={projects}
							value={issue?.id ?? ""}
							onChange={(nextIssue) => {
								setIssue(nextIssue);
								setWorkerId("");
								setTargetOptions(undefined);
								setError("");
							}}
							disabled={Boolean(busy)}
						/>
					)}
					{binding && !pendingCreate && machines !== undefined && (
						<div className="grid gap-2 border-t border-border pt-4">
							<Choice
								label={t("gateway.execution.worker")}
								value={worker?.id ?? ""}
								options={compatible.map(({ id, name }) => ({ id, name }))}
								placeholder={t("gateway.execution.chooseWorker")}
								disabled={Boolean(busy) || compatible.length === 0}
								onChange={(id) => {
									setWorkerId(id);
									setTargetOptions(undefined);
								}}
							/>
							{compatible.length === 0 && (
								<p className="text-sm text-muted-foreground">
									{t("gateway.execution.noIssueWorkers")}
								</p>
							)}
							{worker && (!worker.online || worker.busy) && (
								<p className="text-sm text-muted-foreground">
									{t("gateway.execution.waiting", { name: worker.name })}
								</p>
							)}
						</div>
					)}
					{binding && !pendingCreate && canSend && (
						<CodexOptionsFields
							worker={worker}
							options={options}
							onChange={(options) => setTargetOptions({ workerId: worker?.id, options })}
							disabled={Boolean(busy)}
						/>
					)}
					{pendingCreate && (
						<p role="status" className="text-sm">
							{t("linear.unknown")}
						</p>
					)}
					{error && <ErrorNotice>{error}</ErrorNotice>}
				</div>
				<DialogFooter>
					<Button type="button" variant="outline" disabled={Boolean(busy)} onClick={onClose}>
						{t(created ? "form.close" : "form.cancel")}
					</Button>
					{isNew && binding && !created && (
						<Button
							type="submit"
							form={createFormId}
							variant={pendingCreate ? "default" : "outline"}
							disabled={Boolean(busy) || !canCreate}
						>
							{t(
								busy === "creating"
									? "gateway.saving"
									: pendingCreate
										? "linear.reconcile"
										: "gateway.issueActions.createOnly",
							)}
						</Button>
					)}
					{binding && !pendingCreate && (
						<Button
							type="button"
							disabled={Boolean(busy) || !canSend || !(isNew ? created || canCreate : issue)}
							onClick={() => void submit(true)}
						>
							{t(
								busy === "dispatching"
									? "gateway.codex.sending"
									: isNew && !created
										? "gateway.issueActions.createAndDispatch"
										: "gateway.issueActions.dispatch",
							)}
						</Button>
					)}
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
