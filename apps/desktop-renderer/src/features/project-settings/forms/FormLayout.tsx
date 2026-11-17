import type { ProjectSettings } from "@/types/api";

export interface ProjectSettingsFormProps {
	project: ProjectSettings;
	environment: string;
	workspaceEntryId: string;
	readOnly?: boolean;
}
