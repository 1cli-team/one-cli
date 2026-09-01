import type React from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
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
		<ManifestDraftLayout
			title={t("projectInspector.general.title")}
			description={t("projectInspector.general.description")}
		>
			<div className="grid grid-cols-2 gap-x-5 gap-y-4 rounded-lg border border-border bg-muted/25 p-4">
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
			<div className="grid grid-cols-2 gap-4 rounded-lg border border-warning-border/70 bg-warning-surface/35 p-4">
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
		</ManifestDraftLayout>
	);
};
