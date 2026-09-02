import type React from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import { SecretsManager } from "@/features/secrets/SecretsManager";
import {
	ProjectField,
	type ProjectSettingsFormProps,
	ManifestDraftLayout,
	ReadOnlyDatum,
} from "@/features/project-settings/forms/FormLayout";
import type { ProjectGeneralPatch } from "@/types/api";

export const GeneralForm: React.FC<ProjectSettingsFormProps> = ({
	project,
	revision,
	environment,
	workspaceEntryId,
	readOnly,
}) => {
	const { t } = useTranslation();
	const staged = useManifestDraftStore(
		(state) => state.drafts[manifestDraftKey(workspaceEntryId)]?.changes[project.name]?.general,
	);
	const stageSection = useManifestDraftStore((state) => state.stageSection);
	const initial: ProjectGeneralPatch = {
		buildVersion: project.buildVersion ?? "",
		devCommand: project.devCommand ?? "",
	};
	const value = staged ?? initial;

	function update(next: ProjectGeneralPatch) {
		stageSection({
			entryId: workspaceEntryId,
			revision,
			project: project.name,
			section: "general",
			initial,
			next,
			labels: {
				buildVersion: "projectInspector.general.buildVersion",
				devCommand: "projectInspector.general.devCommand",
			},
		});
	}

	return (
		<ManifestDraftLayout>
			<div className="grid gap-4 rounded-lg border border-border bg-muted/25 p-4 sm:grid-cols-4">
				<ReadOnlyDatum label={t("projectInspector.general.template")} value={project.templateId} />
				<ReadOnlyDatum label={t("projectInspector.general.toolchain")} value={project.toolchain} />
				<ReadOnlyDatum
					label={t("projectInspector.general.packageManager")}
					value={project.packageManager}
				/>
				<ReadOnlyDatum
					label={t("projectInspector.general.path")}
					value={project.relativeDir}
					mono
				/>
			</div>
			<div className="grid gap-4 rounded-lg border border-border bg-card p-4 sm:grid-cols-2">
				<ProjectField
					label={t("projectInspector.general.buildVersion")}
					htmlFor="project-build-version"
				>
					<Input
						id="project-build-version"
						value={value.buildVersion}
						onChange={(event) => update({ ...value, buildVersion: event.target.value })}
						disabled={readOnly}
					/>
				</ProjectField>
				<ProjectField
					label={t("projectInspector.general.devCommand")}
					htmlFor="project-dev-command"
				>
					<Input
						id="project-dev-command"
						className="font-mono"
						value={value.devCommand}
						onChange={(event) => update({ ...value, devCommand: event.target.value })}
						disabled={readOnly}
					/>
				</ProjectField>
			</div>
			{project.environment.backend === "infisical" ? (
				<SecretsManager
					workspaceEntryId={workspaceEntryId}
					environment={environment}
					fixedProject={project.name}
					variant="embedded"
					readOnly={readOnly}
				/>
			) : null}
		</ManifestDraftLayout>
	);
};
