import { ArrowRight, Braces, FilePenLine } from "lucide-react";
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
import { Card } from "@/components/ui/card";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
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

	async function selectProfile(nextProfile: string) {
		if (
			readOnly ||
			saving ||
			binding.isLoading ||
			!configurable ||
			nextProfile === directProfile
		)
			return;
		setDraftProfile(nextProfile);
		setWorkspaceDirty(true);
		setSaving(true);
		setSaveError("");
		try {
			const next = await updateWorkspaceProfileBinding(
				nextProfile,
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
			setDraftProfile(null);
			clearEnvironmentDirty(dirtyOwner);
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
			className="overflow-hidden rounded-[6px] border-border shadow-none"
		>
			<div className="border-b border-border px-3 py-2.5">
				<div className="flex min-w-0 items-center gap-3">
					<div className="grid size-8 shrink-0 place-items-center rounded-[6px] bg-primary/8 text-primary">
						<Braces className="size-4" />
					</div>
					<div className="min-w-0">
						<div className="flex items-center gap-2">
							<h2 id="workspace-environment-title" className="text-sm font-semibold">
								{t("overview.workspaceEnv.title")}
							</h2>
							<span className="font-mono text-[8px] font-semibold tracking-[0.12em] text-muted-foreground uppercase">
								{t("overview.workspaceEnv.scope")}
							</span>
							{readOnly ? (
								<Badge variant="secondary" className="rounded-[6px] px-2 text-[9px]">
									{t("overview.workspaceEnv.readOnly")}
								</Badge>
							) : null}
						</div>
						<p className="mt-0.5 text-xs leading-relaxed text-muted-foreground">
							{t("overview.workspaceEnv.description")}
						</p>
					</div>
				</div>
			</div>

			<div className="p-3">
				<section
					data-testid="workspace-backend-settings"
					className="@container/workspace-backend"
					aria-labelledby="workspace-backend-title"
				>
					<div className="grid items-start gap-3 @4xl/workspace-backend:grid-cols-2">
						<div className="min-w-0">
							<div className="flex items-start justify-between gap-3">
								<div>
									<h3 id="workspace-backend-title" className="text-sm font-semibold">
										{t("overview.workspaceEnv.backend")}
									</h3>
									<p className="mt-1 text-xs leading-relaxed text-muted-foreground">
										{t("overview.workspaceEnv.backendDescription")}
									</p>
								</div>
								<Badge
									variant={backendChanged ? "secondary" : "outline"}
									className="rounded-[6px] px-2 font-mono text-[8px] tracking-wide uppercase"
								>
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
									className="mt-3 rounded-[5px] border border-warning-border bg-warning-surface/55 p-2.5 text-warning-foreground"
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
						</div>

						<div className="min-w-0 @4xl/workspace-backend:border-l @4xl/workspace-backend:border-border @4xl/workspace-backend:pl-3">
							{binding.isLoading ? (
								<div className="space-y-2" role="status">
									<span className="sr-only">{t("overview.workspaceEnv.loading")}</span>
									<Skeleton className="h-20 w-full" />
								</div>
							) : null}
							{binding.error ? (
								<Alert variant="destructive">
									<AlertDescription>
										{t("overview.workspaceEnv.loadFailed")} {errorMessage(binding.error)}
									</AlertDescription>
								</Alert>
							) : null}
							{binding.data && backendChanged ? (
								<div className="rounded-[5px] border border-warning-border bg-warning-surface/35 p-2.5">
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
								<p className="rounded-[5px] border border-border bg-muted/25 p-2.5 text-xs text-muted-foreground">
									{t("overview.workspaceEnv.missingBackendCli")}
								</p>
							) : null}
							{binding.data && !backendChanged && manifestBackend && !binding.data.configurable ? (
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
									variant="embedded"
									showDescription
								/>
							) : null}
							{binding.data && configurable ? (
								<div aria-busy={saving} className="space-y-4">
									<ProfileBindingField
										id="workspace-env-profile"
										scope="workspace"
										directSource={environment ? "workspace-environment" : "workspace"}
										domain="env"
										backend={manifestBackend ?? ""}
										configurable
										binding={binding.data.profile}
										value={selectedProfile}
										onChange={(value) => void selectProfile(value)}
										disabled={readOnly || saving}
										variant="embedded"
										showDescription
									/>
									{saveError ? (
										<p role="alert" className="text-xs text-error-foreground">
											{t("overview.workspaceEnv.saveFailed")} {saveError}
										</p>
									) : null}
								</div>
							) : null}
						</div>
					</div>
				</section>
			</div>
		</Card>
	);
};
