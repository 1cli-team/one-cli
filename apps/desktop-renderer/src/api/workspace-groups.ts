import http from "@/lib/http";
import type { WorkspaceRegistryEntry } from "@/types/api";

export interface WorkspaceGroup {
	id: string;
	name: string;
	workspaceIds: string[];
	createdAt: string;
	updatedAt: string;
}

export interface WorkspaceGroupsResponse {
	schema: string;
	projects: WorkspaceProject[];
	groups: WorkspaceGroup[];
	workspaces: WorkspaceRegistryEntry[];
	ungroupedWorkspaceIds: string[];
}

export interface WorkspaceProject {
	id: string;
	name: string;
	kind: "workspace" | "group";
	workspaceIds: string[];
}

type WorkspaceGroupsPayload = Omit<WorkspaceGroupsResponse, "projects"> & {
	projects?: WorkspaceProject[] | null;
};

// Earlier API versions and SWR entries created before a hot update omit projects.
// Reconstruct the same projection from their groups and workspace registry.
export function normalizeWorkspaceGroups(data: WorkspaceGroupsPayload): WorkspaceGroupsResponse {
	const groups = data.groups ?? [];
	const workspaces = data.workspaces ?? [];
	const groupedIds = new Set(groups.flatMap((group) => group.workspaceIds));
	const standalone = workspaces.filter((workspace) => !groupedIds.has(workspace.entryId));
	const projects = data.projects ?? [
		...groups.map((group): WorkspaceProject => ({
			id: group.id,
			name: group.name,
			kind: "group",
			workspaceIds: group.workspaceIds,
		})),
		...standalone.map((workspace): WorkspaceProject => ({
			id: workspace.entryId,
			name: workspace.name,
			kind: "workspace",
			workspaceIds: [workspace.entryId],
		})),
	];
	return {
		...data,
		groups,
		workspaces,
		projects,
		ungroupedWorkspaceIds:
			data.ungroupedWorkspaceIds ?? standalone.map((workspace) => workspace.entryId),
	};
}

export const workspaceGroupsKey = "/workspace-groups";
export async function getWorkspaceGroups(): Promise<WorkspaceGroupsResponse> {
	return normalizeWorkspaceGroups(await http.get<WorkspaceGroupsPayload>(workspaceGroupsKey));
}
const groupURL = (id: string) => `${workspaceGroupsKey}/${encodeURIComponent(id)}`;
export const createWorkspaceGroup = (name: string) =>
	http.post<WorkspaceGroup>(workspaceGroupsKey, { name });
export const renameWorkspaceGroup = (id: string, name: string) =>
	http.put<WorkspaceGroup>(groupURL(id), { name });
export const deleteWorkspaceGroup = (id: string) => http.delete<void>(groupURL(id));
export const addGroupMembers = (id: string, workspaceIds: string[]) =>
	http.post<WorkspaceGroup>(`${groupURL(id)}/members`, { workspaceIds });
export const removeGroupMember = (id: string, workspaceId: string) =>
	http.delete<WorkspaceGroup>(`${groupURL(id)}/members/${encodeURIComponent(workspaceId)}`);
export const moveGroupMember = (id: string, workspaceId: string, before: string) =>
	http.post<WorkspaceGroup>(`${groupURL(id)}/members/${encodeURIComponent(workspaceId)}/move`, {
		before,
	});
