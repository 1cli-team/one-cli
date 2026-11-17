import type { CodexOptions } from "@/features/gateway/api";
import http from "@/lib/http";
import type { HttpError } from "@/types/api";

export interface Named {
	id: string;
	name: string;
}
export interface Scope {
	kind: "group" | "workspace";
	id: string;
}
export interface PageInfo {
	endCursor: string;
	hasNextPage: boolean;
}
export interface Page<T> {
	nodes: T[];
	pageInfo: PageInfo;
}
export interface Connection extends Named {
	organizationId: string;
	organizationName: string;
	canCreateProjects: boolean;
}
export interface OAuthConfig {
	clientId: string;
	redirectUri: string;
}
export interface Authorization {
	id: string;
	url: string;
	status: string;
	errorCode?: string;
}
export interface ConnectionState {
	config: OAuthConfig;
	connections: Connection[];
	authorization?: Authorization;
}
export interface Project extends Named {
	description: string;
	url: string;
	teams: Named[];
}
export interface State extends Named {
	type: string;
	position: number;
}
export interface Team extends Named {
	states: State[];
	members: Named[];
	labels: Named[];
}
export interface Binding {
	scope: Scope;
	connectionId: string;
	organizationId: string;
	project: Project;
	teamId: string;
}
export interface Comment {
	id: string;
	body: string;
	createdAt: string;
	user: Named | null;
}
export interface Issue {
	id: string;
	identifier: string;
	title: string;
	description: string;
	url: string;
	updatedAt: string;
	priority: number;
	team: Named;
	project: Named;
	state: State;
	assignee: Named | null;
	labels: Named[];
	comments: Comment[];
	commentsPageInfo: PageInfo;
}
export interface IssueInput {
	title: string;
	description: string;
	teamId: string;
	stateId: string;
	assigneeId: string;
	priority: number;
	labelIds: string[];
	expectedUpdatedAt?: string;
	operationId?: string;
}
export interface IssuePage extends Page<Issue> {
	stale: boolean;
	fetchedAt: string;
}
export interface Operation {
	id: string;
	kind: string;
	status: string;
	resourceId: string;
}

export const connectionKey = "/linear/connections";
export const projectKey = (connection: string, project: string) =>
	`/issue-tracking/projects/${encodeURIComponent(project)}?${new URLSearchParams({ connection })}`;
export const teamKey = (connection: string, team: string) =>
	`/issue-tracking/teams/${encodeURIComponent(team)}?${new URLSearchParams({ connection })}`;
export const teamsKey = (connection: string) =>
	`/issue-tracking/teams?${new URLSearchParams({ connection })}`;
export function scopeQuery(scope: Scope, values: Record<string, string> = {}) {
	const query = new URLSearchParams({ kind: scope.kind, scope: scope.id });
	for (const [key, value] of Object.entries(values)) if (value !== "") query.set(key, value);
	return query.toString();
}
export const scoped = (scope: Scope, path: string, values?: Record<string, string>) =>
	`/issue-tracking/${path}?${scopeQuery(scope, values)}`;
export const fetchJSON = <T>(url: string) => http.get<T>(url, { timeout: 90000 });
export const saveConfig = (config: OAuthConfig) =>
	http.put<ConnectionState>("/linear/config", config);
export const authorize = () => http.post<Authorization>("/linear/authorize");
export const authorizeProjectCreation = () =>
	http.post<Authorization>("/linear/authorize/project-creation");
export const cancelAuthorization = () => http.delete<ConnectionState>("/linear/authorize");
export const disconnect = (id: string) =>
	http.delete<ConnectionState>(`/linear/connections/${encodeURIComponent(id)}`);
export const saveBinding = (scope: Scope, connectionId: string, project: Project, teamId: string) =>
	http.put<Binding>(
		scoped(scope, "binding"),
		{ connectionId, project, teamId },
		{ timeout: 90000 },
	);
export const unbind = (scope: Scope) => http.delete(scoped(scope, "binding"));
export const createAndBindProject = (
	scope: Scope,
	connectionId: string,
	name: string,
	teamId: string,
) =>
	http.post<Binding>(
		scoped(scope, "projects"),
		{ connectionId, name, teamId },
		{ timeout: 120000 },
	);
export const createIssue = (scope: Scope, input: IssueInput) =>
	http.post<Issue>(scoped(scope, "issues"), input, { timeout: 120000 });
export const updateIssue = (scope: Scope, id: string, input: IssueInput) =>
	http.put<Issue>(scoped(scope, `issues/${encodeURIComponent(id)}`), input, { timeout: 120000 });
export const createComment = (scope: Scope, id: string, body: string, operationId: string) =>
	http.post<Comment>(
		scoped(scope, `issues/${encodeURIComponent(id)}/comments`),
		{ body, operationId },
		{ timeout: 120000 },
	);
export function errorCode(error: unknown) {
	return String((error as HttpError)?.context?.linearCode ?? "");
}
export function errorText(error: unknown) {
	return (error as HttpError)?.message || String(error);
}

export type GatewayStatus = "processing" | "submitted" | "failed";

export interface GatewayJobs {
	enabled: boolean;
	issues: Record<string, GatewayStatus>;
}

export const gatewayJobsKey = (scope: Scope) => scoped(scope, "gateway/jobs");
export function dispatchToGateway(scope: Scope, id: string, options: CodexOptions) {
	const configured = !!(options.model || options.effort);
	const path = configured ? "gateway/jobs" : `gateway/jobs/${encodeURIComponent(id)}`;
	return http.post<{ status: GatewayStatus }>(
		scoped(scope, path),
		configured ? { issue: id, ...options } : undefined,
		{ timeout: 60000 },
	);
}
