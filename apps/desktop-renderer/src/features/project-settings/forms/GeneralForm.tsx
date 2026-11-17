import type React from "react";
import { useTranslation } from "react-i18next";
import type { ProjectSettingsFormProps } from "@/features/project-settings/forms/FormLayout";

export const GeneralForm: React.FC<
	Pick<ProjectSettingsFormProps, "project"> & {
		projectSelector?: React.ReactNode;
		actions?: React.ReactNode;
	}
> = ({ project, projectSelector, actions }) => {
	const { t } = useTranslation();

	return (
		<header className="flex shrink-0 flex-wrap items-start justify-between gap-4">
			<div className="flex min-w-0 flex-1 flex-col gap-2">
				<h1 className="break-all text-2xl font-medium leading-8">{project.name}</h1>
				{projectSelector}
				<dl className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1">
					<dt className="shrink-0 text-xs text-muted-foreground">
						{t("projectInspector.general.path")}
					</dt>
					<dd
						className="min-w-0 break-all font-mono text-sm text-muted-foreground"
						title={project.relativeDir}
					>
						{project.relativeDir}
					</dd>
				</dl>
			</div>
			{actions}
		</header>
	);
};
