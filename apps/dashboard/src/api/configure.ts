// api/configure.ts is the typed wrapper around the /api/configure*
// surface in internal/serve/handlers_configure.go. SWR consumers pass
// these as fetchers; mutations go through the explicit methods.

import http from "@/lib/http";
import type {
	CloudflareProfile,
	ConfigResponse,
	ContainerProfile,
	DotenvProfile,
	EdgeOneProfile,
	InfisicalProfile,
	KustomizeProfile,
	RemoveResponse,
	S3Profile,
	SectionResponse,
	UpsertResponse,
	UseResponse,
	VercelProfile,
} from "@/types/api";

// AnyTypedSection is the union the section-fetch funcs return — narrowed
// at the call site by the caller's choice of section.
export type ProfileByPair = {
	"env/infisical": InfisicalProfile;
	"env/dotenv": DotenvProfile;
	"deploy/aliyun-oss": S3Profile;
	"deploy/tencent-cos": S3Profile;
	"deploy/aws-s3": S3Profile;
	"deploy/minio": S3Profile;
	"deploy/rustfs": S3Profile;
	"deploy/r2": S3Profile;
	"deploy/kustomize": KustomizeProfile;
	"deploy/vercel": VercelProfile;
	"deploy/cloudflare": CloudflareProfile;
	"deploy/edgeone": EdgeOneProfile;
	"container/docker": ContainerProfile;
	"container/dockerhub": ContainerProfile;
	"container/ghcr": ContainerProfile;
	"container/acr": ContainerProfile;
};

export const configKey = "/configure";

export async function getConfig(reveal = false): Promise<ConfigResponse> {
	return http.get<ConfigResponse>("/configure", {
		params: reveal ? { reveal: 1 } : undefined,
	});
}

export function sectionKey(domain: string, backend: string, reveal = false): string {
	return `/configure/${domain}/${backend}` + (reveal ? "?reveal=1" : "");
}

export async function getSection<K extends keyof ProfileByPair>(
	domain: string,
	backend: string,
	reveal = false,
): Promise<SectionResponse<ProfileByPair[K]>> {
	return http.get<SectionResponse<ProfileByPair[K]>>(`/configure/${domain}/${backend}`, {
		params: reveal ? { reveal: 1 } : undefined,
	});
}

export async function upsertProfile<K extends keyof ProfileByPair>(
	domain: string,
	backend: string,
	body: { name: string; profile: ProfileByPair[K]; use?: boolean },
): Promise<UpsertResponse> {
	return http.post<UpsertResponse>(`/configure/${domain}/${backend}`, body);
}

export async function removeProfile(
	domain: string,
	backend: string,
	name: string,
): Promise<RemoveResponse> {
	return http.delete<RemoveResponse>(
		`/configure/${domain}/${backend}/${encodeURIComponent(name)}`,
	);
}

export async function setDefault(
	domain: string,
	backend: string,
	name: string,
): Promise<UseResponse> {
	return http.put<UseResponse>(`/configure/${domain}/${backend}/default`, { name });
}
