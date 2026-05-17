// types/api.ts mirrors the Go shapes in internal/profile + the response
// envelopes in internal/serve/handlers_profile.go. Keep in sync — when the
// Go side adds a field, surface it here.

// ──────────────────────────── per-backend profile types ─────────────────

export interface InfisicalCredentials {
	clientId: string;
	clientSecret: string;
}

export interface InfisicalProfile {
	siteUrl: string;
	credentials?: InfisicalCredentials;
}

export interface S3Credentials {
	accessKeyId: string;
	accessKeySecret: string;
}

export interface S3Profile {
	endpoint?: string;
	region?: string;
	forcePathStyle?: boolean;
	credentials?: S3Credentials;
}

export interface KustomizeProfile {
	kubeconfigPath?: string;
	kubeconfigContext?: string;
}

// Dotenv carries no fields today; keep the type so the wire shape stays
// uniform across sections (handler still accepts {} for the body).
export type DotenvProfile = Record<string, never>;

export interface ContainerCredentials {
	username: string;
	password: string;
}

// ContainerProfile is shared across all four container backend kinds
// (docker / dockerhub / ghcr / acr). Only `docker` reads `registry`;
// `acr` reads `region` (host is derived from it); `dockerhub` and
// `ghcr` ignore both (host is fixed). Mirrors profile.ContainerProfile
// in packages/cli/internal/profile/types.go.
export interface ContainerProfile {
	registry?: string;
	region?: string;
	namespace?: string;
	credentials?: ContainerCredentials;
}

export interface VercelCredentials {
	apiToken: string;
}

export interface VercelProfile {
	team?: string;
	credentials?: VercelCredentials;
}

export interface CloudflareCredentials {
	apiToken: string;
}

export interface CloudflareProfile {
	accountId?: string;
	credentials?: CloudflareCredentials;
}

export interface EdgeOneCredentials {
	apiToken: string;
}

export interface EdgeOneProfile {
	region?: string;
	credentials?: EdgeOneCredentials;
}

// ──────────────────────────── (domain, backend) pairs ───────────────────

export const SECTION_KEYS = [
	"env/infisical",
	"deploy/aliyun-oss",
	"deploy/tencent-cos",
	"deploy/aws-s3",
	"deploy/minio",
	"deploy/rustfs",
	"deploy/r2",
	"deploy/kustomize",
	"deploy/vercel",
	"deploy/cloudflare",
	"deploy/edgeone",
	"container/docker",
	"container/dockerhub",
	"container/ghcr",
	"container/acr",
] as const;

export type SectionKey = (typeof SECTION_KEYS)[number];

// Canonical order for grouping the sidebar / home grid by domain.
export const SECTION_DOMAINS = ["env", "deploy", "container"] as const;
export type SectionDomain = (typeof SECTION_DOMAINS)[number];

// SECTION_KEYS_BY_DOMAIN preserves the SECTION_KEYS within-group order
// (which is also the canonical CLI display order). Empty arrays are
// possible if a domain has every backend stripped (env after PR 2
// removes dotenv, container has just docker — both still non-empty).
export const SECTION_KEYS_BY_DOMAIN: Record<SectionDomain, SectionKey[]> = {
	env: SECTION_KEYS.filter((k) => k.startsWith("env/")),
	deploy: SECTION_KEYS.filter((k) => k.startsWith("deploy/")),
	container: SECTION_KEYS.filter((k) => k.startsWith("container/")),
};

// SectionMeta describes one (domain, backend) pair for the list view.
// Labels are zh-CN to match the rest of the CLI's user-facing strings.
export interface SectionMeta {
	key: SectionKey;
	domain: "env" | "deploy" | "container";
	backend: string;
	title: string;
	description: string;
}

export const SECTION_META: Record<SectionKey, SectionMeta> = {
	"env/infisical": {
		key: "env/infisical",
		domain: "env",
		backend: "infisical",
		title: "Infisical",
		description: "Universal Auth 凭据 + Infisical site URL（env 域）",
	},
	"deploy/aliyun-oss": {
		key: "deploy/aliyun-oss",
		domain: "deploy",
		backend: "aliyun-oss",
		title: "Aliyun OSS",
		description: "阿里云 OSS endpoint + AK/SK（deploy 域，S3 协议）",
	},
	"deploy/tencent-cos": {
		key: "deploy/tencent-cos",
		domain: "deploy",
		backend: "tencent-cos",
		title: "Tencent COS",
		description: "腾讯云 COS endpoint + AK/SK（deploy 域，S3 协议）",
	},
	"deploy/aws-s3": {
		key: "deploy/aws-s3",
		domain: "deploy",
		backend: "aws-s3",
		title: "AWS S3",
		description: "AWS S3 region + AK/SK（deploy 域；endpoint 留空走 SDK 默认）",
	},
	"deploy/minio": {
		key: "deploy/minio",
		domain: "deploy",
		backend: "minio",
		title: "MinIO",
		description: "MinIO 自部署对象存储（deploy 域，path-style 寻址）",
	},
	"deploy/rustfs": {
		key: "deploy/rustfs",
		domain: "deploy",
		backend: "rustfs",
		title: "RustFS",
		description: "RustFS 自部署对象存储（deploy 域，path-style 寻址）",
	},
	"deploy/r2": {
		key: "deploy/r2",
		domain: "deploy",
		backend: "r2",
		title: "Cloudflare R2",
		description: "Cloudflare R2 endpoint + AK/SK（deploy 域）",
	},
	"deploy/kustomize": {
		key: "deploy/kustomize",
		domain: "deploy",
		backend: "kustomize",
		title: "Kustomize",
		description: "kubeconfig context（deploy 域）",
	},
	"deploy/vercel": {
		key: "deploy/vercel",
		domain: "deploy",
		backend: "vercel",
		title: "Vercel",
		description: "Vercel API token + 可选 team slug（deploy 域）",
	},
	"deploy/cloudflare": {
		key: "deploy/cloudflare",
		domain: "deploy",
		backend: "cloudflare",
		title: "Cloudflare",
		description: "Cloudflare API token + 可选 account ID（deploy 域）",
	},
	"deploy/edgeone": {
		key: "deploy/edgeone",
		domain: "deploy",
		backend: "edgeone",
		title: "EdgeOne Pages",
		description: "Tencent EdgeOne Pages token + 可选 region（deploy 域）",
	},
	"container/docker": {
		key: "container/docker",
		domain: "container",
		backend: "docker",
		title: "Docker Registry",
		description: "通用 Docker registry 协议（自建 Harbor / Quay / 任意私有 registry）",
	},
	"container/dockerhub": {
		key: "container/dockerhub",
		domain: "container",
		backend: "dockerhub",
		title: "Docker Hub",
		description: "Docker Hub（host 固定 index.docker.io；username + PAT）",
	},
	"container/ghcr": {
		key: "container/ghcr",
		domain: "container",
		backend: "ghcr",
		title: "GitHub Container Registry",
		description: "GHCR（host 固定 ghcr.io；GitHub PAT，需 write:packages）",
	},
	"container/acr": {
		key: "container/acr",
		domain: "container",
		backend: "acr",
		title: "Aliyun ACR",
		description:
			"阿里云 Container Registry（host 由 region 派生为 registry.<region>.aliyuncs.com）",
	},
};

// ──────────────────────────── per-section payload shape ─────────────────

export interface Section<T> {
	default?: string;
	profiles?: Record<string, T>;
}

export interface Config {
	version: number;
	"env/infisical"?: Section<InfisicalProfile>;
	"env/dotenv"?: Section<DotenvProfile>;
	"deploy/aliyun-oss"?: Section<S3Profile>;
	"deploy/tencent-cos"?: Section<S3Profile>;
	"deploy/aws-s3"?: Section<S3Profile>;
	"deploy/minio"?: Section<S3Profile>;
	"deploy/rustfs"?: Section<S3Profile>;
	"deploy/r2"?: Section<S3Profile>;
	"deploy/kustomize"?: Section<KustomizeProfile>;
	"deploy/vercel"?: Section<VercelProfile>;
	"deploy/cloudflare"?: Section<CloudflareProfile>;
	"deploy/edgeone"?: Section<EdgeOneProfile>;
	"container/docker"?: Section<ContainerProfile>;
	"container/dockerhub"?: Section<ContainerProfile>;
	"container/ghcr"?: Section<ContainerProfile>;
	"container/acr"?: Section<ContainerProfile>;
}

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

// Backend kinds accepted by the manifest-mutate endpoints in
// internal/serve/handlers_workspace_mutate.go. Kept in sync by hand with
// the knownEnvKinds / knownDeployKinds / knownContainerKinds maps there.
export const ENV_KINDS = ["dotenv", "infisical"] as const;
export const DEPLOY_KINDS = [
	"kustomize",
	"vercel",
	"cloudflare",
	"edgeone",
	"aws-s3",
	"aliyun-oss",
	"tencent-cos",
	"minio",
	"rustfs",
	"r2",
] as const;
export const CONTAINER_KINDS = ["docker", "dockerhub", "ghcr", "acr"] as const;
