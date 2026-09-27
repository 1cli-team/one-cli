import { http } from "@/lib/http";
export interface SessionInfo {
	loggedIn: boolean;
	expired: boolean;
	email?: string;
	userId?: string;
	siteUrl?: string;
	organizationId?: string;
	expiresAt?: string;
}
export interface SessionState {
	session: SessionInfo;
	login?: { status: "waiting" | "complete" | "failed"; url: string };
	error?: string;
}
export const sessionKey = "/session";
export const getSession = () => http.get<SessionState>(sessionKey);
export const startLogin = (siteUrl: string) =>
	http.post<{ url: string }>("/session/login", { siteUrl });
export const cancelLogin = () => http.delete("/session/login");
export const logout = () => http.delete("/session");
export interface RemoteProject {
	id: string;
	name: string;
	orgId: string;
	environments: { name: string; slug: string }[];
}
export const getProjects = () => http.get<RemoteProject[]>("/infisical/projects");
export const createRemoteProject = (name: string) =>
	http.post<RemoteProject>("/infisical/projects", { name }, { timeout: 120000 });
export const getProject = (id: string) =>
	http.get<RemoteProject>(`/infisical/projects/${encodeURIComponent(id)}`);
export interface GlobalLocation {
	siteUrl: string;
	userId: string;
	organizationId: string;
	projectId: string;
	projectName: string;
	defaultEnvironment: string;
}
export interface GlobalListing {
	location: GlobalLocation;
	environment: string;
	path: string;
	folders: string[];
	variables: { key: string; description?: string }[];
}
export const locationKey = "/global-env/location";
export const getLocation = () => http.get<{ location: GlobalLocation | null }>(locationKey);
export const bindLocation = (projectId: string, environment: string) =>
	http.put<{ location: GlobalLocation }>(locationKey, { projectId, environment });
export const initializeGlobalLocation = () =>
	http.post<{ location: GlobalLocation }>(`${locationKey}/default`, {}, { timeout: 120000 });
export const globalQuery = (environment: string, path: string) =>
	`?${new URLSearchParams({ env: environment, path })}`;
export const getGlobalListing = (query: string) =>
	http.get<GlobalListing>(`/global-env/secrets${query}`);
export const readGlobalSecret = (key: string, query: string) =>
	http.get<{ value: string }>(`/global-env/secrets/${encodeURIComponent(key)}${query}`);
export const saveGlobalSecret = (key: string, value: string, query: string, existing: boolean) =>
	http[existing ? "put" : "post"](`/global-env/secrets/${encodeURIComponent(key)}${query}`, {
		value,
	});
export const deleteGlobalSecret = (key: string, query: string) =>
	http.delete(`/global-env/secrets/${encodeURIComponent(key)}${query}`);
export const createGlobalFolder = (name: string, query: string) =>
	http.post(`/global-env/folders${query}`, { name });
export function message(error: unknown): string {
	return error && typeof error === "object" && "message" in error
		? String(error.message)
		: String(error);
}
