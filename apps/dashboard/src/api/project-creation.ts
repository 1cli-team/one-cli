import { workspaceBasePath } from "@/api/workspaces";
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
	input: { name: string; templateId: string },
): Promise<CreatedProject> {
	return http.post(`${workspaceBasePath(entryId)}/projects`, input, { timeout: 60000 });
}
