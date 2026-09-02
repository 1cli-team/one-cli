import { AlertTriangle, FilePenLine, Save, Trash2 } from "lucide-react";
import type React from "react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useMatch } from "react-router-dom";
import useSWR, { useSWRConfig } from "swr";
import { humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import { applyManifestDraft, previewManifestDraft } from "@/api/manifest";
import { switchWorkspaceEnvironmentBackend } from "@/api/workspace";
import { getWorkspaces, workspacesKey } from "@/api/workspaces";
import {
	AlertDialog,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
	Breadcrumb,
	BreadcrumbItem,
	BreadcrumbLink,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { environmentFromSearch } from "@/features/environment-context/environment";
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
import { useThemeStore } from "@/lib/stores/theme";
import { SettingsDialog } from "@/router/SettingsDialog";
import type { ApplyManifestRequest, HttpError, PreviewManifestResponse } from "@/types/api";
import type { SectionKey } from "@/types/api";

interface TopBarProps {
	devDataMode?: string;
}

export const TopBar: React.FC<TopBarProps> = () => {
	const { t } = useTranslation();
	const sectionMatch = useMatch("/section/:domain/:backend");
	const profileMatch = useMatch("/profile");
	const settingsSectionMatch = useMatch("/settings/:domain/:backend");
	const settingsMatch = useMatch("/settings");
	const workspaceMatch = useMatch("/workspace/:entryId");
	const { pathname } = useLocation();
	const { mode } = useThemeStore();
	const detailMatch = settingsSectionMatch ?? sectionMatch;
	const logoSrc = mode === "dark" ? "/brand/icon-inverted.svg" : "/brand/icon.svg";

	return (
		<header className="flex h-16 shrink-0 items-center justify-between gap-4 border-b border-border bg-card/95 px-5 shadow-sm">
			<div className="flex min-w-0 items-center gap-4">
				<EnvironmentLink to="/" className="flex shrink-0 items-center gap-2">
					<img src={logoSrc} alt="One CLI" className="size-8" />
					<div className="hidden sm:block">
						<p className="font-heading text-base font-semibold leading-none tracking-tight">One CLI</p>
						<p className="mt-1 font-mono text-[9px] font-medium tracking-[0.12em] text-muted-foreground uppercase">
							{t("sidebar.brand")}
						</p>
					</div>
				</EnvironmentLink>
				<div className="min-w-0 border-l border-border pl-3">
					<Breadcrumb>
						<BreadcrumbList>
							{detailMatch ? (
								<SectionCrumb
									match={detailMatch.params}
									settingsRoute={Boolean(settingsSectionMatch)}
								/>
							) : settingsMatch ? (
								<SettingsCrumb />
							) : profileMatch ? (
								<ProfileCrumb />
							) : workspaceMatch ? (
								<WorkspaceCrumb entryId={workspaceMatch.params.entryId ?? ""} />
							) : (
								<HomeCrumb />
							)}
						</BreadcrumbList>
					</Breadcrumb>
				</div>
			</div>
			<div className="flex items-center gap-2">
				{workspaceMatch ? (
					<WorkspaceHeaderActions entryId={workspaceMatch.params.entryId ?? ""} />
				) : null}
				{pathname === "/" ? <SettingsDialog /> : null}
			</div>
		</header>
	);
};

const WorkspaceHeaderActions: React.FC<{ entryId: string }> = ({ entryId }) => {
	return <ManifestSaveControl entryId={entryId} />;
};

export const ManifestSaveControl: React.FC<{ entryId: string }> = ({ entryId }) => {
	const { t } = useTranslation();
	const { search } = useLocation();
	const { mutate } = useSWRConfig();
	const toast = useToast();
	const draft = useManifestDraftStore((state) => state.drafts[manifestDraftKey(entryId)]);
	const commitWorkspaceSection = useManifestDraftStore((state) => state.commitWorkspaceSection);
	const clearWorkspace = useManifestDraftStore((state) => state.clearWorkspace);
	const [open, setOpen] = useState(false);
	const [saving, setSaving] = useState(false);
	const [previewing, setPreviewing] = useState(false);
	const [preview, setPreview] = useState<PreviewManifestResponse>();
	const [error, setError] = useState("");
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
			let revision = draft.revision;
			const workspaceEnvironment = draft.workspace?.environment;
			if (workspaceEnvironment) {
				const switched = await switchWorkspaceEnvironmentBackend(
					workspaceEnvironment.backend,
					revision,
					entryId,
					environmentFromSearch(search),
				);
				revision = switched.revision;
				// The Backend switch and Project draft are separate revision-checked
				// publications. Rebase any remaining Project changes immediately so
				// a later failure can be retried without replaying the completed switch.
				commitWorkspaceSection(entryId, "environment", revision);
			}
			const changes = Object.values(draft.changes);
			const payload: ApplyManifestRequest = {
				revision,
				changes,
			};
			if (changes.length > 0) await applyManifestDraft(payload, entryId);
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
			<Button
				variant="outline"
				size="sm"
				className="border-warning-border bg-warning-surface text-warning-foreground hover:bg-warning-surface/80"
				onClick={() => void showPreview()}
			>
				<FilePenLine className="h-4 w-4" />
				{t("manifestDraft.saveButton", { count: changedCount })}
			</Button>

			<AlertDialog open={open} onOpenChange={(next) => !saving && setOpen(next)}>
				<AlertDialogContent
					size="wide"
					className="max-h-[92dvh] sm:grid-rows-[auto_minmax(0,1fr)_auto_auto]"
				>
					<AlertDialogHeader>
						<div className="flex items-start gap-3">
							<div className="mt-0.5 grid h-9 w-9 shrink-0 place-items-center bg-warning-surface text-warning-foreground">
								<AlertTriangle className="h-4 w-4" />
							</div>
							<div>
								<AlertDialogTitle>{t("manifestDraft.title")}</AlertDialogTitle>
								<AlertDialogDescription className="mt-1">
									{t("manifestDraft.description")}
								</AlertDialogDescription>
							</div>
						</div>
					</AlertDialogHeader>

					<div className="min-h-0 overflow-auto rounded-lg border border-border bg-background font-mono text-[11px] leading-5">
						{previewing ? (
							<div className="flex min-h-40 items-center justify-center gap-2 text-muted-foreground">
								<Spinner />
								<span>{t("manifestDraft.previewing")}</span>
							</div>
						) : preview ? (
							<div className="min-w-[56rem]">
								<div className="sticky top-0 z-10 grid grid-cols-1 border-b border-border bg-background md:grid-cols-2">
									<div className="border-b border-border bg-error-surface px-3 py-2 text-error-foreground md:border-r md:border-b-0">
										<span className="font-sans text-xs font-semibold">
											{t("manifestDraft.currentManifest")}
										</span>
										<span className="ml-2 text-muted-foreground">a/one.manifest.json</span>
									</div>
									<div className="bg-success-surface px-3 py-2 text-success-foreground">
										<span className="font-sans text-xs font-semibold">
											{t("manifestDraft.updatedManifest")}
										</span>
										<span className="ml-2 text-muted-foreground">b/one.manifest.json</span>
									</div>
								</div>
								<div className="py-1.5">
									{diffRows.map((row, index) => (
										<div
											key={`${row.before?.beforeLine ?? ""}:${row.after?.afterLine ?? ""}:${index}`}
											className="grid grid-cols-1 md:grid-cols-2"
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
						<p
							role="alert"
							className="border border-error-border bg-error-surface px-3 py-2 text-xs text-error-foreground"
						>
							{error}
						</p>
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

const HomeCrumb: React.FC = () => {
	const { t } = useTranslation();
	return (
		<BreadcrumbItem>
			<BreadcrumbPage>{t("topbar.workspaces")}</BreadcrumbPage>
		</BreadcrumbItem>
	);
};

const ProfileCrumb: React.FC = () => {
	const { t } = useTranslation();
	return (
		<BreadcrumbItem>
			<BreadcrumbPage>{t("topbar.profile")}</BreadcrumbPage>
		</BreadcrumbItem>
	);
};

const SettingsCrumb: React.FC = () => {
	const { t } = useTranslation();
	return (
		<BreadcrumbItem>
			<BreadcrumbPage>{t("topbar.settings", { defaultValue: "Settings" })}</BreadcrumbPage>
		</BreadcrumbItem>
	);
};

const WorkspaceCrumb: React.FC<{ entryId: string }> = ({ entryId }) => {
	const { t } = useTranslation();
	const registry = useSWR(workspacesKey, getWorkspaces);
	const workspace = registry.data?.workspaces.find((entry) => entry.entryId === entryId);
	return (
		<>
			<BreadcrumbItem>
				<BreadcrumbLink asChild>
					<EnvironmentLink to="/">{t("topbar.workspaces")}</EnvironmentLink>
				</BreadcrumbLink>
			</BreadcrumbItem>
			<BreadcrumbSeparator />
			<BreadcrumbItem>
				<BreadcrumbPage>
					{workspace?.name ?? t("topbar.home")}
					{workspace?.id ? (
						<span className="ml-2 font-mono text-xs font-normal text-muted-foreground">
							{workspace.id}
						</span>
					) : null}
				</BreadcrumbPage>
			</BreadcrumbItem>
		</>
	);
};

const SectionCrumb: React.FC<{
	match: { domain?: string; backend?: string };
	settingsRoute?: boolean;
}> = ({ match, settingsRoute = false }) => {
	const { t } = useTranslation();
	const catalog = useBackendCatalog();
	const key = `${match.domain ?? ""}/${match.backend ?? ""}` as SectionKey;
	const backend = catalog.byID.get(key);
	const title = backend
		? t(`sections.${backend.domain}.${backend.name}.title`, {
				defaultValue: humanizeBackendName(backend.name),
			})
		: key;
	return (
		<>
			<BreadcrumbItem>
				<BreadcrumbLink asChild>
					<EnvironmentLink to={settingsRoute ? "/settings" : "/profile"}>
						{settingsRoute
							? t("topbar.settings", { defaultValue: "Settings" })
							: t("topbar.sectionsRoot")}
					</EnvironmentLink>
				</BreadcrumbLink>
			</BreadcrumbItem>
			<BreadcrumbSeparator />
			<BreadcrumbItem>
				<BreadcrumbPage>
					{title}
					{backend ? (
						<span className="ml-2 text-xs font-normal text-muted-foreground">{backend.id}</span>
					) : null}
				</BreadcrumbPage>
			</BreadcrumbItem>
		</>
	);
};
