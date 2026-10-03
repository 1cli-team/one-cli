import type { ProjectSettings, ProjectSettingsResponse } from "@/types/api";

export interface ProjectSettingsFormProps {
	project: ProjectSettings;
	revision: string;
	environment: string;
	workspaceEntryId?: string;
	readOnly?: boolean;
	onUpdated(next: ProjectSettingsResponse): void;
	onDirtyChange(dirty: boolean): void;
}
