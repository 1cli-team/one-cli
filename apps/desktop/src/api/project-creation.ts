import { invoke } from "@tauri-apps/api/core";
import { useDesktop, native } from "@/lib/desktop/runtime";
import i18n from "@/lib/i18n";
import http from "@/lib/http";

export interface ProjectTemplate {
	id: string;
	name: string;
	description: string;
	category: "frontend" | "backend" | "library";
	directory: string;
	toolchain: "node" | "go" | "none";
	projects?: { directory: string; suffix: string }[];
}

export interface CreatedProject {
	name: string;
	relativeDir: string;
	templateId: string;
	warnings?: string[];
	projects?: CreatedProject[];
}

export const projectTemplatesKey = "/project-templates";
export async function getProjectTemplates(): Promise<{ templates: ProjectTemplate[] }> {
	return http.get(projectTemplatesKey);
}

export async function createProject(
	entryId: string,
	input: { name: string; templateId: string; skipSkills?: boolean },
): Promise<CreatedProject> {
	if (!native) throw new Error(i18n.t("server.localCreationRequired"));
	return invoke<CreatedProject>("local_operation", {
		operation: "createProject",
		payload: { workspaceId: entryId, serverURL: useDesktop.getState().serverURL, ...input },
	});
}
