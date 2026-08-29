import { useMemo } from "react";
import useSWRImmutable from "swr/immutable";
import http from "@/lib/http";
import type { BackendDomain, BackendSpec, CatalogResponse, SectionKey } from "@/types/api";

export const catalogKey = "/catalog";
export const BACKEND_DOMAINS: readonly BackendDomain[] = ["env", "deploy", "container"];

const EMPTY_BACKENDS: readonly BackendSpec[] = [];

export async function getCatalog(): Promise<CatalogResponse> {
	return http.get<CatalogResponse>(catalogKey);
}

export function humanizeBackendName(name: string): string {
	return name
		.split("-")
		.filter(Boolean)
		.map((part) => {
			if (part.length <= 3) return part.toUpperCase();
			return part.charAt(0).toUpperCase() + part.slice(1);
		})
		.join(" ");
}

export function useBackendCatalog() {
	const result = useSWRImmutable(catalogKey, getCatalog);
	const backends = result.data?.backends ?? EMPTY_BACKENDS;
	const index = useMemo(() => {
		const byID = new Map<SectionKey, BackendSpec>();
		const byDomain = new Map<BackendDomain, BackendSpec[]>();
		for (const domain of BACKEND_DOMAINS) byDomain.set(domain, []);
		for (const backend of backends) {
			byID.set(backend.id, backend);
			byDomain.get(backend.domain)?.push(backend);
		}
		return {
			byID,
			byDomain,
			configurable: backends.filter((backend) => backend.profile.configurable),
		};
	}, [backends]);

	return { ...result, backends, ...index };
}
