import { AlertTriangle, FilePenLine, Save, Trash2 } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useMatch } from "react-router-dom";
import useSWR, { useSWRConfig } from "swr";
import { humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import { applyManifestDraft } from "@/api/manifest";
import { switchWorkspaceEnvironmentBackend } from "@/api/workspace";
import { getWorkspaces, workspacesKey } from "@/api/workspaces";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";
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
import { EnvironmentSelector } from "@/features/environment-context/EnvironmentSelector";
import { environmentFromSearch } from "@/features/environment-context/environment";
import {
	displayDraftValue,
	manifestDraftKey,
	useManifestDraftStore,
	WORKSPACE_DRAFT_SUBJECT,
} from "@/features/manifest-draft/manifest-draft-store";
import { useToast } from "@/hooks/useToast";
import type { ApplyManifestRequest, HttpError } from "@/types/api";
import type { SectionKey } from "@/types/api";

export const TopBar: React.FC = () => {
	const sectionMatch = useMatch("/section/:domain/:backend");
	const profileMatch = useMatch("/profile");
	const settingsSectionMatch = useMatch("/settings/:domain/:backend");
	const settingsMatch = useMatch("/settings");
	const workspaceMatch = useMatch("/workspace/:entryId");
	const detailMatch = settingsSectionMatch ?? sectionMatch;
	const showEnvironmentSelector = Boolean(workspaceMatch);

	return (
		<header className="flex h-[68px] shrink-0 items-center justify-between gap-4 border-b border-border bg-background/90 px-7 backdrop-blur">
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
			<div className="flex items-center gap-2">
				{showEnvironmentSelector ? <EnvironmentSelector /> : null}
				{workspaceMatch ? (
					<ManifestSaveControl entryId={workspaceMatch.params.entryId ?? ""} />
				) : null}
				<LanguageSwitcher />
			</div>
		</header>
	);
};

const ManifestSaveControl: React.FC<{ entryId: string }> = ({ entryId }) => {
	const { t } = useTranslation();
	const { search } = useLocation();
	const { mutate } = useSWRConfig();
	const toast = useToast();
	const draft = useManifestDraftStore((state) => state.drafts[manifestDraftKey(entryId)]);
	const commitWorkspaceSection = useManifestDraftStore((state) => state.commitWorkspaceSection);
	const clearWorkspace = useManifestDraftStore((state) => state.clearWorkspace);
	const [open, setOpen] = useState(false);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState("");

	if (!draft) return null;

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
				className="border-warning-border bg-warning-surface text-warning-foreground shadow-sm hover:bg-warning-surface/80"
				onClick={() => {
					setError("");
					setOpen(true);
				}}
			>
				<FilePenLine className="h-4 w-4" />
				{t("manifestDraft.saveButton", { count: draft.summaries.length })}
			</Button>

			<AlertDialog open={open} onOpenChange={(next) => !saving && setOpen(next)}>
				<AlertDialogContent className="max-w-2xl">
					<AlertDialogHeader>
						<div className="flex items-start gap-3">
							<div className="mt-0.5 grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-warning-surface text-warning-foreground">
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

					<div className="max-h-72 space-y-2 overflow-y-auto rounded-lg border border-border bg-muted/20 p-2">
						{draft.summaries.map((summary) => (
							<div
								key={summary.id}
								className="rounded-md border border-border bg-background px-3 py-2.5"
							>
								<div className="flex items-center justify-between gap-4">
									<span className="font-mono text-[10px] font-semibold text-primary">
										{summary.project === WORKSPACE_DRAFT_SUBJECT
											? t("manifestDraft.workspaceScope")
											: summary.project}
									</span>
									<span className="text-[10px] font-medium text-muted-foreground">
										{t(summary.labelKey, { defaultValue: summary.labelKey })}
									</span>
								</div>
								<div className="mt-2 grid grid-cols-[1fr_auto_1fr] items-center gap-2 font-mono text-[11px]">
									<span className="truncate rounded bg-muted px-2 py-1 text-muted-foreground line-through">
										{displayDraftValue(summary.before)}
									</span>
									<span aria-hidden="true" className="text-muted-foreground">
										→
									</span>
									<span className="truncate rounded bg-primary/8 px-2 py-1 text-primary">
										{displayDraftValue(summary.after)}
									</span>
								</div>
							</div>
						))}
					</div>

					{error ? (
						<p
							role="alert"
							className="rounded-md border border-error-border bg-error-surface px-3 py-2 text-xs text-error-foreground"
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
							<Button onClick={() => void save()} disabled={saving}>
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
