import useSWR from "swr";
import http from "@/lib/http";
import { scopeQuery, type Binding, type Scope } from "@/features/linear/api";
import type { HttpError } from "@/types/api";

export type ConnectionState =
	| "unconfigured"
	| "invalid"
	| "unreachable"
	| "unauthorized"
	| "incompatible"
	| "connected";

export interface GatewayConnection {
	url: string;
	token: string;
}

interface RepositoryIdentity {
	github: string;
	repository?: { host: string; slug: string };
}

export function repositoryRef(value: RepositoryIdentity) {
	return (
		value.repository ?? (value.github ? { host: "github.com", slug: value.github } : undefined)
	);
}

export function repositoryLabel(value: RepositoryIdentity) {
	return value.repository ? `${value.repository.host}/${value.repository.slug}` : value.github;
}

export interface ProjectSummary extends RepositoryIdentity {
	projectId: string;
	base: string;
	repositories?: (RepositoryIdentity & { base: string })[];
}

export interface GatewaySnapshot {
	startedAt: string;
	projects: number;
	mappings?: ProjectSummary[] | null;
	jobs: Record<"queued" | "active" | "review" | "completed" | "failed", number>;
}

export interface GatewayStatus {
	url: string;
	hasToken: boolean;
	state: ConnectionState;
	checkedAt: string;
	snapshot?: GatewaySnapshot;
}

const statusKey = "/gateway/status";
const fetchStatus = (path: string) => http.get<GatewayStatus>(path);

export function useGatewayStatus() {
	return useSWR(statusKey, fetchStatus, { refreshInterval: 5000 });
}

export interface JobRepository {
	repository: { host: string; slug: string };
	base?: string;
	branch?: string;
	unchanged?: boolean;
	pr?: { number: number; url: string; state: string; merged: boolean };
}

export interface GatewayJob {
	id: string;
	identifier: string;
	title: string;
	projectName?: string;
	description?: string;
	url?: string;
	stage: string;
	createdAt: string;
	lastError?: string;
	repositories: JobRepository[];
}

export interface GatewayJobPage {
	items: GatewayJob[];
	page: number;
	pageCount: number;
}

export function useGatewayJobs(connection: GatewayStatus | undefined, page: number) {
	return useSWR<GatewayJobPage, HttpError>(
		connection?.snapshot
			? (["/gateway/jobs", page, connection.url, connection.snapshot.startedAt] as const)
			: null,
		([path, page]) => http.get<GatewayJobPage>(`${path}?page=${page}`),
		{ refreshInterval: 5000, shouldRetryOnError: (error) => error.status !== 404 },
	);
}

export function saveConnection(connection: GatewayConnection) {
	return http.put<GatewayStatus>("/gateway/connection", connection);
}

export function testConnection(connection: GatewayConnection) {
	return http.post<GatewayStatus>("/gateway/connection/test", connection);
}

export interface ProjectOrigin extends RepositoryIdentity {
	path: string;
	name?: string;
	hasOrigin: boolean;
}

export interface RepositoryOption extends RepositoryIdentity {
	id: string;
	name: string;
	path: string;
}

export interface TeamFlow {
	running: string;
	review: string;
	done: string;
}

export interface ProjectMapping {
	repositoryId: string;
	repositoryIds?: string[];
	base: string;
	checkTask: string;
	existingCheck: boolean;
	teams: Record<string, TeamFlow>;
}

export type MappingInput = { repositoryId: string } | { repositoryIds: string[] };

export interface ProjectBinding {
	name: string;
	binding: Binding;
}

export const projectBindingsKey = "/gateway/project-bindings";
export const gatewayRepositoriesKey = "/gateway/repositories";
export const mappingKey = (scope: Scope) => `/gateway/project-mapping?${scopeQuery(scope)}`;
export const projectOriginsKey = (scope: Scope) => `/gateway/project-origins?${scopeQuery(scope)}`;
export const getProjectOrigins = (path: string) => http.get<ProjectOrigin[]>(path);
export const getProjectBindings = (path: string) => http.get<ProjectBinding[]>(path);
export const getRepositories = (path: string) =>
	http.get<RepositoryOption[]>(path, { timeout: 15000 });
export const getProjectMapping = (path: string) => http.get<ProjectMapping | null>(path);
export const saveProjectMapping = (scope: Scope, input: MappingInput) =>
	http.put<ProjectMapping>(mappingKey(scope), input, { timeout: 55000 });
