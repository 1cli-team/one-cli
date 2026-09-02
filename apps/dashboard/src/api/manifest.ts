import { workspaceBasePath } from "@/api/workspaces";
import http from "@/lib/http";
import type {
	ApplyManifestRequest,
	ApplyManifestResponse,
	PreviewManifestRequest,
	PreviewManifestResponse,
} from "@/types/api";

export function manifestPath(entryId?: string): string {
	return `${workspaceBasePath(entryId)}/manifest`;
}

export async function applyManifestDraft(
	payload: ApplyManifestRequest,
	entryId?: string,
): Promise<ApplyManifestResponse> {
	return http.put<ApplyManifestResponse>(manifestPath(entryId), payload);
}

export async function previewManifestDraft(
	payload: PreviewManifestRequest,
	entryId?: string,
): Promise<PreviewManifestResponse> {
	return http.post<PreviewManifestResponse>(`${manifestPath(entryId)}/preview`, payload);
}
