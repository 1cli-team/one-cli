import { Blocks } from "lucide-react";
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
		<section className="shrink-0 rounded-lg border border-border bg-card p-3">
			<div className="flex flex-wrap items-center justify-between gap-3">
				<div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-3 gap-y-2">
					<h2 className="flex shrink-0 items-center gap-2 text-sm font-semibold">
						<Blocks className="size-4 text-primary-text" aria-hidden="true" />
						{t("projectInspector.general.metadata")}
					</h2>
					{projectSelector}
					<p
						className="hidden max-w-36 shrink-0 truncate text-sm font-medium ud-md:block"
						title={project.name}
					>
						{project.name}
					</p>
					<dl className="flex min-w-0 basis-full items-baseline gap-2 ud-md:flex-1 ud-md:basis-auto">
						<dt className="shrink-0 text-xs text-muted-foreground">
							{t("projectInspector.general.path")}
						</dt>
						<dd className="min-w-0 truncate font-mono text-sm" title={project.relativeDir}>
							{project.relativeDir}
						</dd>
					</dl>
				</div>
				{actions}
			</div>
		</section>
	);
};
