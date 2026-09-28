import { workspaceBasePath } from "@/api/workspaces";
import http from "@/lib/http";
import type { DevService } from "@/types/api";

export function servicePath(project: string, entryId?: string) {
	return `${workspaceBasePath(entryId)}/projects/${encodeURIComponent(project)}/service`;
}
export function controlService(
	project: string,
	entryId: string | undefined,
	action: "start" | "stop" | "restart",
	environment: string,
) {
	return http.post<DevService>(`${servicePath(project, entryId)}/${action}`, { environment });
}
export function watchService(
	project: string,
	entryId: string | undefined,
	receive: (state: DevService) => void,
	connection: (connected: boolean) => void,
) {
	const source = new EventSource(`/api${servicePath(project, entryId)}/events`);
	source.onmessage = (event) => {
		receive(JSON.parse(event.data) as DevService);
		connection(true);
	};
	source.onerror = () => connection(false);
	return () => source.close();
}

export function servicesKey(entryId?: string) {
	return `${workspaceBasePath(entryId)}/services`;
}
export function getServices(entryId?: string) {
	return http.get<{ services: DevService[] }>(servicesKey(entryId));
}
