import { Blocks, Terminal } from "lucide-react";
import { SectionHeading } from "@/components/ui/page-layout";
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
	const build = project.build;

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
			<div className="space-y-5 rounded-lg border border-border bg-card p-5">
				<SectionHeading icon={Blocks} title={t("projectInspector.general.metadata")} />
				<div className="grid gap-x-6 gap-y-5 ud-sm:grid-cols-2 @xl:grid-cols-4">
					<ReadOnlyDatum
						label={t("projectInspector.general.template")}
						value={project.templateId}
					/>
					<ReadOnlyDatum
						label={t("projectInspector.general.toolchain")}
						value={project.toolchain}
					/>
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
			</div>
			<div className="space-y-5 rounded-lg border border-border bg-card p-5">
				<SectionHeading
					icon={Terminal}
					title={t("projectInspector.general.runtime")}
					description={t("projectInspector.draftHint")}
				/>
				<div className="grid gap-x-6 gap-y-5 @xl:grid-cols-2">
					<ProjectField
						label={t("projectInspector.general.buildVersion")}
						htmlFor="project-build-version"
					>
						<Input
							id="project-build-version"
							value={value.buildVersion}
							onChange={(event) => update({ ...value, buildVersion: event.target.value })}
							readOnly={readOnly}
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
							readOnly={readOnly}
						/>
					</ProjectField>
					<div className="@xl:col-span-2">
						<ProjectField
							label={t("projectInspector.general.buildCommand")}
							htmlFor="project-build-command"
						>
							<Input
								id="project-build-command"
								className="font-mono"
								value={build?.command ?? ""}
								placeholder={t(
									build?.status === "missing"
										? "projectInspector.general.buildMissing"
										: "projectInspector.general.buildUnavailable",
								)}
								readOnly
								aria-describedby="project-build-command-hint"
							/>
							<p id="project-build-command-hint" className="text-xs text-muted-foreground">
								{build?.source
									? t("projectInspector.general.buildSource", { source: build.source })
									: t("projectInspector.general.buildUnsupported")}
							</p>
						</ProjectField>
					</div>
				</div>
			</div>
		</ManifestDraftLayout>
	);
};
