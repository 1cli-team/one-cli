import http from "@/lib/http";
import type { WorkspacesResponse } from "@/types/api";

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
