// types/api.ts mirrors the transport-neutral shapes exposed by the Go
// application layer. Backend identities and profile fields intentionally come
// from GET /api/catalog instead of a second hard-coded frontend registry.

export type BackendDomain = "env" | "deploy" | "container";
export type SectionKey = `${BackendDomain}/${string}`;
export type ProfileValue = string | number | boolean | null | AnyProfile | ProfileValue[];
export interface AnyProfile {
	[key: string]: ProfileValue | undefined;
}

export type BackendFieldType = "string" | "secret" | "boolean";

export interface BackendFieldSpec {
	path: string;
	input_name: string;
	type: BackendFieldType;
	label_key: string;
	required?: boolean;
	placeholder?: string;
	default?: ProfileValue;
}

export interface BackendRequirement {
	kind: "binary" | "capability" | "profile";
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
	profile: {
		configurable: boolean;
		fields?: BackendFieldSpec[];
	};
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

// ──────────────────────────── per-section payload shape ─────────────────

export interface Section<T> {
	default?: string;
	profiles?: Record<string, T>;
}

export type Config = { version: number } & Partial<Record<SectionKey, Section<AnyProfile>>>;

// ──────────────────────────── server response envelopes ─────────────────

export interface ConfigResponse {
	schema: "one-cli/serve-configure-config/v1";
	config_path: string;
	credentials_path: string;
	reveal: boolean;
	config: Config;
}

export interface SectionResponse<T = unknown> {
	schema: "one-cli/serve-configure-section/v1";
	domain: string;
	backend: string;
	reveal: boolean;
	section: Section<T>;
}

export interface UpsertResponse {
	schema: "one-cli/serve-configure-upsert/v1";
	status: "completed" | "updated";
	domain: string;
	backend: string;
	name: string;
	default: boolean;
}

export interface UseResponse {
	schema: "one-cli/serve-configure-use/v1";
	domain: string;
	backend: string;
	name: string;
}

export interface RemoveResponse {
	schema: "one-cli/serve-configure-remove/v1";
	status: "removed";
	domain: string;
	backend: string;
	name: string;
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

export type OverviewIssueDomain = "container" | "deploy" | "env";
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
	compatibleDeployTargets?: string[];
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

export interface ProfileBinding {
	name: string;
	source: "workspace-project" | "workspace" | "default" | string;
}

export type ProjectProfileBinding = ProfileBinding;

export interface WorkspaceProfileSettings {
	schema: "one-cli/workspace-profile/v1";
	root: string;
	environment?: string;
	revision: string;
	domain: "env";
	backend?: string;
	configurable: boolean;
	selectedProfile?: string;
	profile?: ProfileBinding;
}

export interface ProjectEnvironmentSettings {
	backend?: string;
	path?: string;
	inherits: boolean;
	disabled: boolean;
	keys?: string[];
	selectedProfile?: string;
	profile?: ProjectProfileBinding;
}

export interface ProjectContainerSettings {
	enabled: boolean;
	backend?: string;
	image?: string;
	namespace?: string;
	selectedProfile?: string;
	profile?: ProjectProfileBinding;
}

export interface ProjectDeploySettings {
	backend?: string;
	compatibleTargets?: string[];
	config?: Record<string, ProfileValue>;
	selectedProfile?: string;
	profile?: ProjectProfileBinding;
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
	defaultEnvironment?: string;
	availableEnvironments?: string[];
	environment: ProjectEnvironmentSettings;
	container: ProjectContainerSettings;
	deploy: ProjectDeploySettings;
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

export interface ProjectContainerPatch {
	enabled: boolean;
	backend: string;
	image: string;
	namespace: string;
}

export interface ProjectDeployPatch {
	backend: string;
	config: Record<string, ProfileValue>;
}

export interface WorkspaceEnvironmentPatch {
	backend: string;
}

export interface WorkspaceManifestPatch {
	environment?: WorkspaceEnvironmentPatch;
}

export interface ProjectManifestPatch {
	project: string;
	general?: ProjectGeneralPatch;
	environment?: ProjectEnvironmentPatch;
	container?: ProjectContainerPatch;
	deploy?: ProjectDeployPatch;
}

export interface ApplyManifestRequest {
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
