import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSWRConfig } from "swr";
import { updateProjectProfileBinding } from "@/api/workspace";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import { ProfileBindingField } from "@/features/profile-binding/ProfileBindingField";
import {
	BackendBanner,
	CheckField,
	FormLayout,
	ProjectField,
	type ProjectSettingsFormProps,
} from "@/features/project-settings/forms/FormLayout";
import {
	projectBindingValue,
	refreshOverview,
	showSaveError,
} from "@/features/project-settings/forms/helpers";
import { useToast } from "@/hooks/useToast";
import type { ProjectEnvironmentPatch } from "@/types/api";

export const EnvironmentForm: React.FC<ProjectSettingsFormProps> = ({
	project,
	revision,
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
	const staged = useManifestDraftStore(
		(state) => state.drafts[manifestDraftKey(workspaceEntryId)]?.changes[project.name]?.environment,
	);
	const stageSection = useManifestDraftStore((state) => state.stageSection);
	const initialManifest: ProjectEnvironmentPatch = {
		path: settings.path ?? "",
		inherits: settings.inherits,
		disabled: settings.disabled,
	};
	const manifest = staged ?? initialManifest;

	function updateManifest(next: ProjectEnvironmentPatch) {
		stageSection({
			entryId: workspaceEntryId,
			revision,
			project: project.name,
			section: "environment",
			initial: initialManifest,
			next,
			labels: {
				path: "projectInspector.environment.path",
				inherits: "projectInspector.environment.inherits",
				disabled: "projectInspector.environment.disabled",
			},
		});
	}

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
			<div className="space-y-3 rounded-lg border border-warning-border/70 bg-warning-surface/35 p-4">
				<ProjectField label={t("projectInspector.environment.path")} htmlFor="project-env-path">
					<Input
						id="project-env-path"
						className="font-mono"
						value={manifest.path}
						onChange={(event) => updateManifest({ ...manifest, path: event.target.value })}
						disabled={readOnly}
					/>
				</ProjectField>
				<div className="grid grid-cols-2 gap-3">
					<CheckField
						id="project-env-inherits"
						label={t("projectInspector.environment.inherits")}
						description={t("projectInspector.environment.inheritsHint")}
						checked={manifest.inherits}
						onChange={(inherits) => updateManifest({ ...manifest, inherits })}
						disabled={readOnly}
					/>
					<CheckField
						id="project-env-disabled"
						label={t("projectInspector.environment.disabled")}
						description={t("projectInspector.environment.disabledHint")}
						checked={manifest.disabled}
						onChange={(disabled) => updateManifest({ ...manifest, disabled })}
						disabled={readOnly}
					/>
				</div>
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
