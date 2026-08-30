// api/workspace.ts exposes only read projections of one.manifest.json and
// machine-local Profile binding mutations. Dashboard writes must never carry
// manifest-owned backend or project fields.

import { workspaceBasePath } from "@/api/workspaces";
import http from "@/lib/http";
import type {
	BackendDomain,
	Overview,
	ProjectSettingsResponse,
	WorkspaceProfileSettings,
} from "@/types/api";

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

export function workspaceProfileBindingKey(entryId?: string, environment?: string): string {
	return withEnvironment(`${workspaceBasePath(entryId)}/profile-bindings/env`, environment);
}

export async function getWorkspaceProfileBinding(
	entryId?: string,
	environment?: string,
): Promise<WorkspaceProfileSettings> {
	return http.get<WorkspaceProfileSettings>(workspaceProfileBindingKey(entryId, environment));
}

export async function updateWorkspaceProfileBinding(
	profile: string,
	entryId?: string,
	environment?: string,
): Promise<WorkspaceProfileSettings> {
	return http.put<WorkspaceProfileSettings>(workspaceProfileBindingKey(entryId, environment), {
		profile,
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

export function projectProfileBindingKey(
	project: string,
	domain: BackendDomain,
	entryId?: string,
	environment?: string,
): string {
	return withEnvironment(
		`${projectBasePath(project, entryId)}/profile-bindings/${domain}`,
		environment,
	);
}

export async function updateProjectProfileBinding(
	project: string,
	domain: BackendDomain,
	profile: string,
	entryId?: string,
	environment?: string,
): Promise<ProjectSettingsResponse> {
	return http.put<ProjectSettingsResponse>(
		projectProfileBindingKey(project, domain, entryId, environment),
		{ profile },
	);
}
