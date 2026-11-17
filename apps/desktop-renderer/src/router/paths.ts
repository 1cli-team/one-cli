export function workspaceContext(entryId: string, worktreeId?: string): string {
	return worktreeId ? `${entryId.split("~")[0]}~${worktreeId}` : entryId;
}

export function workspacePath(entryId: string, worktreeId?: string): string {
	const [logicalId, selected] = workspaceContext(entryId, worktreeId).split("~");
	const base = `/workspaces/${encodeURIComponent(logicalId)}`;
	return selected ? `${base}/worktrees/${encodeURIComponent(selected)}` : base;
}

export function projectPath(entryId: string, projectName: string): string {
	return `${workspacePath(entryId)}/projects/${encodeURIComponent(projectName)}`;
}

export function legacyWorkspaceLocation(entryId: string, search: string) {
	const parameters = new URLSearchParams(search);
	const project = parameters.get("project");
	parameters.delete("project");
	parameters.delete("tab");
	return {
		pathname: project ? projectPath(entryId, project) : workspacePath(entryId),
		search: parameters.size ? `?${parameters}` : "",
	};
}
