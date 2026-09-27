import { SecretsManager } from "@/features/secrets/SecretsManager";
import { FileKey2 } from "lucide-react";
import { SectionHeading } from "@/components/ui/page-layout";
import type React from "react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import {
	ManifestDraftLayout,
	ProjectField,
	SwitchField,
	type ProjectSettingsFormProps,
} from "@/features/project-settings/forms/FormLayout";
import type { ProjectEnvironmentPatch } from "@/types/api";

export const EnvironmentForm: React.FC<ProjectSettingsFormProps> = ({
	project,
	revision,
	environment,
	workspaceEntryId,
	readOnly,
}) => {
	const { t } = useTranslation();
	const settings = project.environment;
	const staged = useManifestDraftStore(
		(state) => state.drafts[manifestDraftKey(workspaceEntryId)]?.changes[project.name]?.environment,
	);
	const stageSection = useManifestDraftStore((state) => state.stageSection);
	const initialManifest: ProjectEnvironmentPatch = {
		path: settings.path ?? "",
		inherits: settings.inherits,
		disabled: settings.disabled,
	};
	const manifest = staged ?? initialManifest;

	function updateManifest(next: ProjectEnvironmentPatch) {
		stageSection({
			entryId: workspaceEntryId,
			revision,
			project: project.name,
			section: "environment",
			initial: initialManifest,
			next,
			labels: {
				path: "projectInspector.environment.path",
				inherits: "projectInspector.environment.inherits",
				disabled: "projectInspector.environment.disabled",
			},
		});
	}

	return (
		<ManifestDraftLayout>
			<div className="space-y-3">
				<div
					data-testid="environment-settings-grid"
					className="@container/backend-config space-y-4 rounded-lg border border-border bg-card p-5"
				>
					<SectionHeading
						icon={FileKey2}
						title={t("projectInspector.environment.title")}
						description={t("projectInspector.draftHint")}
					/>
					<ProjectField label={t("projectInspector.environment.path")} htmlFor="project-env-path">
						<Input
							id="project-env-path"
							className="font-mono"
							value={manifest.path}
							onChange={(event) => updateManifest({ ...manifest, path: event.target.value })}
							readOnly={readOnly}
						/>
					</ProjectField>
					<div className="grid gap-4 ud-lg:grid-cols-2">
						<SwitchField
							id="project-env-inherits"
							label={t("projectInspector.environment.inherits")}
							description={t("projectInspector.environment.inheritsHint")}
							checked={manifest.inherits}
							onChange={(inherits) => updateManifest({ ...manifest, inherits })}
							disabled={readOnly}
						/>
						<SwitchField
							id="project-env-disabled"
							label={t("projectInspector.environment.disabled")}
							description={t("projectInspector.environment.disabledHint")}
							checked={manifest.disabled}
							onChange={(disabled) => updateManifest({ ...manifest, disabled })}
							disabled={readOnly}
						/>
					</div>
				</div>

				{(settings.keys?.length ?? 0) > 0 ? (
					<div>
						<Label>{t("projectInspector.environment.keys")}</Label>
						<div className="mt-1.5 flex flex-wrap gap-1.5 rounded-[5px] border border-border bg-card p-2.5">
							{settings.keys?.map((key) => (
								<Badge key={key} variant="outline" className="font-mono">
									{key}
								</Badge>
							))}
						</div>
					</div>
				) : null}
			</div>
			{settings.backend === "infisical" ? (
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
