import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSWRConfig } from "swr";
import { humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import { updateProjectProfileBinding } from "@/api/workspace";
import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
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
import type { ProjectContainerPatch } from "@/types/api";

export const ContainerForm: React.FC<ProjectSettingsFormProps> = ({
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
	const catalog = useBackendCatalog();
	const settings = project.container;
	const initialProfile = projectBindingValue(
		settings.selectedProfile,
		settings.profile,
		environment,
	);
	const [profile, setProfile] = useState(initialProfile);
	const [saving, setSaving] = useState(false);
	const staged = useManifestDraftStore(
		(state) => state.drafts[manifestDraftKey(workspaceEntryId)]?.changes[project.name]?.container,
	);
	const stageSection = useManifestDraftStore((state) => state.stageSection);
	const initialManifest: ProjectContainerPatch = {
		enabled: settings.enabled,
		backend: settings.backend ?? "docker",
		image: settings.image ?? "",
		namespace: settings.namespace ?? "",
	};
	const manifest = staged ?? initialManifest;
	const backends = catalog.byDomain.get("container") ?? [];

	function updateManifest(next: ProjectContainerPatch) {
		stageSection({
			entryId: workspaceEntryId,
			revision,
			project: project.name,
			section: "container",
			initial: initialManifest,
			next,
			labels: {
				enabled: "projectInspector.container.enabled",
				backend: "projectInspector.backend",
				image: "projectInspector.container.image",
				namespace: "projectInspector.container.namespace",
			},
		});
	}

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
			<div className="space-y-3 rounded-lg border border-warning-border/70 bg-warning-surface/35 p-4">
				<CheckField
					id="project-container-enabled"
					label={t("projectInspector.container.enabled")}
					description={t("projectInspector.container.enabledHint")}
					checked={manifest.enabled}
					onChange={(enabled) => updateManifest({ ...manifest, enabled })}
					disabled={readOnly}
				/>
				<div className="grid grid-cols-2 gap-3">
					<ProjectField label={t("projectInspector.backend")} htmlFor="project-container-backend">
						<Select
							value={manifest.backend}
							onValueChange={(backend) => updateManifest({ ...manifest, backend })}
							disabled={readOnly || !manifest.enabled}
						>
							<SelectTrigger id="project-container-backend">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								{backends.map((backend) => (
									<SelectItem key={backend.id} value={backend.name}>
										{humanizeBackendName(backend.name)}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</ProjectField>
					<ProjectField
						label={t("projectInspector.container.namespace")}
						htmlFor="project-container-namespace"
					>
						<Input
							id="project-container-namespace"
							value={manifest.namespace}
							onChange={(event) => updateManifest({ ...manifest, namespace: event.target.value })}
							disabled={readOnly || !manifest.enabled}
						/>
					</ProjectField>
				</div>
				<ProjectField
					label={t("projectInspector.container.image")}
					htmlFor="project-container-image"
				>
					<Input
						id="project-container-image"
						className="font-mono"
						value={manifest.image}
						onChange={(event) => updateManifest({ ...manifest, image: event.target.value })}
						disabled={readOnly || !manifest.enabled}
					/>
				</ProjectField>
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
