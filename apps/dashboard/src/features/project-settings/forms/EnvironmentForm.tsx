import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSWRConfig } from "swr";
import { updateProjectProfileBinding } from "@/api/workspace";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
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

export const EnvironmentForm: React.FC<ProjectSettingsFormProps> = ({
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
	const settings = project.environment;
	const initialProfile = projectBindingValue(
		settings.selectedProfile,
		settings.profile,
		environment,
	);
	const [profile, setProfile] = useState(initialProfile);
	const [saving, setSaving] = useState(false);

	async function save() {
		if (!settings.backend || profile === initialProfile || readOnly) return;
		setSaving(true);
		try {
			const next = await updateProjectProfileBinding(
				project.name,
				"env",
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
			title={t("projectInspector.environment.title")}
			description={t("projectInspector.environment.description")}
			onSave={save}
			saving={saving}
			disabled={readOnly || !settings.backend || profile === initialProfile}
		>
			<BackendBanner domain="env" backend={settings.backend} />
			<div className="grid grid-cols-3 gap-x-4 gap-y-3 rounded-lg border border-border bg-muted/25 p-4">
				<ReadOnlyDatum label={t("projectInspector.environment.path")} value={settings.path} mono />
				<ReadOnlyDatum
					label={t("projectInspector.environment.inherits")}
					value={t(
						settings.inherits
							? "projectInspector.values.enabled"
							: "projectInspector.values.disabled",
					)}
				/>
				<ReadOnlyDatum
					label={t("projectInspector.environment.disabled")}
					value={t(
						settings.disabled
							? "projectInspector.values.enabled"
							: "projectInspector.values.disabled",
					)}
				/>
			</div>

			{(settings.keys?.length ?? 0) > 0 ? (
				<div>
					<Label>{t("projectInspector.environment.keys")}</Label>
					<div className="mt-2 flex flex-wrap gap-1.5 rounded-lg border border-border bg-muted/25 p-3">
						{settings.keys?.map((key) => (
							<Badge key={key} variant="outline" className="font-mono">
								{key}
							</Badge>
						))}
					</div>
				</div>
			) : null}

			<ProfileBindingField
				id="project-profile-env"
				scope="project"
				directSource={environment ? "workspace-project-environment" : "workspace-project"}
				domain="env"
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
