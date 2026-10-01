import { MoreHorizontal, SearchX } from "lucide-react";
import { IconButton } from "@/components/ui/icon-button";
import { ErrorNotice, SearchInput, StatePanel } from "@/components/ui/page-layout";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { getSession, sessionKey } from "@/api/session";
import {
	Copy,
	Eye,
	EyeOff,
	KeyRound,
	Pencil,
	Plus,
	RefreshCw,
	ShieldCheck,
	Trash2,
} from "lucide-react";
import type React from "react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import {
	createSecret,
	deleteSecret,
	listSecrets,
	revealSecret,
	secretsKey,
	updateSecret,
} from "@/api/secrets";
import { InfisicalBindingDialog } from "@/features/infisical-binding/InfisicalBindingDialog";
import {
	AlertDialog,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Empty, EmptyDescription, EmptyHeader } from "@/components/ui/empty";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { useToast } from "@/hooks/useToast";
import { cn } from "@/lib/utils";
import type { HttpError, OverviewProject } from "@/types/api";

interface SecretEditorState {
	mode: "create" | "edit";
	key: string;
	value: string;
	originalValue?: string;
}

const WORKSPACE_SECRET_SCOPE = "__workspace_secret_scope__";

export const SecretsManager: React.FC<{
	workspaceEntryId?: string;
	environment: string;
	projects?: OverviewProject[];
	fixedProject?: string;
	variant?: "standalone" | "embedded";
	readOnly?: boolean;
}> = ({
	workspaceEntryId,
	environment,
	projects = [],
	fixedProject,
	variant = "standalone",
	readOnly,
}) => {
	const { t } = useTranslation();
	const toast = useToast();
	const [selectedProject, setSelectedProject] = useState("");
	const project = fixedProject ?? selectedProject;
	const session = useSWR(sessionKey, getSession, { refreshInterval: 2000 });
	const requestEpoch = useRef(0);
	const actionTrigger = useRef<HTMLButtonElement | null>(null);
	const [revealed, setRevealed] = useState<Record<string, string>>({});
	const [loadingKey, setLoadingKey] = useState("");
	const [editor, setEditor] = useState<SecretEditorState | null>(null);
	const [deleteKey, setDeleteKey] = useState("");
	const [deleteConfirmation, setDeleteConfirmation] = useState("");
	const [saving, setSaving] = useState(false);
	const [bindingOpen, setBindingOpen] = useState(false);
	const [retrying, setRetrying] = useState(false);
	const [recoveryError, setRecoveryError] = useState("");
	const [search, setSearch] = useState("");
	const [editorError, setEditorError] = useState("");
	const key = secretsKey(workspaceEntryId, environment, project || undefined);
	const result = useSWR(
		key,
		() => listSecrets(workspaceEntryId, environment, project || undefined),
		{ revalidateIfStale: false },
	);
	const listError = result.error as HttpError | undefined;
	const needsInitialization = listError?.code === "INFISICAL_NOT_CONFIGURED";
	const showLoading = result.isLoading && !result.data;
	const showError = !showLoading && !result.data && Boolean(listError);
	const showEmpty = !showLoading && !showError && result.data?.keys.length === 0;

	useEffect(() => {
		requestEpoch.current++;
		setRevealed({});
		setEditor(null);
		setBindingOpen(false);
		setDeleteKey("");
		setDeleteConfirmation("");
		setRecoveryError("");
		setSearch("");
		setEditorError("");
		return () => {
			requestEpoch.current++;
		};
	}, [
		environment,
		project,
		workspaceEntryId,
		session.data?.session.userId,
		session.data?.session.loggedIn,
	]);

	async function retryList() {
		if (retrying) return;
		setRetrying(true);
		setRecoveryError("");
		try {
			await result.mutate();
		} catch (error) {
			const failure = error as HttpError;
			setRecoveryError(failure.message || String(error));
		} finally {
			setRetrying(false);
		}
	}

	async function toggleReveal(secretKey: string) {
		const epoch = requestEpoch.current;
		if (revealed[secretKey] !== undefined) {
			setRevealed((current) => {
				const next = { ...current };
				delete next[secretKey];
				return next;
			});
			return;
		}
		setLoadingKey(secretKey);
		try {
			const secret = await revealSecret(
				workspaceEntryId,
				environment,
				project || undefined,
				secretKey,
			);
			if (epoch === requestEpoch.current)
				setRevealed((current) => ({ ...current, [secretKey]: secret.value }));
		} catch (error) {
			showSecretError(toast, t("secrets.revealFailed"), error);
		} finally {
			setLoadingKey("");
		}
	}

	async function editSecret(secretKey: string) {
		const epoch = requestEpoch.current;
		setLoadingKey(secretKey);
		try {
			const secret = await revealSecret(
				workspaceEntryId,
				environment,
				project || undefined,
				secretKey,
			);
			if (epoch === requestEpoch.current) {
				setEditorError("");
				setEditor({
					mode: "edit",
					key: secretKey,
					value: secret.value,
					originalValue: secret.value,
				});
			}
		} catch (error) {
			showSecretError(toast, t("secrets.revealFailed"), error);
		} finally {
			setLoadingKey("");
		}
	}

	async function saveEditor() {
		if (!editor || !editor.key.trim() || saving || readOnly) return;
		if (needsInitialization) {
			setEditorError(t("secrets.notConfiguredHint"));
			return;
		}
		if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(editor.key.trim())) {
			setEditorError(t("secrets.invalidKey"));
			return;
		}
		setSaving(true);
		setEditorError("");
		try {
			if (editor.mode === "create") {
				await createSecret(
					workspaceEntryId,
					environment,
					project || undefined,
					editor.key.trim(),
					editor.value,
				);
			} else {
				await updateSecret(
					workspaceEntryId,
					environment,
					project || undefined,
					editor.key,
					editor.value,
				);
			}
			setRevealed((current) => {
				const next = { ...current };
				delete next[editor.key];
				return next;
			});
			setEditor(null);
			await result.mutate();
			toast.success(t(editor.mode === "create" ? "secrets.created" : "secrets.updated"));
		} catch (error) {
			setEditorError((error as HttpError).message || t("secrets.saveFailed"));
		} finally {
			setSaving(false);
		}
	}

	async function confirmDelete() {
		if (!deleteKey || deleteConfirmation !== deleteKey || saving) return;
		setSaving(true);
		try {
			await deleteSecret(workspaceEntryId, environment, project || undefined, deleteKey);
			setRevealed((current) => {
				const next = { ...current };
				delete next[deleteKey];
				return next;
			});
			setDeleteKey("");
			setDeleteConfirmation("");
			await result.mutate();
			toast.success(t("secrets.deleted"));
		} catch (error) {
			showSecretError(toast, t("secrets.deleteFailed"), error);
		} finally {
			setSaving(false);
		}
	}

	return (
		<Card
			className={cn(
				"overflow-hidden rounded-lg border-border shadow-none",
				variant === "embedded" && "rounded-lg",
			)}
		>
			<CardHeader className="border-b border-border px-4 py-3.5">
				<div className="flex flex-col items-start justify-between gap-3 sm:flex-row sm:items-center">
					<div className="flex items-center gap-3">
						<div className="grid size-9 place-items-center rounded-lg bg-primary/8 text-primary">
							<KeyRound className="h-4 w-4" />
						</div>
						<div>
							<CardTitle className="text-base">{t("secrets.title")}</CardTitle>
							<p className="mt-1 text-xs text-muted-foreground">{t("secrets.scopeHint")}</p>
						</div>
					</div>
					<div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
						{fixedProject === undefined ? (
							<Select
								value={selectedProject || WORKSPACE_SECRET_SCOPE}
								onValueChange={(scope) =>
									setSelectedProject(scope === WORKSPACE_SECRET_SCOPE ? "" : scope)
								}
							>
								<SelectTrigger aria-label={t("secrets.scope")} className="w-full sm:w-52">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value={WORKSPACE_SECRET_SCOPE}>
										{t("secrets.workspaceScope")}
									</SelectItem>
									{projects.map((item) => (
										<SelectItem key={item.name} value={item.name}>
											{item.name}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						) : null}
						<Button
							size="sm"
							onClick={() => {
								setEditorError("");
								actionTrigger.current = null;
								setEditor({ mode: "create", key: "", value: "" });
							}}
							disabled={readOnly || showError || showLoading}
						>
							<Plus />
							{t("secrets.add")}
						</Button>
					</div>
				</div>
			</CardHeader>
			<CardContent className="p-0">
				{showLoading ? <SecretListLoading /> : null}
				{showError && listError ? (
					<StatePanel
						icon={KeyRound}
						title={t(
							listError.code === "INFISICAL_NOT_CONFIGURED"
								? "secrets.notConfiguredTitle"
								: "secrets.loadError",
						)}
						description={
							listError.code === "INFISICAL_NOT_CONFIGURED"
								? t("secrets.notConfiguredHint")
								: recoveryError || listError.message
						}
					>
						{recoveryError && listError.code === "INFISICAL_NOT_CONFIGURED" ? (
							<ErrorNotice>{recoveryError}</ErrorNotice>
						) : null}
						{needsInitialization ? (
							<Button disabled={readOnly} onClick={() => setBindingOpen(true)}>
								<KeyRound />
								{t("binding.workspaceTitle")}
							</Button>
						) : null}
						<Button variant="outline" disabled={retrying} onClick={() => void retryList()}>
							{retrying ? <Spinner /> : <RefreshCw />}
							{t("secrets.retry")}
						</Button>
					</StatePanel>
				) : null}
				{showEmpty ? (
					<Empty className="min-h-36">
						<EmptyHeader>
							<EmptyDescription>{t("secrets.empty")}</EmptyDescription>
						</EmptyHeader>
						<Button
							size="sm"
							onClick={() => {
								setEditorError("");
								actionTrigger.current = null;
								setEditor({ mode: "create", key: "", value: "" });
							}}
							disabled={readOnly || showError || showLoading}
						>
							<Plus />
							{t("secrets.add")}
						</Button>
					</Empty>
				) : null}
				{!showLoading && !showError && result.data && result.data.keys.length > 0 ? (
					<>
						<div className="p-4">
							<SearchInput
								value={search}
								onChange={(e) => setSearch(e.target.value)}
								aria-label={t("secrets.search")}
								placeholder={t("secrets.search")}
							/>
						</div>
						<Table className="min-w-[28rem]">
							<TableHeader>
								<TableRow>
									<TableHead>{t("secrets.key")}</TableHead>
									<TableHead className="w-1/3">{t("secrets.value")}</TableHead>
									<TableHead className="w-28 text-right">{t("secrets.actions")}</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{result.data.keys
									.filter((secretKey) =>
										secretKey.toLowerCase().includes(search.trim().toLowerCase()),
									)
									.map((secretKey) => {
										const value = revealed[secretKey];
										const busy = loadingKey === secretKey;
										return (
											<TableRow key={secretKey}>
												<TableCell className="font-mono text-xs font-semibold">
													{secretKey}
												</TableCell>
												<TableCell className="min-w-0">
													<code className="block truncate rounded bg-muted/60 px-2 py-1 text-xs">
														{value === undefined
															? "••••••••••••"
															: value || t("secrets.emptyValue")}
													</code>
												</TableCell>
												<TableCell>
													<div className="flex justify-end gap-1">
														<IconButton
															label={value === undefined ? t("secrets.reveal") : t("secrets.hide")}
															onClick={() => void toggleReveal(secretKey)}
															disabled={busy}
														>
															{busy ? <Spinner /> : value === undefined ? <Eye /> : <EyeOff />}
														</IconButton>
														<IconButton
															label={t("secrets.copy")}
															disabled={value === undefined}
															onClick={() => {
																void navigator.clipboard
																	.writeText(value ?? "")
																	.then(() => toast.success(t("global.copied")))
																	.catch((error) =>
																		showSecretError(toast, t("secrets.revealFailed"), error),
																	);
															}}
														>
															<Copy />
														</IconButton>
														<DropdownMenu>
															<DropdownMenuTrigger asChild>
																<Button
																	variant="ghost"
																	size="icon"
																	aria-label={t("secrets.more", { key: secretKey })}
																	onPointerDown={(event) => {
																		actionTrigger.current = event.currentTarget;
																	}}
																	onFocus={(event) => {
																		actionTrigger.current = event.currentTarget;
																	}}
																	disabled={readOnly || busy}
																>
																	<MoreHorizontal />
																</Button>
															</DropdownMenuTrigger>
															<DropdownMenuContent align="end">
																<DropdownMenuItem onSelect={() => void editSecret(secretKey)}>
																	<Pencil />
																	{t("secrets.edit")}
																</DropdownMenuItem>
																<DropdownMenuSeparator />
																<DropdownMenuItem
																	variant="destructive"
																	onSelect={() => {
																		setDeleteConfirmation("");
																		setDeleteKey(secretKey);
																	}}
																>
																	<Trash2 />
																	{t("secrets.delete")}
																</DropdownMenuItem>
															</DropdownMenuContent>
														</DropdownMenu>
													</div>
												</TableCell>
											</TableRow>
										);
									})}
							</TableBody>
						</Table>
						{result.data.keys.every(
							(secretKey) => !secretKey.toLowerCase().includes(search.trim().toLowerCase()),
						) ? (
							<StatePanel icon={SearchX} title={t("secrets.noMatches")}>
								<Button variant="outline" onClick={() => setSearch("")}>
									{t("workspaces.home.clearSearch")}
								</Button>
							</StatePanel>
						) : null}
					</>
				) : null}
			</CardContent>

			<InfisicalBindingDialog
				open={bindingOpen}
				onOpenChange={setBindingOpen}
				scope="workspace"
				workspaceEntryId={workspaceEntryId}
				environment={environment}
				readOnly={readOnly}
			/>
			<SecretEditor
				editor={editor}
				error={editorError}
				returnFocus={actionTrigger.current}
				saving={saving}
				onChange={setEditor}
				onSave={() => void saveEditor()}
				onClose={() => !saving && setEditor(null)}
			/>
			<AlertDialog
				open={Boolean(deleteKey)}
				onOpenChange={(open) => {
					if (!open && !saving) {
						setDeleteKey("");
						setDeleteConfirmation("");
					}
				}}
			>
				<AlertDialogContent
					onCloseAutoFocus={(event) => {
						if (actionTrigger.current?.isConnected) {
							event.preventDefault();
							actionTrigger.current.focus();
							actionTrigger.current = null;
						}
					}}
				>
					<AlertDialogHeader>
						<AlertDialogTitle>{t("secrets.deleteTitle", { key: deleteKey })}</AlertDialogTitle>
						<AlertDialogDescription>{t("secrets.deleteDescription")}</AlertDialogDescription>
					</AlertDialogHeader>
					<Field>
						<FieldLabel htmlFor="secret-delete-confirmation">
							{t("secrets.deleteConfirmation", { key: deleteKey })}
						</FieldLabel>
						<Input
							id="secret-delete-confirmation"
							value={deleteConfirmation}
							onChange={(event) => setDeleteConfirmation(event.target.value)}
							autoComplete="off"
						/>
					</Field>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={saving}>{t("form.cancel")}</AlertDialogCancel>
						<Button
							variant="destructive"
							disabled={saving || deleteConfirmation !== deleteKey}
							onClick={() => void confirmDelete()}
						>
							{saving ? <Spinner /> : <Trash2 />}
							{t("secrets.delete")}
						</Button>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</Card>
	);
};

const SecretEditor: React.FC<{
	editor: SecretEditorState | null;
	saving: boolean;
	error: string;
	returnFocus: HTMLButtonElement | null;
	onChange(editor: SecretEditorState): void;
	onSave(): void;
	onClose(): void;
}> = ({ editor, saving, error, returnFocus, onChange, onSave, onClose }) => {
	const { t } = useTranslation();
	const [showValue, setShowValue] = useState(false);
	const [confirmClose, setConfirmClose] = useState(false);
	const isOpen = Boolean(editor);
	useEffect(() => {
		setShowValue(false);
		setConfirmClose(false);
	}, [isOpen]);
	const dirty =
		editor &&
		(editor.mode === "create"
			? Boolean(editor.key || editor.value)
			: editor.value !== editor.originalValue);
	useEffect(() => {
		if (!dirty) return;
		const warn = (event: BeforeUnloadEvent) => event.preventDefault();
		window.addEventListener("beforeunload", warn);
		return () => window.removeEventListener("beforeunload", warn);
	}, [dirty]);
	function close() {
		if (saving) return;
		if (dirty) setConfirmClose(true);
		else onClose();
	}
	return (
		<Dialog
			open={isOpen}
			onOpenChange={(open) => {
				if (!open) close();
			}}
		>
			<DialogContent
				showCloseButton={!saving}
				onCloseAutoFocus={(event) => {
					if (returnFocus?.isConnected) {
						event.preventDefault();
						returnFocus.focus();
					}
				}}
			>
				<DialogHeader>
					<DialogTitle>
						{t(editor?.mode === "edit" ? "secrets.editTitle" : "secrets.createTitle")}
					</DialogTitle>
					<DialogDescription>{t("secrets.editorDescription")}</DialogDescription>
				</DialogHeader>
				<form
					className="space-y-5"
					onSubmit={(event) => {
						event.preventDefault();
						if (!saving && editor?.key.trim() && !confirmClose) onSave();
					}}
				>
					{editor ? (
						<>
							<Field>
								<FieldLabel htmlFor="secret-key">{t("secrets.key")}</FieldLabel>
								<Input
									id="secret-key"
									className="font-mono"
									value={editor.key}
									onChange={(event) =>
										onChange({ ...editor, key: event.target.value.toUpperCase() })
									}
									readOnly={editor.mode === "edit"}
									disabled={saving}
									autoComplete="off"
								/>
							</Field>
							<Field>
								<div className="flex items-center justify-between">
									<FieldLabel htmlFor="secret-value">{t("secrets.value")}</FieldLabel>
									<IconButton
										label={t(showValue ? "secrets.hide" : "secrets.reveal")}
										onClick={() => setShowValue(!showValue)}
									>
										{showValue ? <EyeOff /> : <Eye />}
									</IconButton>
								</div>
								<Input
									id="secret-value"
									type={showValue ? "text" : "password"}
									className="font-mono"
									value={editor.value}
									onChange={(event) => onChange({ ...editor, value: event.target.value })}
									disabled={saving}
									autoComplete="new-password"
								/>
							</Field>
						</>
					) : null}
					{error && <ErrorNotice>{error}</ErrorNotice>}
					{confirmClose ? (
						<>
							<p
								role="status"
								className="rounded-md border border-warning-border bg-warning-surface p-3 text-sm text-warning-foreground"
							>
								{t("form.discardDescription")}
							</p>
							<DialogFooter>
								<Button variant="outline" onClick={() => setConfirmClose(false)}>
									{t("form.continueEditing")}
								</Button>
								<Button variant="destructive" onClick={onClose}>
									{t("global.discard")}
								</Button>
							</DialogFooter>
						</>
					) : (
						<DialogFooter>
							<Button variant="outline" onClick={close} disabled={saving}>
								{t("form.cancel")}
							</Button>
							<Button type="submit" disabled={saving || !editor?.key.trim()}>
								{saving ? <Spinner /> : <ShieldCheck />}
								{t("secrets.save")}
							</Button>
						</DialogFooter>
					)}
				</form>
			</DialogContent>
		</Dialog>
	);
};

const SecretListLoading: React.FC = () => (
	<div className="space-y-1.5 p-3" role="status">
		<Skeleton className="h-8 w-full" />
		<Skeleton className="h-8 w-full opacity-70" />
		<Skeleton className="h-8 w-full opacity-40" />
	</div>
);

function showSecretError(toast: ReturnType<typeof useToast>, title: string, error: unknown) {
	const failure = error as HttpError;
	toast.error(title, { description: failure.message || String(error) });
}
