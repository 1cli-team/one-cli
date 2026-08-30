import { Braces, Save } from "lucide-react";
import type React from "react";
import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR, { useSWRConfig } from "swr";
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
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
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
	const binding = useSWR(workspaceProfileBindingKey(workspaceEntryId, environment), () =>
		getWorkspaceProfileBinding(workspaceEntryId, environment),
	);
	const backend = binding.data ? binding.data.backend : currentBackend;
	const directProfile =
		binding.data?.selectedProfile ??
		(binding.data?.profile?.source === (environment ? "workspace-environment" : "workspace")
			? binding.data.profile.name
			: "");
	const [selectedProfile, setSelectedProfile] = useState("");
	const [savedProfile, setSavedProfile] = useState("");
	const [saving, setSaving] = useState(false);
	const [saveError, setSaveError] = useState("");
	const setEnvironmentDirty = useEnvironmentDirtyStore((state) => state.setDirty);
	const clearEnvironmentDirty = useEnvironmentDirtyStore((state) => state.clearOwner);

	useEffect(() => {
		setSelectedProfile(directProfile);
		setSavedProfile(directProfile);
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
			setSelectedProfile(savedProfile);
			setSaveError("");
		});
	}

	const configurable = Boolean(binding.data?.configurable && backend);
	const saveDisabled =
		readOnly || saving || binding.isLoading || !configurable || selectedProfile === savedProfile;

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
			const nextDirect =
				next.selectedProfile ??
				(next.profile?.source === (environment ? "workspace-environment" : "workspace")
					? next.profile.name
					: "");
			await binding.mutate(next, { revalidate: false });
			setSelectedProfile(nextDirect);
			setSavedProfile(nextDirect);
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
			<div className="grid min-h-32 grid-cols-[minmax(0,0.82fr)_minmax(430px,1fr)]">
				<div className="relative flex min-w-0 items-center gap-4 border-r border-border px-5 py-4">
					<div className="absolute inset-y-0 left-0 w-1 bg-primary" aria-hidden="true" />
					<div className="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
						<Braces className="h-4.5 w-4.5" />
					</div>
					<div className="min-w-0">
						<div className="flex items-center gap-2">
							<h2 id="workspace-environment-title" className="text-sm font-semibold">
								{t("overview.workspaceEnv.title")}
							</h2>
							<Badge variant="outline" className="text-[10px]">
								{t("overview.workspaceEnv.scope")}
							</Badge>
							{readOnly ? (
								<Badge variant="secondary" className="text-[10px]">
									{t("overview.workspaceEnv.readOnly")}
								</Badge>
							) : null}
						</div>
						<p className="mt-1 max-w-xl text-xs leading-relaxed text-muted-foreground">
							{t("overview.workspaceEnv.description")}
						</p>
						<span className="mt-2 inline-block rounded bg-muted/55 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
							{t("overview.workspaceEnv.localBinding")}
						</span>
					</div>
				</div>

				<div className="space-y-3 bg-muted/15 px-5 py-4">
					<div>
						<div className="flex items-center justify-between gap-3">
							<p className="text-xs font-medium">{t("overview.workspaceEnv.backend")}</p>
							<Badge variant="secondary" className="text-[10px]">
								{t("overview.workspaceEnv.backendReadOnly")}
							</Badge>
						</div>
						<p className="mt-1 font-mono text-sm font-semibold">
							{backend || t("overview.workspaceEnv.backendMissing")}
						</p>
						<p className="mt-0.5 text-[10px] text-muted-foreground">
							{t("overview.workspaceEnv.backendSource")}
						</p>
					</div>

					{binding.isLoading ? (
						<div className="space-y-2" role="status">
							<span className="sr-only">{t("overview.workspaceEnv.loading")}</span>
							<Skeleton className="h-20 w-full rounded-lg" />
						</div>
					) : null}
					{binding.error ? (
						<Alert variant="destructive">
							<AlertDescription>
								{t("overview.workspaceEnv.loadFailed")} {errorMessage(binding.error)}
							</AlertDescription>
						</Alert>
					) : null}
					{binding.data && !backend ? (
						<p className="rounded-lg border border-border bg-background/70 p-3 text-xs text-muted-foreground">
							{t("overview.workspaceEnv.missingBackendCli")}
						</p>
					) : null}
					{binding.data && backend && !binding.data.configurable ? (
						<ProfileBindingField
							id="workspace-env-profile"
							scope="workspace"
							directSource={environment ? "workspace-environment" : "workspace"}
							domain="env"
							backend={backend}
							configurable={false}
							binding={binding.data.profile}
							value=""
							onChange={() => undefined}
							disabled
						/>
					) : null}
					{binding.data && configurable ? (
						<form
							onSubmit={(event) => {
								event.preventDefault();
								void save();
							}}
							aria-busy={saving}
							className="space-y-2.5"
						>
							<ProfileBindingField
								id="workspace-env-profile"
								scope="workspace"
								directSource={environment ? "workspace-environment" : "workspace"}
								domain="env"
								backend={backend}
								configurable
								binding={binding.data.profile}
								value={selectedProfile}
								onChange={(value) => {
									setSelectedProfile(value);
									setSaveError("");
									setWorkspaceDirty(value !== savedProfile);
								}}
								disabled={readOnly || saving}
							/>
							{saveError ? (
								<p role="alert" className="text-[11px] text-error-foreground">
									{t("overview.workspaceEnv.saveFailed")} {saveError}
								</p>
							) : null}
							<div className="flex justify-end">
								<Button type="submit" size="sm" disabled={saveDisabled}>
									{saving ? <Spinner /> : <Save />}
									{saving ? t("overview.workspaceEnv.saving") : t("overview.workspaceEnv.save")}
								</Button>
							</div>
						</form>
					) : null}
				</div>
			</div>
		</Card>
	);
};
