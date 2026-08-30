import http from "@/lib/http";
import type { WorkspacesResponse } from "@/types/api";

export const workspacesKey = "/workspaces";

export async function getWorkspaces(): Promise<WorkspacesResponse> {
	return http.get<WorkspacesResponse>(workspacesKey);
}

export async function forgetWorkspace(entryId: string): Promise<void> {
	await http.delete(`/workspaces/${encodeURIComponent(entryId)}`);
}

// The singular path remains the compatibility surface for the workspace
// `one serve` was launched in. A registry entry selects the stateless plural
// route, so separate tabs can inspect different workspaces without sharing a
// mutable server-side "current workspace".
export function workspaceBasePath(entryId?: string): string {
	return entryId ? `/workspaces/${encodeURIComponent(entryId)}` : "/workspace";
}
