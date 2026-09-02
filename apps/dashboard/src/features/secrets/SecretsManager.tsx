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
import { useEffect, useState } from "react";
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
import { initializeWorkspaceEnvironmentBackend } from "@/api/workspace";
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
	const [revealed, setRevealed] = useState<Record<string, string>>({});
	const [loadingKey, setLoadingKey] = useState("");
	const [editor, setEditor] = useState<SecretEditorState | null>(null);
	const [deleteKey, setDeleteKey] = useState("");
	const [deleteConfirmation, setDeleteConfirmation] = useState("");
	const [saving, setSaving] = useState(false);
	const [retrying, setRetrying] = useState(false);
	const [recoveryError, setRecoveryError] = useState("");
	const key = secretsKey(workspaceEntryId, environment, project || undefined);
	const result = useSWR(
		key,
		() => listSecrets(workspaceEntryId, environment, project || undefined),
		{ revalidateIfStale: false },
	);
	const listError = result.error as HttpError | undefined;
	const showLoading = result.isLoading && !result.data;
	const showError = !showLoading && !result.data && Boolean(listError);
	const showEmpty = !showLoading && !showError && result.data?.keys.length === 0;

	useEffect(() => {
		setRevealed({});
		setEditor(null);
		setDeleteKey("");
		setDeleteConfirmation("");
		setRecoveryError("");
	}, [environment, project]);

	async function retryList() {
		if (retrying) return;
		setRetrying(true);
		setRecoveryError("");
		try {
			if ((result.error as HttpError | undefined)?.code === "INFISICAL_NOT_CONFIGURED") {
				await initializeWorkspaceEnvironmentBackend(
					workspaceEntryId,
					environment,
					project || undefined,
				);
			}
			await result.mutate();
		} catch (error) {
			const failure = error as HttpError;
			setRecoveryError(failure.message || String(error));
		} finally {
			setRetrying(false);
		}
	}

	async function toggleReveal(secretKey: string) {
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
			setRevealed((current) => ({ ...current, [secretKey]: secret.value }));
		} catch (error) {
			showSecretError(toast, t("secrets.revealFailed"), error);
		} finally {
			setLoadingKey("");
		}
	}

	async function editSecret(secretKey: string) {
		setLoadingKey(secretKey);
		try {
			const secret = await revealSecret(
				workspaceEntryId,
				environment,
				project || undefined,
				secretKey,
			);
			setEditor({ mode: "edit", key: secretKey, value: secret.value });
		} catch (error) {
			showSecretError(toast, t("secrets.revealFailed"), error);
		} finally {
			setLoadingKey("");
		}
	}

	async function saveEditor() {
		if (!editor || !editor.key.trim() || saving) return;
		setSaving(true);
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
			showSecretError(toast, t("secrets.saveFailed"), error);
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
				"overflow-hidden rounded-xl border-border shadow-sm",
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
							onClick={() => setEditor({ mode: "create", key: "", value: "" })}
							disabled={readOnly}
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
					<Empty className="min-h-36">
						<EmptyHeader>
							<EmptyDescription>
								{recoveryError || listError.message}
							</EmptyDescription>
						</EmptyHeader>
						<Button
							variant="outline"
							size="sm"
							disabled={retrying}
							onClick={() => void retryList()}
						>
							{retrying ? <Spinner /> : <RefreshCw />}
							{t("secrets.retry")}
						</Button>
					</Empty>
				) : null}
				{showEmpty ? (
					<Empty className="min-h-36">
						<EmptyHeader>
							<EmptyDescription>{t("secrets.empty")}</EmptyDescription>
						</EmptyHeader>
						<Button
							size="sm"
							onClick={() => setEditor({ mode: "create", key: "", value: "" })}
							disabled={readOnly}
						>
							<Plus />
							{t("secrets.add")}
						</Button>
					</Empty>
				) : null}
				{!showLoading && !showError && result.data && result.data.keys.length > 0 ? (
					<Table className="table-fixed">
						<TableHeader>
							<TableRow>
								<TableHead>{t("secrets.key")}</TableHead>
								<TableHead className="w-[60%]">{t("secrets.value")}</TableHead>
								<TableHead className="w-44 text-right">{t("secrets.actions")}</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{result.data.keys.map((secretKey) => {
								const value = revealed[secretKey];
								const busy = loadingKey === secretKey;
								return (
									<TableRow key={secretKey}>
										<TableCell className="font-mono text-xs font-semibold">{secretKey}</TableCell>
										<TableCell className="min-w-0">
											<code className="block truncate rounded bg-muted/60 px-2 py-1 text-[11px]">
												{value === undefined ? "••••••••••••" : value || t("secrets.emptyValue")}
											</code>
										</TableCell>
										<TableCell>
											<div className="flex justify-end gap-1">
												<Button
													variant="ghost"
													size="icon-sm"
													aria-label={value === undefined ? t("secrets.reveal") : t("secrets.hide")}
													onClick={() => void toggleReveal(secretKey)}
													disabled={busy}
												>
													{busy ? <Spinner /> : value === undefined ? <Eye /> : <EyeOff />}
												</Button>
												<Button
													variant="ghost"
													size="icon-sm"
													aria-label={t("secrets.copy")}
													disabled={value === undefined}
													onClick={() => void navigator.clipboard.writeText(value ?? "")}
												>
													<Copy />
												</Button>
												<Button
													variant="ghost"
													size="icon-sm"
													aria-label={t("secrets.edit")}
													onClick={() => void editSecret(secretKey)}
													disabled={readOnly || busy}
												>
													<Pencil />
												</Button>
												<Button
													variant="ghost"
													size="icon-sm"
													className="text-error-foreground"
													aria-label={t("secrets.delete")}
													onClick={() => {
														setDeleteConfirmation("");
														setDeleteKey(secretKey);
													}}
													disabled={readOnly}
												>
													<Trash2 />
												</Button>
											</div>
										</TableCell>
									</TableRow>
								);
							})}
						</TableBody>
					</Table>
				) : null}
			</CardContent>

			<SecretEditor
				editor={editor}
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
				<AlertDialogContent>
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
	onChange(editor: SecretEditorState): void;
	onSave(): void;
	onClose(): void;
}> = ({ editor, saving, onChange, onSave, onClose }) => {
	const { t } = useTranslation();
	const [showValue, setShowValue] = useState(false);
	useEffect(() => setShowValue(false), [editor?.key, editor?.mode]);
	return (
		<Dialog open={Boolean(editor)} onOpenChange={(open) => !open && onClose()}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>
						{t(editor?.mode === "edit" ? "secrets.editTitle" : "secrets.createTitle")}
					</DialogTitle>
					<DialogDescription>{t("secrets.editorDescription")}</DialogDescription>
				</DialogHeader>
				{editor ? (
					<div className="space-y-4">
						<Field>
							<FieldLabel htmlFor="secret-key">{t("secrets.key")}</FieldLabel>
							<Input
								id="secret-key"
								className="font-mono uppercase"
								value={editor.key}
								onChange={(event) => onChange({ ...editor, key: event.target.value.toUpperCase() })}
								disabled={editor.mode === "edit" || saving}
								autoComplete="off"
							/>
						</Field>
						<Field>
							<div className="flex items-center justify-between">
								<FieldLabel htmlFor="secret-value">{t("secrets.value")}</FieldLabel>
								<Button variant="ghost" size="xs" onClick={() => setShowValue((value) => !value)}>
									{showValue ? <EyeOff /> : <Eye />}
									{t(showValue ? "secrets.hide" : "secrets.reveal")}
								</Button>
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
					</div>
				) : null}
				<DialogFooter>
					<Button variant="outline" onClick={onClose} disabled={saving}>
						{t("form.cancel")}
					</Button>
					<Button onClick={onSave} disabled={saving || !editor?.key.trim()}>
						{saving ? <Spinner /> : <ShieldCheck />}
						{t("secrets.save")}
					</Button>
				</DialogFooter>
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
