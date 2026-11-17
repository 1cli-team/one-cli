export function workspacePath(entryId: string): string {
	return `/workspaces/${encodeURIComponent(entryId)}`;
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
