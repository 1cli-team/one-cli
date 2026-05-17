// api/workspace.ts is the typed wrapper around /api/workspace/* in
// internal/serve/handlers_workspace.go (read) and
// handlers_workspace_mutate.go (write). The home page reads getOverview()
// and renders the Overview page when `present: true`; the mutate helpers
// patch one.manifest.json so the missing-config dialogs can fix issues
// in-place. Every mutate call returns the freshly-rebuilt Overview so the
// caller can drop it straight into the SWR cache.

import http from "@/lib/http";
import type { Overview } from "@/types/api";

export const overviewKey = "/workspace/overview";

export async function getOverview(): Promise<Overview> {
	return http.get<Overview>("/workspace/overview");
}

// setWorkspaceEnv selects the workspace-level env backend kind
// (manifest.domains.env.kind). kind is "dotenv" | "infisical".
export async function setWorkspaceEnv(kind: string): Promise<Overview> {
	return http.put<Overview>("/workspace/domains/env", { kind });
}

// setProjectDeploy selects a project's deploy backend kind
// (manifest.projects[].domains.deploy.kind).
export async function setProjectDeploy(project: string, kind: string): Promise<Overview> {
	return http.put<Overview>(`/workspace/projects/${encodeURIComponent(project)}/deploy`, {
		kind,
	});
}

// setProjectContainer enables container builds for a project and sets the
// backend kind (manifest.projects[].domains.container). An empty kind is
// allowed — it inherits the workspace default / "docker".
export async function setProjectContainer(
	project: string,
	body: { kind?: string; image?: string },
): Promise<Overview> {
	return http.put<Overview>(`/workspace/projects/${encodeURIComponent(project)}/container`, body);
}
