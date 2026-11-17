import http from "@/lib/http";
import type { WorkspacesResponse, WorkspaceRegistryEntry } from "@/types/api";

export const workspacesKey = "/workspaces";

export async function getWorkspaces(): Promise<WorkspacesResponse> {
	return http.get<WorkspacesResponse>(workspacesKey);
}

export async function forgetWorkspace(entryId: string): Promise<void> {
	await http.delete(`/workspaces/${encodeURIComponent(entryId)}`);
}

// Each request selects a workspace by its opaque registry entry ID.
export function workspaceBasePath(entryId: string): string {
	return `/workspaces/${encodeURIComponent(entryId)}`;
}

export function openLocalWorkspace(path: string) {
	return http.post<WorkspaceRegistryEntry>(workspacesKey, { path });
}

export function initializeWorkspace(entryId: string, name: string) {
	return http.post<WorkspaceRegistryEntry>(`${workspaceBasePath(entryId)}/initialize`, { name });
}

export function relocateWorkspace(entryId: string, path: string) {
	return http.put<WorkspaceRegistryEntry>(`${workspaceBasePath(entryId)}/location`, { path });
}
