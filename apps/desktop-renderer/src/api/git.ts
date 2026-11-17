import http from "@/lib/http";
import { workspaceBasePath } from "@/api/workspaces";
import type { WorkspaceRegistryEntry } from "@/types/api";

export interface Worktree {
	id: string;
	path: string;
	head: string;
	branch: string;
	main: boolean;
	bare: boolean;
	detached: boolean;
	locked: boolean;
	lockReason?: string;
	prunable: boolean;
	missing: boolean;
	dirty: boolean;
	statusError?: string;
	workspaceRoot?: string;
}

export interface GitRepository {
	schema: string;
	id?: string;
	name?: string;
	path?: string;
	remotes: GitRemote[];
	commonDir: string;
	defaultParent: string;
	branches: string[];
	worktrees: Worktree[];
}

export interface AddWorktreeInput {
	path: string;
	branch: string;
	startPoint: string;
	newBranch: boolean;
	detached: boolean;
}

export interface CreateBranchInput {
	branch: string;
	startPoint: string;
	checkout: boolean;
}

export interface BranchResult {
	branch: string;
	head: string;
	checkedOut: boolean;
	warning?: string;
}

export interface WorktreeChange {
	worktree: Worktree;
	workspace?: WorkspaceRegistryEntry;
	warning?: string;
}

export interface GitRemote {
	name: string;
	fetchUrl: string;
	pushUrl: string;
}

export type GitScope = { kind: "workspace"; id: string };

function scopePath(scope: GitScope) {
	return `${workspaceBasePath(scope.id)}/git`;
}

export function worktreesKey(scope: GitScope) {
	return `${scopePath(scope)}/worktrees`;
}

export function getWorktrees(scope: GitScope) {
	return http.get<GitRepository>(worktreesKey(scope), { timeout: 150000 });
}

export function createBranch(scope: GitScope, input: CreateBranchInput) {
	return http.post<BranchResult>(`${scopePath(scope)}/branches`, input, { timeout: 150000 });
}

export function addWorktree(scope: GitScope, input: AddWorktreeInput) {
	return http.post<WorktreeChange>(worktreesKey(scope), input, { timeout: 150000 });
}

export function removeWorktree(scope: GitScope, id: string) {
	return http.delete(`${worktreesKey(scope)}/${encodeURIComponent(id)}`, { timeout: 150000 });
}

export function openWorktree(scope: GitScope, id: string) {
	return http.post<WorkspaceRegistryEntry>(`${worktreesKey(scope)}/${encodeURIComponent(id)}/open`);
}

export function cloneRepository(url: string, path: string) {
	return http.post<{ path: string; workspace?: WorkspaceRegistryEntry; warning?: string }>(
		"/git/clone",
		{ url, path },
		{ timeout: 960000 },
	);
}
