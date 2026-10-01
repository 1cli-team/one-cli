// api/workspace.ts exposes Manifest projections, env Backend workflows, and
// single-account environment configuration. Reviewed Manifest publication lives
// in api/manifest.ts; remote secret operations live in api/secrets.ts.

import { workspaceBasePath } from "@/api/workspaces";
import http from "@/lib/http";
import type { Overview, ProjectSettingsResponse, WorkspaceEnvironmentSettings } from "@/types/api";

export const overviewKey = "/workspace/overview";

function withEnvironment(path: string, environment?: string): string {
	const selected = environment?.trim();
	if (!selected) return path;
	const search = new URLSearchParams({ env: selected });
	return `${path}?${search.toString()}`;
}

export function overviewKeyFor(entryId?: string, environment?: string): string {
	return withEnvironment(`${workspaceBasePath(entryId)}/overview`, environment);
}

export async function getOverview(entryId?: string, environment?: string): Promise<Overview> {
	return http.get<Overview>(overviewKeyFor(entryId, environment));
}

export function workspaceEnvironmentKey(entryId?: string, environment?: string): string {
	return withEnvironment(`${workspaceBasePath(entryId)}/environment`, environment);
}

export async function getWorkspaceEnvironment(
	entryId?: string,
	environment?: string,
): Promise<WorkspaceEnvironmentSettings> {
	return http.get<WorkspaceEnvironmentSettings>(workspaceEnvironmentKey(entryId, environment));
}

export interface WorkspaceBinding {
	project_id: string;
	project_name?: string;
	created: boolean;
	requested_name?: string;
	environments: string[];
}

export function bindWorkspaceEnvironment(
	entryId: string | undefined,
	input: { revision: string; create: boolean; projectId?: string },
): Promise<WorkspaceBinding> {
	return http.post<WorkspaceBinding>(`${workspaceBasePath(entryId)}/environment/bind`, input, {
		timeout: 120000,
	});
}

function projectBasePath(project: string, entryId?: string): string {
	return `${workspaceBasePath(entryId)}/projects/${encodeURIComponent(project)}`;
}

export function projectSettingsKey(
	project: string,
	entryId?: string,
	environment?: string,
): string {
	return withEnvironment(projectBasePath(project, entryId), environment);
}

export async function getProjectSettings(
	project: string,
	entryId?: string,
	environment?: string,
): Promise<ProjectSettingsResponse> {
	return http.get<ProjectSettingsResponse>(projectSettingsKey(project, entryId, environment));
}
