import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useSWRConfig } from "swr";
import { humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import { updateProjectProfileBinding } from "@/api/workspace";
import { Badge } from "@/components/ui/badge";
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
const DEFAULT_DEPLOY_ENV_VALUE = "__default_deploy_environment__";

export const DeployForm: React.FC<ProjectSettingsFormProps> = ({
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
	const settings = project.deploy;
	const initialProfile = projectBindingValue(
		settings.selectedProfile,
		settings.profile,
		environment,
	);
	const [profile, setProfile] = useState(initialProfile);
	const [saving, setSaving] = useState(false);
	const staged = useManifestDraftStore(
		(state) => state.drafts[manifestDraftKey(workspaceEntryId)]?.changes[project.name]?.deploy,
	);
	const stageSection = useManifestDraftStore((state) => state.stageSection);
	const initialManifest: ProjectDeployPatch = {
		backend: settings.backend ?? "",
		config: settings.config ?? {},
	};
	const manifest = staged ?? initialManifest;
	const selected = manifest.backend ? catalog.byID.get(`deploy/${manifest.backend}`) : undefined;
	const manifestFields = selected?.project?.fields ?? [];
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

			<div className="space-y-4 rounded-lg border border-warning-border/70 bg-warning-surface/35 p-4">
				<div className="grid grid-cols-2 gap-x-5 gap-y-4">
					<ProjectField label={t("projectInspector.backend")} htmlFor="project-deploy-backend">
						<Select
							value={manifest.backend || NO_DEPLOY_BACKEND_VALUE}
							onValueChange={(backend) =>
								updateManifest({
									backend: backend === NO_DEPLOY_BACKEND_VALUE ? "" : backend,
									config: {},
								})
							}
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
					<div>
						<p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
							{t("projectInspector.deploy.compatibleTargets")}
						</p>
						<div className="mt-2 flex flex-wrap gap-1">
							{settings.compatibleTargets?.map((target) => (
								<Badge key={target} variant="outline" className="font-mono text-[11px]">
									{target}
								</Badge>
							))}
						</div>
					</div>
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
								{field.type === "environment" ? (
									<Select
										value={
											value === undefined || value === null || value === ""
												? DEFAULT_DEPLOY_ENV_VALUE
												: String(value)
										}
										onValueChange={(environmentName) =>
											updateManifest({
												...manifest,
												config: setConfigPathValue(
													manifest.config,
													field.path,
													environmentName === DEFAULT_DEPLOY_ENV_VALUE ? "" : environmentName,
												),
											})
										}
										disabled={readOnly}
									>
										<SelectTrigger id={`project-deploy-${field.input_name}`}>
											<SelectValue />
										</SelectTrigger>
										<SelectContent>
											<SelectItem value={DEFAULT_DEPLOY_ENV_VALUE}>
												{t("projectInspector.deploy.environmentDefault")}
											</SelectItem>
											{project.availableEnvironments?.map((environmentName) => (
												<SelectItem key={environmentName} value={environmentName}>
													{environmentName}
												</SelectItem>
											))}
										</SelectContent>
									</Select>
								) : (
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
								)}
							</ProjectField>
						);
					})}
				</div>
				{manifestFields.length === 0 && Object.keys(manifest.config ?? {}).length > 0 ? (
					<pre className="overflow-x-auto rounded-md border border-border bg-background/70 p-3 font-mono text-[10px] leading-relaxed text-muted-foreground">
						{JSON.stringify(manifest.config, null, 2)}
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
