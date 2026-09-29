import { FilePenLine, FileDiff, Save, Trash2, RefreshCw } from "lucide-react";
import type React from "react";
import { useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useSWRConfig } from "swr";
import { applyManifestDraft, previewManifestDraft } from "@/api/manifest";
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
import { ErrorNotice } from "@/components/ui/page-layout";
import { Spinner } from "@/components/ui/spinner";
import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import {
	sideBySideDiffRows,
	type UnifiedDiffLine,
	unifiedFileDiff,
} from "@/features/manifest-draft/unified-diff";
import { useToast } from "@/hooks/useToast";
import type { ApplyManifestRequest, HttpError, PreviewManifestResponse } from "@/types/api";
export const ManifestSaveControl: React.FC<{ entryId: string }> = ({ entryId }) => {
	const { t } = useTranslation();
	const { mutate } = useSWRConfig();
	const toast = useToast();
	const draft = useManifestDraftStore((state) => state.drafts[manifestDraftKey(entryId)]);
	const clearWorkspace = useManifestDraftStore((state) => state.clearWorkspace);
	const [open, setOpen] = useState(false);
	const [saving, setSaving] = useState(false);
	const [previewing, setPreviewing] = useState(false);
	const [preview, setPreview] = useState<PreviewManifestResponse>();
	const [error, setError] = useState("");
	const diffViewport = useRef<HTMLDivElement>(null);
	const diffLines = useMemo(
		() => (preview ? unifiedFileDiff(preview.before, preview.after) : []),
		[preview],
	);
	const diffRows = useMemo(() => sideBySideDiffRows(diffLines), [diffLines]);

	if (!draft) return null;
	const changedCount = draft.summaries.filter((summary) => summary.changed).length;

	async function showPreview() {
		if (!draft || previewing) return;
		setOpen(true);
		setPreview(undefined);
		setPreviewing(true);
		setError("");
		try {
			const result = await previewManifestDraft(
				{
					revision: draft.revision,
					workspace: draft.workspace,
					changes: Object.values(draft.changes),
				},
				entryId,
			);
			setPreview(result);
		} catch (cause) {
			const failure = cause as HttpError;
			setError(
				failure.code === "SERVE_MANIFEST_CONFLICT"
					? t("manifestDraft.conflict")
					: failure.message || t("manifestDraft.previewFailed"),
			);
		} finally {
			setPreviewing(false);
		}
	}

	async function save() {
		if (!draft || saving) return;
		setSaving(true);
		setError("");
		try {
			const payload: ApplyManifestRequest = {
				revision: draft.revision,
				workspace: draft.workspace,
				changes: Object.values(draft.changes),
			};
			await applyManifestDraft(payload, entryId);
			clearWorkspace(entryId);
			setOpen(false);
			await mutate(
				(key) => typeof key === "string" && key.startsWith(`/workspaces/${entryId}/`),
				undefined,
				{ revalidate: true },
			);
			toast.success(t("manifestDraft.saved"));
		} catch (cause) {
			const failure = cause as HttpError;
			setError(
				failure.code === "SERVE_MANIFEST_CONFLICT"
					? t("manifestDraft.conflict")
					: failure.message || t("manifestDraft.saveFailed"),
			);
		} finally {
			setSaving(false);
		}
	}

	return (
		<>
			<Button variant="default" onClick={() => void showPreview()}>
				<FilePenLine className="h-4 w-4" />
				{t("manifestDraft.saveButton", { count: changedCount })}
			</Button>

			<AlertDialog open={open} onOpenChange={(next) => !saving && setOpen(next)}>
				<AlertDialogContent size="wide" className="max-h-[92dvh] flex flex-col">
					<AlertDialogHeader>
						<div className="flex items-start gap-3">
							<div className="mt-0.5 grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-accent text-primary-text">
								<FileDiff className="h-4 w-4" />
							</div>
							<div>
								<AlertDialogTitle>{t("manifestDraft.title")}</AlertDialogTitle>
								<AlertDialogDescription className="mt-1">
									{t("manifestDraft.description")}
								</AlertDialogDescription>
							</div>
						</div>
					</AlertDialogHeader>

					<div
						className="flex flex-wrap items-center gap-2"
						aria-label={t("manifestDraft.changedFields")}
					>
						{preview && (
							<Button
								variant="outline"
								size="sm"
								onClick={() =>
									diffViewport.current
										?.querySelector('[data-changed="true"]')
										?.scrollIntoView({ block: "center" })
								}
							>
								{t("manifestDraft.jumpToChange")}
							</Button>
						)}
						{draft.summaries
							.filter((item) => item.changed)
							.map((item) => (
								<span
									key={item.id}
									className="rounded-md bg-accent px-2 py-1 text-xs text-primary-text"
								>
									{t(item.labelKey)}
								</span>
							))}
					</div>
					<div
						ref={diffViewport}
						className="min-h-0 flex-1 overflow-auto rounded-lg border border-border bg-background font-mono text-xs leading-5"
					>
						{previewing ? (
							<div className="flex min-h-40 items-center justify-center gap-2 text-muted-foreground">
								<Spinner />
								<span>{t("manifestDraft.previewing")}</span>
							</div>
						) : preview ? (
							<div className="min-w-[48rem]">
								<div className="sticky top-0 z-10 grid grid-cols-2 border-b border-border bg-background md:grid-cols-2">
									<div className="border-b border-border bg-error-surface px-3 py-2 text-error-foreground md:border-r md:border-b-0">
										<span className="font-sans text-xs font-semibold">
											{t("manifestDraft.currentManifest")}
										</span>
										<span className="ml-2 text-muted-foreground">a/one.manifest.toml</span>
									</div>
									<div className="bg-success-surface px-3 py-2 text-success-foreground">
										<span className="font-sans text-xs font-semibold">
											{t("manifestDraft.updatedManifest")}
										</span>
										<span className="ml-2 text-muted-foreground">b/one.manifest.toml</span>
									</div>
								</div>
								<div className="py-1.5">
									{diffRows.map((row, index) => (
										<div
											key={`${row.before?.beforeLine ?? ""}:${row.after?.afterLine ?? ""}:${index}`}
											data-changed={row.before?.kind === "removed" || row.after?.kind === "added"}
											className="grid grid-cols-2"
										>
											<ManifestDiffCell line={row.before} side="before" />
											<ManifestDiffCell line={row.after} side="after" />
										</div>
									))}
								</div>
							</div>
						) : null}
					</div>

					{error ? (
						<ErrorNotice
							action={
								!preview ? (
									<Button
										variant="outline"
										size="sm"
										disabled={previewing}
										onClick={() => void showPreview()}
									>
										<RefreshCw />
										{t("secrets.retry")}
									</Button>
								) : undefined
							}
						>
							{error}
						</ErrorNotice>
					) : null}

					<AlertDialogFooter className="sm:justify-between">
						<Button
							variant="ghost"
							className="text-muted-foreground"
							disabled={saving}
							onClick={() => {
								clearWorkspace(entryId);
								setOpen(false);
							}}
						>
							<Trash2 />
							{t("manifestDraft.discard")}
						</Button>
						<div className="flex gap-2">
							<AlertDialogCancel disabled={saving}>{t("form.cancel")}</AlertDialogCancel>
							<Button onClick={() => void save()} disabled={saving || previewing || !preview}>
								{saving ? <Spinner /> : <Save />}
								{saving ? t("manifestDraft.saving") : t("manifestDraft.confirm")}
							</Button>
						</div>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</>
	);
};

const ManifestDiffCell: React.FC<{
	line?: UnifiedDiffLine;
	side: "before" | "after";
}> = ({ line, side }) => {
	const lineNumber = side === "before" ? line?.beforeLine : line?.afterLine;
	const toneClass = !line
		? "bg-muted/30"
		: line.kind === "removed"
			? "bg-error-surface text-error-foreground"
			: line.kind === "added"
				? "bg-success-surface text-success-foreground"
				: "text-foreground";
	const dividerClass = side === "before" ? "border-b border-border md:border-r md:border-b-0" : "";

	return (
		<div
			aria-hidden={!line || undefined}
			className={`grid min-h-5 grid-cols-[3rem_1.5rem_minmax(0,1fr)] ${toneClass} ${dividerClass}`}
		>
			<span className="select-none border-r border-border px-2 text-right text-muted-foreground">
				{lineNumber}
			</span>
			<span className="select-none text-center">
				{line?.kind === "removed" ? "-" : line?.kind === "added" ? "+" : " "}
			</span>
			<span className="whitespace-pre px-2">{line?.text}</span>
		</div>
	);
};
