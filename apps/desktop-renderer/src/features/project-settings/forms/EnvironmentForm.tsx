import { SecretsManager } from "@/features/secrets/SecretsManager";
import { KeyRound } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { StatePanel } from "@/components/ui/page-layout";
import type { ProjectSettingsFormProps } from "@/features/project-settings/forms/FormLayout";

export const EnvironmentForm: React.FC<ProjectSettingsFormProps> = ({
	project,
	environment,
	workspaceEntryId,
	readOnly,
}) => {
	const { t } = useTranslation();
	return (
		<section className="flex min-h-0 flex-1 flex-col">
			{project.environment.backend === "infisical" ? (
				<SecretsManager
					workspaceEntryId={workspaceEntryId}
					environment={environment}
					fixedProject={project.name}
					variant="embedded"
					readOnly={readOnly}
				/>
			) : (
				<StatePanel
					icon={KeyRound}
					title={t("secrets.unavailableTitle")}
					description={t("secrets.unavailableDescription")}
					className="rounded-lg border border-border bg-card"
				/>
			)}
		</section>
	);
};
