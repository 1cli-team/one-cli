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
// Returned by GET /api/workspace/overview. `present: false` means `one
// serve` was launched outside a workspace; the home page falls back to the
// profile-editor view in that case.

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
	workspace?: OverviewWorkspaceSummary;
	projects?: OverviewProject[];
	issues?: OverviewIssue[];
}
