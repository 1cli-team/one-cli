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
	ProjectField,
	SwitchField,
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
	const profileBackendChanged =
		manifest.enabled !== initialManifest.enabled || manifest.backend !== initialManifest.backend;
	const initialProfile = projectBindingValue(
		settings.selectedProfile,
		settings.profile,
		environment,
	);
	const profileBaseline = profileBackendChanged ? "" : initialProfile;
	const [profile, setProfile] = useState(profileBaseline);
	const [saving, setSaving] = useState(false);
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

	function selectEnabled(enabled: boolean) {
		updateManifest({ ...manifest, enabled });
		const bindingChanged =
			enabled !== initialManifest.enabled || manifest.backend !== initialManifest.backend;
		setProfile(bindingChanged ? "" : initialProfile);
		onDirtyChange(false);
	}

	function selectBackend(backend: string) {
		updateManifest({ ...manifest, backend });
		const bindingChanged =
			manifest.enabled !== initialManifest.enabled || backend !== initialManifest.backend;
		setProfile(bindingChanged ? "" : initialProfile);
		onDirtyChange(false);
	}

	async function saveProfile(nextProfile: string) {
		if (
			!manifest.enabled ||
			!manifest.backend ||
			profileBackendChanged ||
			nextProfile === initialProfile ||
			readOnly ||
			saving
		)
			return;
		setProfile(nextProfile);
		onDirtyChange(true);
		setSaving(true);
		try {
			const next = await updateProjectProfileBinding(
				project.name,
				"container",
				nextProfile,
				workspaceEntryId,
				environment,
			);
			onUpdated(next);
			refreshOverview(mutate, workspaceEntryId, environment);
			toast.success(t("projectInspector.saved"));
		} catch (error) {
			setProfile(initialProfile);
			showSaveError(toast, t("projectInspector.saveFailed"), error);
		} finally {
			setSaving(false);
			onDirtyChange(false);
		}
	}

	return (
		<section aria-label={t("projectInspector.container.title")} className="space-y-3">
			<h4 className="text-sm font-semibold tracking-tight">
				{t("projectInspector.container.title")}
			</h4>
			<div
				data-testid="image-settings-grid"
				className="@container/backend-config space-y-3"
			>
				<SwitchField
					id="project-container-enabled"
					label={t("projectInspector.container.enabled")}
					description={t("projectInspector.container.enabledHint")}
					checked={manifest.enabled}
					onChange={selectEnabled}
					disabled={readOnly}
				/>
				<div className="grid gap-3 sm:grid-cols-2">
					<ProjectField label={t("projectInspector.backend")} htmlFor="project-container-backend">
						<Select
							value={manifest.backend}
							onValueChange={selectBackend}
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
					<ProfileBindingField
						id="project-profile-container"
						scope="project"
						directSource={environment ? "workspace-project-environment" : "workspace-project"}
						domain="container"
						backend={manifest.enabled ? manifest.backend : undefined}
						binding={profileBackendChanged ? undefined : settings.profile}
						value={profile}
						onChange={(value) => void saveProfile(value)}
						disabled={readOnly || saving || !manifest.enabled || profileBackendChanged}
						variant="embedded"
					/>
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
		</section>
	);
};
