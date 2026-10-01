import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useSearchParams } from "react-router-dom";
import useSWR, { useSWRConfig } from "swr";
import {
	getProject,
	getProjects,
	getSession,
	initializeGlobalLocation,
	message,
	sessionKey,
	type GlobalLocation,
} from "@/api/session";
import {
	bindWorkspaceEnvironment,
	getOverview,
	getWorkspaceEnvironment,
	overviewKeyFor,
	workspaceEnvironmentKey,
} from "@/api/workspace";
import { workspaceBasePath } from "@/api/workspaces";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { LocationPicker } from "@/features/global-variables/LocationPicker";
import { ManifestSaveControl } from "@/features/manifest-draft/ManifestSaveControl";
import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import { useToast } from "@/hooks/useToast";
import type { HttpError } from "@/types/api";

type Props = {
	open: boolean;
	onOpenChange(open: boolean): void;
	scope: "global" | "workspace";
	workspaceEntryId?: string;
	environment?: string;
	initial?: GlobalLocation;
	readOnly?: boolean;
};

export function InfisicalBindingDialog(props: Props) {
	const session = useSWR(sessionKey, getSession);
	const account = session.data?.session;
	// Mount a fresh form for each opening and workspace/account scope.
	return (
		<Dialog open={props.open} onOpenChange={props.onOpenChange}>
			{props.open && (
				<BindingForm
					key={`${props.scope}:${props.workspaceEntryId}:${account?.siteUrl}:${account?.userId}:${account?.organizationId}`}
					{...props}
				/>
			)}
		</Dialog>
	);
}

function BindingForm({
	scope,
	workspaceEntryId,
	environment,
	initial,
	readOnly,
	onOpenChange,
}: Props) {
	const { t } = useTranslation();
	const toast = useToast();
	const { mutate } = useSWRConfig();
	const [, setSearchParams] = useSearchParams();
	const session = useSWR(sessionKey, getSession);
	const workspace = scope === "workspace";
	const settings = useSWR(workspace ? workspaceEnvironmentKey(workspaceEntryId) : null, () =>
		getWorkspaceEnvironment(workspaceEntryId),
	);
	const overview = useSWR(workspace ? overviewKeyFor(workspaceEntryId) : null, () =>
		getOverview(workspaceEntryId),
	);
	const projects = useSWR(
		session.data?.session.loggedIn ? "/infisical/projects" : null,
		getProjects,
	);
	const [mode, setMode] = useState("automatic");
	const [project, setProject] = useState("");
	const detail = useSWR(
		workspace && mode === "existing" && project ? `/infisical/projects/${project}` : null,
		() => getProject(project),
	);
	const [busy, setBusy] = useState(false);
	const pending = useRef(false);
	const active = useRef(true);
	const [error, setError] = useState("");
	const [conflict, setConflict] = useState(false);
	const draft = useManifestDraftStore((s) =>
		workspace ? s.drafts[manifestDraftKey(workspaceEntryId)] : undefined,
	);
	const initialId = workspace ? settings.data?.projectId : initial?.projectId;
	const initialized = useRef(false);
	useEffect(() => {
		if (initialized.current || (workspace && !settings.data)) return;
		initialized.current = true;
		if (initialId) {
			setMode("existing");
			setProject(initialId);
		}
	}, [workspace, settings.data, initialId]);
	useEffect(() => {
		active.current = true;
		return () => {
			active.current = false;
		};
	}, []);
	const account = session.data?.session;
	const identity = `${account?.siteUrl}:${account?.userId}:${account?.organizationId}`;
	const currentIdentity = useRef(identity);
	currentIdentity.current = identity;
	const currentName =
		projects.data?.find((p) => p.id === initialId)?.name || initial?.projectName || initialId;
	const targetName = workspace ? overview.data?.workspace?.name : "shared-credentials";
	const isExisting = mode === "existing";
	const canBind =
		!readOnly &&
		!busy &&
		!draft &&
		!conflict &&
		account?.loggedIn &&
		account.organizationId &&
		(!workspace || (settings.data && overview.data && !settings.error && !overview.error)) &&
		(!isExisting ||
			!workspace ||
			(detail.data?.environments.some((e) => e.slug === "dev") && !detail.error));

	async function finish(name: string) {
		if (!active.current) return;
		toast.success(t("binding.success", { name }));
		onOpenChange(false);
		await Promise.allSettled([
			mutate(
				(key) =>
					typeof key === "string" &&
					(workspace
						? key.startsWith(`${workspaceBasePath(workspaceEntryId)}/`)
						: key === "/global-env/location"),
			),
			mutate("/infisical/projects"),
		]);
	}
	async function bind() {
		if (pending.current || !canBind) return;
		pending.current = true;
		setBusy(true);
		setError("");
		const submittedIdentity = identity;
		try {
			if (workspace) {
				const result = await bindWorkspaceEnvironment(workspaceEntryId, {
					revision: settings.data!.revision,
					create: !isExisting,
					...(isExisting ? { projectId: project } : {}),
				});
				if (active.current && submittedIdentity === currentIdentity.current) {
					if (environment && !result.environments.includes(environment)) {
						setSearchParams((current) => {
							const next = new URLSearchParams(current);
							next.set("env", "dev");
							return next;
						});
					}
					await finish(result.project_name || result.project_id);
				}
			} else {
				const result = await initializeGlobalLocation();
				if (submittedIdentity === currentIdentity.current)
					await finish(result.location.projectName);
			}
		} catch (cause) {
			if (!active.current || submittedIdentity !== currentIdentity.current) return;
			const failure = cause as HttpError;
			if (
				failure.context?.partial_state === "project_created_binding_unsaved" &&
				typeof failure.context.project_id === "string"
			) {
				setMode("existing");
				setProject(failure.context.project_id);
				void projects.mutate().catch(() => undefined);
			}
			setConflict(failure.code === "SERVE_MANIFEST_CONFLICT");
			setError(failure.code === "SERVE_MANIFEST_CONFLICT" ? t("binding.conflict") : message(cause));
		} finally {
			pending.current = false;
			if (active.current) setBusy(false);
		}
	}
	return (
		<DialogContent
			showCloseButton={!busy}
			onEscapeKeyDown={(e) => {
				if (busy) e.preventDefault();
			}}
			onPointerDownOutside={(e) => {
				if (busy) e.preventDefault();
			}}
		>
			<DialogHeader>
				<DialogTitle>{t(workspace ? "binding.workspaceTitle" : "binding.globalTitle")}</DialogTitle>
				<DialogDescription>
					{t(workspace ? "binding.workspaceHint" : "binding.globalHint")}
				</DialogDescription>
			</DialogHeader>
			{session.error ? (
				<ErrorNotice>{message(session.error)}</ErrorNotice>
			) : session.isLoading ? (
				<p role="status">{t("session.loading")}</p>
			) : !account?.loggedIn ? (
				<div className="space-y-3">
					<p>{t("global.loginRequired")}</p>
					<Button asChild>
						<Link to="/settings">{t("session.login")}</Link>
					</Button>
				</div>
			) : (
				<>
					<dl className="grid gap-3 rounded-md border bg-muted/40 p-4 text-sm ud-sm:grid-cols-2">
						<div>
							<dt className="text-muted-foreground">{t("session.account")}</dt>
							<dd className="break-all">{account.email || account.userId}</dd>
						</div>
						<div>
							<dt className="text-muted-foreground">{t("session.organization")}</dt>
							<dd className="break-all">
								{account.organizationId || t("binding.organizationRequired")}
							</dd>
						</div>
						<div className="ud-sm:col-span-2">
							<dt className="text-muted-foreground">{t("session.site")}</dt>
							<dd className="break-all">{account.siteUrl}</dd>
						</div>
					</dl>
					{initialId && (
						<p className="break-all text-sm">{t("binding.current", { name: currentName })}</p>
					)}
					<div className="space-y-2">
						<Label>{t("binding.method")}</Label>
						<Select
							value={mode}
							disabled={busy || readOnly || !!initialId}
							onValueChange={(v) => {
								setMode(v);
								setError("");
							}}
						>
							<SelectTrigger aria-label={t("binding.method")}>
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="automatic">{t("binding.automatic")}</SelectItem>
								<SelectItem value="existing">{t("binding.existing")}</SelectItem>
							</SelectContent>
						</Select>
					</div>
					{isExisting && !workspace ? (
						<LocationPicker
							initial={initial}
							onSaved={async (location) => {
								await finish(location?.projectName || t("global.title"));
							}}
							onCancel={() => onOpenChange(false)}
							onBusyChange={setBusy}
						/>
					) : (
						<>
							{isExisting ? (
								<div className="space-y-2">
									<Label>{t("global.project")}</Label>
									<Select
										value={project}
										onValueChange={setProject}
										disabled={busy || readOnly || projects.isLoading}
									>
										<SelectTrigger aria-label={t("global.project")}>
											<SelectValue placeholder={t("global.selectProject")} />
										</SelectTrigger>
										<SelectContent>
											{projects.data?.map((p) => (
												<SelectItem key={p.id} value={p.id}>
													{p.name}
												</SelectItem>
											))}
										</SelectContent>
									</Select>
									{detail.data && !detail.data.environments.some((e) => e.slug === "dev") && (
										<ErrorNotice>{t("binding.devRequired")}</ErrorNotice>
									)}
									{projects.data?.length === 0 && (
										<p className="text-sm text-muted-foreground">{t("binding.noProjects")}</p>
									)}
								</div>
							) : null}
							<div className="space-y-1 rounded-md border p-4 text-sm">
								<p className="break-all font-medium">
									{t("binding.target", {
										name: isExisting
											? detail.data?.name || project
											: targetName || t("session.loading"),
									})}
								</p>
								<p>{t("binding.defaultEnvironment")}</p>
								<p className="text-muted-foreground">
									{t(workspace ? "binding.workspaceStorage" : "binding.globalStorage")}
								</p>
							</div>
							{settings.error ||
							overview.error ||
							(isExisting && (projects.error || detail.error)) ? (
								<ErrorNotice>
									{message(settings.error || overview.error || projects.error || detail.error)}
								</ErrorNotice>
							) : null}
							{draft && (
								<div className="space-y-3 rounded-md border p-3">
									<p className="text-sm">{t("binding.pendingDraft")}</p>
									<div className="flex flex-wrap gap-2">
										{workspaceEntryId && <ManifestSaveControl entryId={workspaceEntryId} />}
										<Button
											variant="outline"
											onClick={() =>
												useManifestDraftStore.getState().clearWorkspace(workspaceEntryId)
											}
										>
											{t("manifestDraft.discard")}
										</Button>
									</div>
								</div>
							)}
							{error && (
								<ErrorNotice
									action={
										conflict ? (
											<Button
												variant="outline"
												onClick={async () => {
													await Promise.allSettled([settings.mutate(), overview.mutate()]);
													setConflict(false);
													setError("");
												}}
											>
												{t("secrets.retry")}
											</Button>
										) : undefined
									}
								>
									{error}
								</ErrorNotice>
							)}
							<DialogFooter>
								<Button variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
									{t("session.cancel")}
								</Button>
								<Button disabled={!canBind} onClick={() => void bind()}>
									{busy && <Spinner aria-hidden="true" />}
									{t(
										busy
											? "binding.binding"
											: isExisting
												? "binding.bindExisting"
												: workspace
													? "binding.createAndBind"
													: "binding.prepareAndBind",
									)}
								</Button>
							</DialogFooter>
						</>
					)}
				</>
			)}
		</DialogContent>
	);
}
