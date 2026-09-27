// types/api.ts mirrors the transport-neutral shapes exposed by the Go
// application layer. Backend identities and project fields intentionally come
// from GET /api/catalog instead of a second hard-coded frontend registry.

export type BackendDomain = "env";
export type SectionKey = `${BackendDomain}/${string}`;
export type JsonValue = string | number | boolean | null | JsonObject | JsonValue[];
export interface JsonObject {
	[key: string]: JsonValue | undefined;
}

export interface BackendRequirement {
	kind: "binary" | "capability";
	name: string;
	optional?: boolean;
}

export interface BackendSpec {
	id: SectionKey;
	domain: BackendDomain;
	name: string;
	capabilities: string[];
	traits?: string[];
	requirements?: BackendRequirement[];
	project?: {
		configurable: boolean;
		fields?: ProjectFieldSpec[];
	};
}

export type ProjectFieldType = "string" | "environment";

export interface ProjectFieldSpec {
	path: string;
	input_name: string;
	type: ProjectFieldType;
	label_key: string;
	required?: boolean;
	placeholder?: string;
}

export interface CatalogResponse {
	schema: "one-cli/catalog/v1";
	backends: BackendSpec[];
}

// ──────────────────────────── error envelope ────────────────────────────

export interface RemediationStep {
	action: string;
	hint?: string;
	command?: string;
	destructive?: boolean;
}

export interface ErrorEnvelope {
	schema: "one-cli/error/v1";
	error: {
		code: string;
		message: string;
		context: Record<string, unknown>;
		remediation: RemediationStep[];
	};
}

// HttpError is what http.ts rejects with. status carries the HTTP code so
// callers can branch on 401/403/404 without needing to inspect the
// envelope.
export interface HttpError {
	status: number;
	code: string;
	message: string;
	context: Record<string, unknown>;
	remediation: RemediationStep[];
}

// ──────────────────────────── workspace overview ────────────────────────
//
// Mirrors workspace.Overview in packages/cli/internal/workspace/overview.go.
// Returned by singular or registry-scoped Workspace overview routes.
// `present: false` is retained for the legacy launch-root route.

export type OverviewIssueDomain = "env";
export type OverviewIssueSeverity = "missing";
export type OverviewIssueReason = "backend" | "profile";

export interface OverviewIssue {
	domain: OverviewIssueDomain;
	severity: OverviewIssueSeverity;
	message: string;
	reason?: OverviewIssueReason;
	backend?: string;
	section?: SectionKey;
	profile?: string;
}

export type OverviewProjectKind = "app" | "service" | "package";

export interface OverviewProject {
	name: string;
	relativeDir: string;
	kind: OverviewProjectKind;
	templateId?: string;
	toolchain?: string;
	domains?: Partial<Record<OverviewIssueDomain, string>>;
	issues?: OverviewIssue[];
}

export interface OverviewWorkspaceSummary {
	id?: string;
	name?: string;
	manifestVersion: number;
	defaultEnvironment?: string;
	environments?: string[];
	domains?: Partial<Record<OverviewIssueDomain, string>>;
}

export interface Overview {
	schema: "one-cli/workspace-overview/v1";
	present: boolean;
	root?: string;
	environment?: string;
	workspace?: OverviewWorkspaceSummary;
	projects?: OverviewProject[];
	issues?: OverviewIssue[];
}

// ───────────────────────── workspace registry ──────────────────────────

export type WorkspaceRegistryStatus =
	| "ready"
	| "missing"
	| "invalid"
	| "identity-missing"
	| "identity-conflict";

export interface WorkspaceRegistryEntry {
	entryId: string;
	id?: string;
	name: string;
	root: string;
	status: WorkspaceRegistryStatus;
	projectCount: number;
	lastSeenAt: string;
}

export interface WorkspacesResponse {
	schema: "one-cli/workspaces/v1";
	currentEntryId?: string;
	workspaces: WorkspaceRegistryEntry[];
}

// ─────────────────────────── project configuration ─────────────────────

export interface ProjectEnvironmentSettings {
	backend?: string;
	path?: string;
	inherits: boolean;
	disabled: boolean;
	keys?: string[];
}

export interface ProjectSettings {
	name: string;
	relativeDir: string;
	kind: OverviewProjectKind;
	templateId?: string;
	toolchain?: string;
	packageManager?: string;
	buildVersion?: string;
	devCommand?: string;
	build?: {
		command?: string;
		source?: string;
		status: "ready" | "missing" | "invalid";
	};
	defaultEnvironment?: string;
	availableEnvironments?: string[];
	environment: ProjectEnvironmentSettings;
}

export interface ProjectSettingsResponse {
	schema: "one-cli/workspace-project/v1";
	root: string;
	environment?: string;
	revision: string;
	project: ProjectSettings;
}

// ───────────────────────── manifest draft publication ──────────────────

export interface ProjectGeneralPatch {
	buildVersion: string;
	devCommand: string;
}

export interface ProjectEnvironmentPatch {
	path: string;
	inherits: boolean;
	disabled: boolean;
}

export interface WorkspaceEnvironmentPatch {
	backend: string;
	projectId?: string;
	projectName?: string;
	siteUrl?: string;
}

export interface WorkspaceManifestPatch {
	environment?: WorkspaceEnvironmentPatch;
}

export interface ProjectManifestPatch {
	project: string;
	general?: ProjectGeneralPatch;
	environment?: ProjectEnvironmentPatch;
}

export interface ApplyManifestRequest {
	workspace?: WorkspaceManifestPatch;
	revision: string;
	changes: ProjectManifestPatch[];
}

export interface ApplyManifestResponse {
	schema: "one-cli/workspace-manifest-apply/v1";
	revision: string;
	applied: number;
}

export interface PreviewManifestRequest extends ApplyManifestRequest {
	workspace?: WorkspaceManifestPatch;
}

export interface PreviewManifestResponse {
	schema: "one-cli/workspace-manifest-preview/v1";
	revision: string;
	before: string;
	after: string;
}

// ─────────────────────────── Infisical secrets ─────────────────────────

export interface SecretListResponse {
	schema: "one-cli/env-list/v1";
	env: string;
	path: string;
	keys: string[];
	total: number;
}

export interface SecretValueResponse {
	schema: "one-cli/env-get/v1";
	env: string;
	path: string;
	key: string;
	value: string;
}

export interface SecretMutationResponse {
	schema: "one-cli/env-set/v1" | "one-cli/env-delete/v1";
	env: string;
	path: string;
	key: string;
	action?: "created" | "updated" | "unchanged";
	status?: "deleted";
}

export interface WorkspaceEnvironmentSettings {
	schema: string;
	revision: string;
	backend: string;
	projectId: string;
	projectName: string;
	siteUrl: string;
}
