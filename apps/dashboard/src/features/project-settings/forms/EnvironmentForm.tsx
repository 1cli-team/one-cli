import { SecretsManager } from "@/features/secrets/SecretsManager";
import { FileKey2 } from "lucide-react";
import { SectionHeading } from "@/components/ui/page-layout";
import type React from "react";
import { useTranslation } from "react-i18next";
import {
	ManifestDraftLayout,
	ReadOnlyDatum,
	type ProjectSettingsFormProps,
} from "@/features/project-settings/forms/FormLayout";

export const EnvironmentForm: React.FC<ProjectSettingsFormProps> = ({
	project,
	environment,
	workspaceEntryId,
	readOnly,
}) => {
	const { t } = useTranslation();
	return (
		<ManifestDraftLayout>
			<div
				data-testid="environment-settings-grid"
				className="space-y-4 rounded-lg border border-border bg-card p-5"
			>
				<SectionHeading
					icon={FileKey2}
					title={t("projectInspector.environment.title")}
					description={t("projectInspector.environment.conventionHint")}
				/>
				<ReadOnlyDatum
					label={t("projectInspector.environment.path")}
					value={project.environment.path}
					mono
				/>
			</div>
			{project.environment.backend === "infisical" && (
				<SecretsManager
					workspaceEntryId={workspaceEntryId}
					environment={environment}
					fixedProject={project.name}
					variant="embedded"
					readOnly={readOnly}
				/>
			)}
		</ManifestDraftLayout>
	);
};
