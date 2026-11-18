import type { WorkspaceRegistryEntry } from "@/types/api";

export function isGitRepositoryWithoutManifest(workspace: WorkspaceRegistryEntry) {
	return workspace.status === "uninitialized" && Boolean(workspace.gitCommonDir);
}

export function workspaceDisplayStatus(workspace: WorkspaceRegistryEntry) {
	return isGitRepositoryWithoutManifest(workspace) ? "ready" : workspace.status;
}
