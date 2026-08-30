import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSWRConfig } from "swr";
import { humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import { updateProjectProfileBinding } from "@/api/workspace";
import { Badge } from "@/components/ui/badge";
import { ProfileBindingField } from "@/features/profile-binding/ProfileBindingField";
import {
	BackendBanner,
	FormLayout,
	type ProjectSettingsFormProps,
	ReadOnlyDatum,
} from "@/features/project-settings/forms/FormLayout";
import {
	configPathValue,
	projectBindingValue,
	refreshOverview,
	showSaveError,
} from "@/features/project-settings/forms/helpers";
import { useToast } from "@/hooks/useToast";

export const DeployForm: React.FC<ProjectSettingsFormProps> = ({
	project,
	environment,
	workspaceEntryId,
	readOnly,
	onUpdated,
	onDirtyChange,
}) => {
	const { t } = useTranslation();
	const toast = useToast();
	const { mutate } = useSWRConfig();
	const catalog = useBackendCatalog();
	const settings = project.deploy;
	const initialProfile = projectBindingValue(
		settings.selectedProfile,
		settings.profile,
		environment,
	);
	const [profile, setProfile] = useState(initialProfile);
	const [saving, setSaving] = useState(false);
	const selected = settings.backend ? catalog.byID.get(`deploy/${settings.backend}`) : undefined;
	const manifestFields = selected?.project?.fields ?? [];

	async function save() {
		if (!settings.backend || profile === initialProfile || readOnly) return;
		setSaving(true);
		try {
			const next = await updateProjectProfileBinding(
				project.name,
				"deploy",
				profile,
				workspaceEntryId,
				environment,
			);
			onUpdated(next);
			refreshOverview(mutate, workspaceEntryId, environment);
			toast.success(t("projectInspector.saved"));
		} catch (error) {
			showSaveError(toast, t("projectInspector.saveFailed"), error);
		} finally {
			setSaving(false);
		}
	}

	return (
		<FormLayout
			title={t("projectInspector.deploy.title")}
			description={t("projectInspector.deploy.description")}
			onSave={save}
			saving={saving}
			disabled={readOnly || !settings.backend || profile === initialProfile}
		>
			<BackendBanner domain="deploy" backend={settings.backend} />

			<div className="space-y-4 rounded-lg border border-border bg-muted/25 p-4">
				<div className="grid grid-cols-2 gap-x-5 gap-y-4">
					<ReadOnlyDatum label={t("projectInspector.backend")} value={settings.backend} mono />
					<ReadOnlyDatum
						label={t("projectInspector.deploy.compatibleTargets")}
						value={
							<div className="flex flex-wrap gap-1">
								{settings.compatibleTargets?.map((target) => (
									<Badge key={target} variant="outline" className="font-mono text-[9px]">
										{target}
									</Badge>
								))}
							</div>
						}
						className="overflow-visible whitespace-normal"
					/>
					{manifestFields.map((field) => {
						const label = t(field.label_key, {
							defaultValue: humanizeBackendName(field.input_name),
						});
						const value = configPathValue(settings.config ?? {}, field.path);
						return (
							<ReadOnlyDatum
								key={field.path}
								label={label}
								value={value === undefined || value === null ? undefined : String(value)}
								mono
							/>
						);
					})}
				</div>
				{manifestFields.length === 0 && Object.keys(settings.config ?? {}).length > 0 ? (
					<pre className="overflow-x-auto rounded-md border border-border bg-background/70 p-3 font-mono text-[10px] leading-relaxed text-muted-foreground">
						{JSON.stringify(settings.config, null, 2)}
					</pre>
				) : null}
			</div>

			<ProfileBindingField
				id="project-profile-deploy"
				scope="project"
				directSource={environment ? "workspace-project-environment" : "workspace-project"}
				domain="deploy"
				backend={settings.backend}
				binding={settings.profile}
				value={profile}
				onChange={(value) => {
					setProfile(value);
					onDirtyChange(value !== initialProfile);
				}}
				disabled={readOnly || saving}
			/>
		</FormLayout>
	);
};
