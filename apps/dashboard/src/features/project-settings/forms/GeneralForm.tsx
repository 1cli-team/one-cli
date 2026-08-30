import type React from "react";
import { useTranslation } from "react-i18next";
import {
	type ProjectSettingsFormProps,
	ReadOnlyDatum,
	ReadOnlyLayout,
} from "@/features/project-settings/forms/FormLayout";

export const GeneralForm: React.FC<ProjectSettingsFormProps> = ({ project }) => {
	const { t } = useTranslation();

	return (
		<ReadOnlyLayout
			title={t("projectInspector.general.title")}
			description={t("projectInspector.general.description")}
		>
			<div className="grid grid-cols-2 gap-x-5 gap-y-4 rounded-lg border border-border bg-muted/25 p-4">
				<ReadOnlyDatum label={t("projectInspector.general.template")} value={project.templateId} />
				<ReadOnlyDatum label={t("projectInspector.general.toolchain")} value={project.toolchain} />
				<ReadOnlyDatum
					label={t("projectInspector.general.buildVersion")}
					value={project.buildVersion}
				/>
				<ReadOnlyDatum
					label={t("projectInspector.general.packageManager")}
					value={project.packageManager}
				/>
				<div className="col-span-2">
					<ReadOnlyDatum
						label={t("projectInspector.general.path")}
						value={project.relativeDir}
						mono
					/>
				</div>
				<div className="col-span-2">
					<ReadOnlyDatum
						label={t("projectInspector.general.devCommand")}
						value={project.devCommand}
						mono
					/>
				</div>
			</div>
		</ReadOnlyLayout>
	);
};
