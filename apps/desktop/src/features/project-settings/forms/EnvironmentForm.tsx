import { SecretsManager } from "@/features/secrets/SecretsManager";
import type React from "react";
import type { ProjectSettingsFormProps } from "@/features/project-settings/forms/FormLayout";

export const EnvironmentForm: React.FC<ProjectSettingsFormProps> = ({
	project,
	environment,
	workspaceEntryId,
	readOnly,
}) => {
	return (
		<section className="flex min-h-0 flex-1 flex-col">
			{project.environment.backend === "infisical" && (
				<SecretsManager
					workspaceEntryId={workspaceEntryId}
					environment={environment}
					fixedProject={project.name}
					variant="embedded"
					readOnly={readOnly}
				/>
			)}
		</section>
	);
};
