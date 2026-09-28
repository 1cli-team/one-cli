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
		devURL: project.devURL ?? "",
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
				devURL: "projectInspector.general.devURL",
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
					<ProjectField label={t("projectInspector.general.devURL")} htmlFor="project-dev-url">
						<Input
							id="project-dev-url"
							value={value.devURL ?? ""}
							placeholder="http://localhost:3000"
							readOnly={readOnly}
							onChange={(event) => update({ ...value, devURL: event.target.value })}
							aria-describedby="project-dev-url-hint"
						/>
						<p id="project-dev-url-hint" className="text-xs text-muted-foreground">
							{t("projectInspector.general.devURLHint")}
						</p>
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
			{project.tasks && (
				<section
					className="space-y-3 rounded-lg border border-border bg-card p-5"
					aria-label={t("projectInspector.general.tasks")}
				>
					<SectionHeading icon={Terminal} title={t("projectInspector.general.tasks")} />
					{project.tasks.status === "unavailable" ? (
						<p className="text-sm text-muted-foreground">
							{t("projectInspector.general.tasksUnavailable")}
						</p>
					) : project.tasks.entries.length === 0 ? (
						<p className="text-sm text-muted-foreground">
							{t("projectInspector.general.tasksEmpty")}
						</p>
					) : (
						project.tasks.entries.map((task) => (
							<div
								key={task.name}
								className="space-y-1 border-t border-border pt-3 text-sm break-words"
							>
								<p className="font-mono">{task.name}</p>
								<p className="text-muted-foreground">
									{t("projectInspector.general.taskSource", { source: task.source })}
								</p>
								<p>
									{t(
										task.cacheEnabled
											? "projectInspector.general.taskCacheOn"
											: "projectInspector.general.taskCacheOff",
									)}
								</p>
								{!!task.depends?.length && (
									<p>
										{t("projectInspector.general.taskDepends", { tasks: task.depends.join(", ") })}
									</p>
								)}
								{!!task.outputs?.length && (
									<p>
										{t("projectInspector.general.taskOutputs", {
											outputs: task.outputs.join(", "),
										})}
									</p>
								)}
							</div>
						))
					)}
				</section>
			)}
		</ManifestDraftLayout>
	);
};
