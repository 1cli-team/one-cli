export function workspaceContext(entryId: string, worktreeId?: string): string {
	return worktreeId ? `${entryId.split("~")[0]}~${worktreeId}` : entryId;
}

export function workspacePath(entryId: string, worktreeId?: string): string {
	const [logicalId, selected] = workspaceContext(entryId, worktreeId).split("~");
	const base = `/workspaces/${encodeURIComponent(logicalId)}`;
	return selected ? `${base}/worktrees/${encodeURIComponent(selected)}` : base;
}

export function workspaceVariablesPath(entryId: string, projectName?: string): string {
	const base = `${workspacePath(entryId)}/environment-variables`;
	return projectName ? `${base}/${encodeURIComponent(projectName)}` : base;
}

export function legacyWorkspaceLocation(entryId: string, search: string) {
	const parameters = new URLSearchParams(search);
	const project = parameters.get("project");
	parameters.delete("project");
	parameters.delete("tab");
	return {
		pathname: workspaceVariablesPath(entryId, project || undefined),
		search: parameters.size ? `?${parameters}` : "",
	};
}

export function groupPath(groupId: string): string {
	return `/projects/${encodeURIComponent(groupId)}`;
}

export function groupWorkspacePath(groupId: string, entryId: string, worktreeId?: string): string {
	return `${workspacePath(entryId, worktreeId)}?group=${encodeURIComponent(groupId)}`;
}
