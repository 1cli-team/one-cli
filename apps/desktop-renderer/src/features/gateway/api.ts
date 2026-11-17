import useSWR from "swr";
import http from "@/lib/http";
import { scopeQuery, type Binding, type Scope } from "@/features/linear/api";
import type { HttpError } from "@/types/api";
import type { ExecutionProgress, ExecutionQuestion, ExecutionState } from "./executions";

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

export interface CodexOptions {
	model?: string;
	effort?: string;
}

export interface CodexModelCatalog {
	models: { id: string; name: string; efforts: string[] }[];
	defaultModel?: string;
	defaultEffort?: string;
}

export function useCodexModels(enabled: boolean) {
	return useSWR(enabled ? "/gateway/codex/models" : null, (path) =>
		http.get<CodexModelCatalog>(path, { timeout: 40000 }),
	);
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
	executions?: boolean;
	workerUrl?: string;
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

export interface GatewayTask {
	id: string;
	identifier?: string;
	title?: string;
	projectName?: string;
	description?: string;
	url?: string;
	stage: string;
	updatedAt: string;
	workerName?: string;
	lastError?: string;
	localResultJobId?: string;
	codex?: CodexOptions;
	repository?: { host: string; slug: string };
	repositories?: JobRepository[];
	pullRequests?: NonNullable<JobRepository["pr"]>[];
	subtasks?: GatewaySubtask[];
	runs?: GatewayTaskRun[];
}

export interface GatewayTaskRun {
	id: string;
	workerId: string;
	workerName: string;
	source: "cli" | "desktop";
	state: ExecutionState;
	revision?: boolean;
	createdAt: string;
	question?: ExecutionQuestion;
	progress?: ExecutionProgress;
}

export interface GatewaySubtask {
	identifier: string;
	title: string;
	stage: string;
	commitSha?: string;
	response?: string;
}

export interface GatewayTaskPage {
	items: GatewayTask[];
	page: number;
	pageCount: number;
	partial?: boolean;
}

export interface GatewayTaskFilters {
	worker: string;
	stage: string;
	source: string;
}

export function useGatewayTasks(
	connection: GatewayStatus | undefined,
	page: number,
	filters: GatewayTaskFilters,
) {
	const query = new URLSearchParams({ page: String(page), ...filters }).toString();
	return useSWR<GatewayTaskPage, HttpError>(
		connection?.snapshot
			? (["/gateway/tasks", query, connection.url, connection.snapshot.startedAt] as const)
			: null,
		([path, query]) => http.get<GatewayTaskPage>(`${path}?${query}`),
		{ refreshInterval: 5000, shouldRetryOnError: (error) => error.status !== 404 },
	);
}

export function useGatewayJobCodexResult(connection: string | undefined, id: string | undefined) {
	return useSWR<{ response?: string }, HttpError>(
		connection && id
			? ([`/gateway/jobs/${encodeURIComponent(id)}/codex-result`, connection] as const)
			: null,
		([path]) => http.get<{ response?: string }>(path, { timeout: 18000 }),
		{ revalidateOnFocus: false, shouldRetryOnError: false },
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
