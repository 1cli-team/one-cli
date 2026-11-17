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
import { GatewayIssuePicker } from "./GatewayIssuePicker";
import {
	getProjectBindings,
	projectBindingsKey,
	type CodexOptions,
	type ProjectBinding,
} from "./api";

export type GatewayIssueAction = "new" | "dispatch";

function bindingId(project: ProjectBinding) {
	const { kind, id } = project.binding.scope;
	return `${kind}:${id}`;
}

export function GatewayIssueDialog({
	action,
	mappedProjectIds,
	onClose,
	onDispatched,
}: {
	action: GatewayIssueAction;
	mappedProjectIds?: string[];
	onClose(): void;
	onDispatched(): void;
}) {
	const { t } = useTranslation();
	const { mutate } = useSWRConfig();
	const bindings = useSWR(mappedProjectIds?.length ? projectBindingsKey : null, getProjectBindings);
	const titleId = useId();
	const createFormId = useId();
	const [project, setProject] = useState("");
	const [title, setTitle] = useState("");
	const [description, setDescription] = useState("");
	const [team, setTeam] = useState("");
	const [issueId, setIssueId] = useState("");
	const [options, setOptions] = useState<CodexOptions>({});
	const [created, setCreated] = useState<Issue>();
	const [pendingCreate, setPendingCreate] = useState("");
	const [busy, setBusy] = useState<"creating" | "dispatching">();
	const [error, setError] = useState("");
	const isNew = action === "new";
	const projects =
		bindings.data?.filter(({ binding }) => mappedProjectIds?.includes(binding.project.id)) ?? [];
	const selected =
		projects.find((entry) => bindingId(entry) === project) ??
		(projects.length === 1 && !project ? projects[0] : undefined);
	const binding = selected?.binding;
	const teams = binding?.project.teams ?? [];
	const teamId =
		team ||
		teams.find(({ id }) => id === binding?.teamId)?.id ||
		(teams.length === 1 ? teams[0]?.id : "");
	const canCreate = Boolean(binding && title.trim() && teamId);
	const draftLocked = Boolean(busy || pendingCreate || created);

	async function submit(send: boolean) {
		if (!selected || !binding || busy) return;
		const { scope } = binding;
		let id = isNew ? created?.id : issueId;
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
				const { status } = await dispatchToGateway(scope, dispatchedId, options);
				void mutate<GatewayJobs>(
					gatewayJobsKey(scope),
					(current) => ({
						enabled: true,
						issues: { ...current?.issues, [dispatchedId]: status },
					}),
					{ revalidate: false },
				);
				onDispatched();
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
					{!mappedProjectIds?.length && (
						<p className="text-sm text-muted-foreground">
							{t(
								mappedProjectIds ? "gateway.issueActions.noMappedProjects" : "gateway.offlineHint",
							)}
						</p>
					)}
					{Boolean(mappedProjectIds?.length) && bindings.data && !projects.length && (
						<p className="text-sm text-muted-foreground">
							{t("gateway.issueActions.noMappedBindings")}
						</p>
					)}
					{projects.length > 0 && (
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
								setIssueId("");
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
					{!isNew && selected && (
						<GatewayIssuePicker
							key={bindingId(selected)}
							scope={selected.binding.scope}
							value={issueId}
							onChange={setIssueId}
							disabled={Boolean(busy)}
						/>
					)}
					{binding && !pendingCreate && (
						<div className="grid gap-2 border-t border-border pt-4">
							<CodexOptionsFields
								options={options}
								onChange={setOptions}
								disabled={Boolean(busy)}
							/>
						</div>
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
							disabled={Boolean(busy) || !(isNew ? created || canCreate : issueId)}
							onClick={() => void submit(true)}
						>
							{t(
								busy === "dispatching"
									? "gateway.codex.sending"
									: isNew && !created
										? "gateway.issueActions.createAndDispatch"
										: "gateway.dispatch",
							)}
						</Button>
					)}
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
