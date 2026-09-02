import { workspaceBasePath } from "@/api/workspaces";
import http from "@/lib/http";
import type { SecretListResponse, SecretMutationResponse, SecretValueResponse } from "@/types/api";

function secretSearch(environment: string, project?: string): string {
	const search = new URLSearchParams({ env: environment });
	if (project) search.set("project", project);
	return search.toString();
}

export function secretsKey(entryId: string | undefined, environment: string, project?: string) {
	return `${workspaceBasePath(entryId)}/secrets?${secretSearch(environment, project)}`;
}

export async function listSecrets(
	entryId: string | undefined,
	environment: string,
	project?: string,
) {
	return http.get<SecretListResponse>(secretsKey(entryId, environment, project));
}

function secretPath(
	entryId: string | undefined,
	environment: string,
	project: string | undefined,
	key: string,
) {
	return `${workspaceBasePath(entryId)}/secrets/${encodeURIComponent(key)}?${secretSearch(environment, project)}`;
}

export async function revealSecret(
	entryId: string | undefined,
	environment: string,
	project: string | undefined,
	key: string,
) {
	return http.get<SecretValueResponse>(secretPath(entryId, environment, project, key));
}

export async function createSecret(
	entryId: string | undefined,
	environment: string,
	project: string | undefined,
	key: string,
	value: string,
) {
	return http.post<SecretMutationResponse>(secretsKey(entryId, environment, project), {
		key,
		value,
	});
}

export async function updateSecret(
	entryId: string | undefined,
	environment: string,
	project: string | undefined,
	key: string,
	value: string,
) {
	return http.put<SecretMutationResponse>(secretPath(entryId, environment, project, key), {
		value,
	});
}

export async function deleteSecret(
	entryId: string | undefined,
	environment: string,
	project: string | undefined,
	key: string,
) {
	return http.delete<SecretMutationResponse>(secretPath(entryId, environment, project, key));
}
