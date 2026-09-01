import { ArrowRight, Braces, FilePenLine, HardDrive, Save, ShieldCheck } from "lucide-react";
import type React from "react";
import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR, { useSWRConfig } from "swr";
import { humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import {
	getWorkspaceProfileBinding,
	overviewKeyFor,
	updateWorkspaceProfileBinding,
	workspaceProfileBindingKey,
} from "@/api/workspace";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import { ProfileBindingField } from "@/features/profile-binding/ProfileBindingField";
import { useToast } from "@/hooks/useToast";

interface WorkspaceEnvironmentSettingsProps {
	currentBackend?: string;
	environment: string;
	workspaceEntryId?: string;
	readOnly?: boolean;
}

function errorMessage(error: unknown): string {
	if (error && typeof error === "object" && "message" in error) {
		return String(error.message);
	}
	return String(error);
}

export const WorkspaceEnvironmentSettings: React.FC<WorkspaceEnvironmentSettingsProps> = ({
	currentBackend,
	environment,
	workspaceEntryId,
	readOnly = false,
}) => {
	const { t } = useTranslation();
	const dirtyOwner = useId();
	const toast = useToast();
	const { mutate } = useSWRConfig();
	const catalog = useBackendCatalog();
	const binding = useSWR(workspaceProfileBindingKey(workspaceEntryId, environment), () =>
		getWorkspaceProfileBinding(workspaceEntryId, environment),
	);
	const manifestBackend = binding.data ? binding.data.backend : currentBackend;
	const stagedEnvironment = useManifestDraftStore(
		(state) => state.drafts[manifestDraftKey(workspaceEntryId)]?.workspace?.environment,
	);
	const stageWorkspaceSection = useManifestDraftStore((state) => state.stageWorkspaceSection);
	const backend = stagedEnvironment?.backend ?? manifestBackend ?? "";
	const backendChanged = backend !== (manifestBackend ?? "");
	const envBackends = catalog.byDomain.get("env") ?? [];
	const directProfile =
		binding.data?.selectedProfile ??
		(binding.data?.profile?.source === (environment ? "workspace-environment" : "workspace")
			? binding.data.profile.name
			: "");
	const [draftProfile, setDraftProfile] = useState<string | null>(null);
	const selectedProfile = draftProfile ?? directProfile;
	const [saving, setSaving] = useState(false);
	const [saveError, setSaveError] = useState("");
	const setEnvironmentDirty = useEnvironmentDirtyStore((state) => state.setDirty);
	const clearEnvironmentDirty = useEnvironmentDirtyStore((state) => state.clearOwner);

	useEffect(() => {
		setDraftProfile(null);
		clearEnvironmentDirty(dirtyOwner);
	}, [clearEnvironmentDirty, directProfile, dirtyOwner]);

	useEffect(
		() => () => {
			clearEnvironmentDirty(dirtyOwner);
		},
		[clearEnvironmentDirty, dirtyOwner],
	);

	function setWorkspaceDirty(next: boolean) {
		setEnvironmentDirty(dirtyOwner, next, () => {
			setDraftProfile(null);
			setSaveError("");
		});
	}

	const profileDirty = selectedProfile !== directProfile;
	const configurable = Boolean(binding.data?.configurable && manifestBackend && !backendChanged);
	const saveDisabled =
		readOnly || saving || binding.isLoading || !configurable || selectedProfile === directProfile;

	function changeBackend(nextBackend: string) {
		const revision = binding.data?.revision;
		if (!revision || readOnly || profileDirty) return;
		stageWorkspaceSection({
			entryId: workspaceEntryId,
			revision,
			section: "environment",
			initial: { backend: manifestBackend ?? "" },
			next: { backend: nextBackend },
			labels: { backend: "overview.workspaceEnv.backend" },
		});
	}

	async function save() {
		if (saveDisabled) return;
		setSaving(true);
		setSaveError("");
		try {
			const next = await updateWorkspaceProfileBinding(
				selectedProfile,
				workspaceEntryId,
				environment,
			);
			await binding.mutate(next, { revalidate: false });
			setDraftProfile(null);
			clearEnvironmentDirty(dirtyOwner);
			void mutate(overviewKeyFor(workspaceEntryId, environment));
			toast.success(t("overview.workspaceEnv.saved"));
		} catch (error) {
			const message = errorMessage(error);
			setSaveError(message);
			toast.error(t("overview.workspaceEnv.saveFailed"), { description: message });
		} finally {
			setSaving(false);
		}
	}

	return (
		<Card
			role="region"
			aria-labelledby="workspace-environment-title"
			className="overflow-hidden rounded-xl"
		>
			<div className="flex items-center justify-between gap-5 border-b border-border bg-muted/15 px-5 py-4">
				<div className="flex min-w-0 items-center gap-3">
					<div className="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
						<Braces className="h-4.5 w-4.5" />
					</div>
					<div className="min-w-0">
						<div className="flex items-center gap-2">
							<h2 id="workspace-environment-title" className="text-sm font-semibold">
								{t("overview.workspaceEnv.title")}
							</h2>
							<Badge variant="outline" className="text-xs">
								{t("overview.workspaceEnv.scope")}
							</Badge>
							{readOnly ? (
								<Badge variant="secondary" className="text-xs">
									{t("overview.workspaceEnv.readOnly")}
								</Badge>
							) : null}
						</div>
						<p className="mt-1 text-xs leading-relaxed text-muted-foreground">
							{t("overview.workspaceEnv.description")}
						</p>
					</div>
				</div>
				<div className="flex shrink-0 items-center gap-2 text-xs text-muted-foreground">
					<span className="inline-flex items-center gap-1.5 rounded-full border border-warning-border bg-warning-surface px-2.5 py-1 text-warning-foreground">
						<FilePenLine className="h-3.5 w-3.5" />
						{t("overview.workspaceEnv.manifestLegend")}
					</span>
					<span className="inline-flex items-center gap-1.5 rounded-full border border-primary/20 bg-primary/8 px-2.5 py-1 text-primary">
						<HardDrive className="h-3.5 w-3.5" />
						{t("overview.workspaceEnv.localLegend")}
					</span>
				</div>
			</div>

			<div className="grid grid-cols-2 gap-4 p-5">
				<section
					className="rounded-xl border border-border bg-background p-4"
					aria-labelledby="workspace-backend-title"
				>
					<div className="flex items-start justify-between gap-3">
						<div>
							<h3 id="workspace-backend-title" className="text-sm font-semibold">
								{t("overview.workspaceEnv.backend")}
							</h3>
							<p className="mt-1 text-xs leading-relaxed text-muted-foreground">
								{t("overview.workspaceEnv.backendDescription")}
							</p>
						</div>
						<Badge variant={backendChanged ? "secondary" : "outline"} className="text-xs">
							{backendChanged
								? t("overview.workspaceEnv.backendPending")
								: t("overview.workspaceEnv.backendManifest")}
						</Badge>
					</div>
					<Select
						value={backend || undefined}
						onValueChange={changeBackend}
						disabled={
							readOnly ||
							binding.isLoading ||
							catalog.isLoading ||
							!binding.data?.revision ||
							profileDirty ||
							envBackends.length === 0
						}
					>
						<SelectTrigger
							className="mt-3 font-mono"
							aria-label={t("overview.workspaceEnv.backend")}
						>
							<SelectValue placeholder={t("overview.workspaceEnv.backendMissing")} />
						</SelectTrigger>
						<SelectContent>
							{envBackends.map((candidate) => (
								<SelectItem key={candidate.id} value={candidate.name}>
									{humanizeBackendName(candidate.name)}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
					{backendChanged ? (
						<div
							role="status"
							className="mt-3 rounded-lg border border-warning-border bg-warning-surface/55 p-3 text-warning-foreground"
						>
							<div className="flex items-center gap-2 font-mono text-xs font-semibold">
								<span>
									{humanizeBackendName(
										manifestBackend ?? t("overview.workspaceEnv.backendMissing"),
									)}
								</span>
								<ArrowRight className="h-3.5 w-3.5" />
								<span>{humanizeBackendName(backend)}</span>
							</div>
							<p className="mt-2 text-xs leading-relaxed">
								{t("overview.workspaceEnv.backendPendingHint")}
							</p>
						</div>
					) : (
						<p className="mt-2 text-xs leading-relaxed text-muted-foreground">
							{t("overview.workspaceEnv.backendSource")}
						</p>
					)}
				</section>

				<section
					className="rounded-xl border border-border bg-background p-4"
					aria-labelledby="workspace-profile-title"
				>
					<div className="flex items-start justify-between gap-3">
						<div>
							<h3 id="workspace-profile-title" className="text-sm font-semibold">
								{t("overview.workspaceEnv.profile.title")}
							</h3>
							<p className="mt-1 text-xs leading-relaxed text-muted-foreground">
								{t("overview.workspaceEnv.profile.description")}
							</p>
						</div>
						<Badge
							variant="outline"
							className="border-primary/20 bg-primary/8 text-xs text-primary"
						>
							<HardDrive className="h-3 w-3" />
							{t("overview.workspaceEnv.profile.localBadge")}
						</Badge>
					</div>

					{binding.isLoading ? (
						<div className="mt-3 space-y-2" role="status">
							<span className="sr-only">{t("overview.workspaceEnv.loading")}</span>
							<Skeleton className="h-20 w-full rounded-lg" />
						</div>
					) : null}
					{binding.error ? (
						<Alert variant="destructive" className="mt-3">
							<AlertDescription>
								{t("overview.workspaceEnv.loadFailed")} {errorMessage(binding.error)}
							</AlertDescription>
						</Alert>
					) : null}
					{binding.data && backendChanged ? (
						<div className="mt-3 rounded-lg border border-warning-border bg-warning-surface/35 p-3">
							<div className="flex items-start gap-2">
								<FilePenLine className="mt-0.5 h-4 w-4 shrink-0 text-warning-foreground" />
								<div>
									<p className="text-xs font-semibold">
										{t("overview.workspaceEnv.profile.waitTitle")}
									</p>
									<p className="mt-1 text-xs leading-relaxed text-muted-foreground">
										{t("overview.workspaceEnv.profile.waitDescription")}
									</p>
								</div>
							</div>
						</div>
					) : null}
					{binding.data && !backend ? (
						<p className="mt-3 rounded-lg border border-border bg-muted/25 p-3 text-xs text-muted-foreground">
							{t("overview.workspaceEnv.missingBackendCli")}
						</p>
					) : null}
					{binding.data && !backendChanged && manifestBackend && !binding.data.configurable ? (
						<div className="mt-3">
							<ProfileBindingField
								id="workspace-env-profile"
								scope="workspace"
								directSource={environment ? "workspace-environment" : "workspace"}
								domain="env"
								backend={manifestBackend ?? ""}
								configurable={false}
								binding={binding.data.profile}
								value=""
								onChange={() => undefined}
								disabled
							/>
						</div>
					) : null}
					{binding.data && configurable ? (
						<form
							onSubmit={(event) => {
								event.preventDefault();
								void save();
							}}
							aria-busy={saving}
							className="mt-3 space-y-3"
						>
							<ProfileBindingField
								id="workspace-env-profile"
								scope="workspace"
								directSource={environment ? "workspace-environment" : "workspace"}
								domain="env"
								backend={manifestBackend ?? ""}
								configurable
								binding={binding.data.profile}
								value={selectedProfile}
								onChange={(value) => {
									setDraftProfile(value);
									setSaveError("");
									setWorkspaceDirty(value !== directProfile);
								}}
								disabled={readOnly || saving}
							/>
							{saveError ? (
								<p role="alert" className="text-xs text-error-foreground">
									{t("overview.workspaceEnv.saveFailed")} {saveError}
								</p>
							) : null}
							<div className="flex items-center justify-between gap-3 border-t border-border pt-3">
								<p className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
									<ShieldCheck className="h-3.5 w-3.5 text-success-foreground" />
									{profileDirty
										? t("overview.workspaceEnv.profile.unsaved")
										: t("overview.workspaceEnv.profile.savedLocally")}
								</p>
								<Button type="submit" size="sm" disabled={saveDisabled}>
									{saving ? <Spinner /> : <Save />}
									{saving ? t("overview.workspaceEnv.saving") : t("overview.workspaceEnv.save")}
								</Button>
							</div>
						</form>
					) : null}
				</section>
			</div>
		</Card>
	);
};
