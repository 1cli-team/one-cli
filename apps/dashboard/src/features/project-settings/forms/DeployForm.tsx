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
	FormLayout,
	ProjectField,
	type ProjectSettingsFormProps,
} from "@/features/project-settings/forms/FormLayout";
import {
	configPathValue,
	projectBindingValue,
	refreshOverview,
	setConfigPathValue,
	showSaveError,
} from "@/features/project-settings/forms/helpers";
import { useToast } from "@/hooks/useToast";
import type { ProjectDeployPatch } from "@/types/api";

const NO_DEPLOY_BACKEND_VALUE = "__no_deploy_backend__";

export const DeployForm: React.FC<React.PropsWithChildren<ProjectSettingsFormProps>> = ({
	project,
	revision,
	environment,
	workspaceEntryId,
	readOnly,
	onUpdated,
	onDirtyChange,
	children,
}) => {
	const { t } = useTranslation();
	const toast = useToast();
	const { mutate } = useSWRConfig();
	const catalog = useBackendCatalog();
	const settings = project.deploy;
	const staged = useManifestDraftStore(
		(state) => state.drafts[manifestDraftKey(workspaceEntryId)]?.changes[project.name]?.deploy,
	);
	const stageSection = useManifestDraftStore((state) => state.stageSection);
	const initialManifest: ProjectDeployPatch = {
		backend: settings.backend ?? "",
		config: settings.config ?? {},
	};
	const manifest = staged ?? initialManifest;
	const backendChanged = manifest.backend !== initialManifest.backend;
	const initialProfile = projectBindingValue(
		settings.selectedProfile,
		settings.profile,
		environment,
	);
	const profileBaseline = backendChanged ? "" : initialProfile;
	const [profile, setProfile] = useState(profileBaseline);
	const [saving, setSaving] = useState(false);
	const selected = manifest.backend ? catalog.byID.get(`deploy/${manifest.backend}`) : undefined;
	const backendFields = selected?.project?.fields ?? [];
	const manifestFields = backendFields.filter((field) => field.type !== "environment");
	const compatible = (catalog.byDomain.get("deploy") ?? []).filter((backend) =>
		(settings.compatibleTargets ?? []).includes(backend.name),
	);

	function updateManifest(next: ProjectDeployPatch) {
		const labels: Record<string, string> = { backend: "projectInspector.backend" };
		for (const backend of catalog.byDomain.get("deploy") ?? []) {
			for (const field of backend.project?.fields ?? []) {
				labels[`config/${field.path}`] = field.label_key;
			}
		}
		stageSection({
			entryId: workspaceEntryId,
			revision,
			project: project.name,
			section: "deploy",
			initial: initialManifest,
			next,
			labels,
		});
	}

	function selectBackend(value: string) {
		const backend = value === NO_DEPLOY_BACKEND_VALUE ? "" : value;
		updateManifest({ backend, config: {} });
		setProfile(backend === initialManifest.backend ? initialProfile : "");
		onDirtyChange(false);
	}

	async function saveProfile(nextProfile: string) {
		if (!manifest.backend || backendChanged || nextProfile === initialProfile || readOnly || saving)
			return;
		setProfile(nextProfile);
		onDirtyChange(true);
		setSaving(true);
		try {
			const next = await updateProjectProfileBinding(
				project.name,
				"deploy",
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
		<FormLayout title={t("projectInspector.deploy.title")}>
			<div
				data-testid="deployment-settings-grid"
				className="@container/backend-config space-y-3 rounded-[5px] border border-border bg-card p-3"
			>
				<div className="grid gap-x-4 gap-y-3 @3xl/backend-config:grid-cols-2">
					<ProjectField label={t("projectInspector.backend")} htmlFor="project-deploy-backend">
						<Select
							value={manifest.backend || NO_DEPLOY_BACKEND_VALUE}
							onValueChange={selectBackend}
							disabled={readOnly || compatible.length === 0}
						>
							<SelectTrigger id="project-deploy-backend">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value={NO_DEPLOY_BACKEND_VALUE}>
									{t("projects.matrix.notConfigured")}
								</SelectItem>
								{compatible.map((backend) => (
									<SelectItem key={backend.id} value={backend.name}>
										{humanizeBackendName(backend.name)}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</ProjectField>
					<ProfileBindingField
						id="project-profile-deploy"
						scope="project"
						directSource={environment ? "workspace-project-environment" : "workspace-project"}
						domain="deploy"
						backend={manifest.backend || undefined}
						binding={backendChanged ? undefined : settings.profile}
						value={profile}
						onChange={(value) => void saveProfile(value)}
						disabled={readOnly || saving || backendChanged}
						variant="embedded"
					/>
					{manifestFields.map((field) => {
						const label = t(field.label_key, {
							defaultValue: humanizeBackendName(field.input_name),
						});
						const value = configPathValue(manifest.config ?? {}, field.path);
						return (
							<ProjectField
								key={field.path}
								label={label}
								htmlFor={`project-deploy-${field.input_name}`}
							>
								<Input
									id={`project-deploy-${field.input_name}`}
									value={value === undefined || value === null ? "" : String(value)}
									placeholder={field.placeholder}
									onChange={(event) =>
										updateManifest({
											...manifest,
											config: setConfigPathValue(manifest.config, field.path, event.target.value),
										})
									}
									disabled={readOnly}
								/>
							</ProjectField>
						);
					})}
				</div>
				{backendFields.length === 0 && Object.keys(manifest.config ?? {}).length > 0 ? (
					<pre className="overflow-x-auto rounded-md border border-border bg-background/70 p-3 font-mono text-[10px] leading-relaxed text-muted-foreground">
						{JSON.stringify(manifest.config, null, 2)}
					</pre>
				) : null}
				{children ? <div className="border-t border-border pt-3">{children}</div> : null}
			</div>
		</FormLayout>
	);
};
