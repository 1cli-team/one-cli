import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSWRConfig } from "swr";
import { updateProjectProfileBinding } from "@/api/workspace";
import { ProfileBindingField } from "@/features/profile-binding/ProfileBindingField";
import {
	BackendBanner,
	FormLayout,
	type ProjectSettingsFormProps,
	ReadOnlyDatum,
} from "@/features/project-settings/forms/FormLayout";
import {
	projectBindingValue,
	refreshOverview,
	showSaveError,
} from "@/features/project-settings/forms/helpers";
import { useToast } from "@/hooks/useToast";

export const ContainerForm: React.FC<ProjectSettingsFormProps> = ({
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
	const settings = project.container;
	const initialProfile = projectBindingValue(
		settings.selectedProfile,
		settings.profile,
		environment,
	);
	const [profile, setProfile] = useState(initialProfile);
	const [saving, setSaving] = useState(false);

	async function save() {
		if (!settings.enabled || !settings.backend || profile === initialProfile || readOnly) return;
		setSaving(true);
		try {
			const next = await updateProjectProfileBinding(
				project.name,
				"container",
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
			title={t("projectInspector.container.title")}
			description={t("projectInspector.container.description")}
			onSave={save}
			saving={saving}
			disabled={readOnly || !settings.enabled || !settings.backend || profile === initialProfile}
		>
			<BackendBanner domain="container" backend={settings.backend} />
			<div className="grid grid-cols-2 gap-x-5 gap-y-4 rounded-lg border border-border bg-muted/25 p-4">
				<ReadOnlyDatum
					label={t("projectInspector.container.enabled")}
					value={t(
						settings.enabled
							? "projectInspector.values.enabled"
							: "projectInspector.values.disabled",
					)}
				/>
				<ReadOnlyDatum label={t("projectInspector.backend")} value={settings.backend} mono />
				<ReadOnlyDatum label={t("projectInspector.container.image")} value={settings.image} mono />
				<ReadOnlyDatum
					label={t("projectInspector.container.namespace")}
					value={settings.namespace}
				/>
			</div>

			<ProfileBindingField
				id="project-profile-container"
				scope="project"
				directSource={environment ? "workspace-project-environment" : "workspace-project"}
				domain="container"
				backend={settings.enabled ? settings.backend : undefined}
				binding={settings.profile}
				value={profile}
				onChange={(value) => {
					setProfile(value);
					onDirtyChange(value !== initialProfile);
				}}
				disabled={readOnly || saving || !settings.enabled}
			/>
		</FormLayout>
	);
};
