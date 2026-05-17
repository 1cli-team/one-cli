// Package profile is the machine-level configuration for env / deploy /
// container endpoints + credentials.
//
// Profiles are NOT per-workspace. A user configures their endpoints
// once on their machine (e.g. "I have a work Infisical account and a
// personal one"), then any workspace they `cd` into can pick which
// profile to use. Mirrors how kubectl / aws / gcloud handle multi-
// account / multi-cluster scenarios.
//
// On-disk layout — schema v1 splits AWS-style into two files:
//
//	~/.config/one/
//	├── config.json         # non-sensitive: endpoints, regions, paths,
//	│                       # default pointers, credentialSource markers
//	├── credentials.json    # sensitive: only fields that are actual
//	│                       # secrets (clientSecret / accessKeySecret /
//	│                       # registry password)
//	└── cache/              # short-lived tokens (e.g. Infisical OIDC)
//	    └── <domain>/<backend>/<profile>.json
//
// Each profile in config.json carries a `credentialSource` discriminator
// telling the resolver where to read the matching secret from. The current implementation only
// implements `file` (look in credentials.json); `env` / `command:<cmd>`
// / `keyring` are reserved sentinel values that surface
// PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED until they're wired up.
//
// File mode is 0600 on both files; parent dir is 0700.
//
// In-memory shape: profile structs still carry an inlined
// `Credentials *T` field so consumers (envcmd / deploycmd / etc.) keep
// reading `resolved.Profile.X.Credentials.Y`. The split is purely
// physical at the file boundary: store.go's Save zeroes Credentials
// before serializing config.json, and Load reads both files and merges
// credentials back into the in-memory profile. The HTTP handlers in
// internal/serve serialize the in-memory shape directly so the web
// UI continues to receive `{ "credentials": {...} }` inline (subject
// to masking when reveal != 1).

package profile

// SchemaVersion is the on-disk schema version Save always writes.
// Bumped on incompatible shape changes.
//
// The current schema uses per-section profile pointers named `default`.
// Shared one.manifest.json no longer stores profile names; this file
// owns both global defaults and per-workspace overrides.
const SchemaVersion = 1

// MinSupportedVersion is the oldest on-disk schema this binary still
// reads. Bumped only when an actually-incompatible shape lands.
const MinSupportedVersion = 1

// Config is the root document persisted to ~/.config/one/config.json.
//
// Each (domain/backend) is a top-level JSON key with a literal slash
// (Go's json package treats tag values as opaque strings). Section is
// typed to its backend's profile struct so reads are statically
// checked: storage cannot mix an S3 profile into the kustomize
// section. The profile's `Credentials *T` field is omitted at the
// file-write boundary by store.go (configForDisk).
type Config struct {
	Version            int                        `json:"version"`
	Workspaces         map[string]WorkspaceConfig `json:"workspaces,omitempty"`
	EnvInfisical       Section[InfisicalProfile]  `json:"env/infisical,omitempty"`
	EnvDotenv          Section[DotenvProfile]     `json:"env/dotenv,omitempty"`
	DeployAliyunOSS    Section[S3Profile]         `json:"deploy/aliyun-oss,omitempty"`
	DeployTencentCOS   Section[S3Profile]         `json:"deploy/tencent-cos,omitempty"`
	DeployAWSS3        Section[S3Profile]         `json:"deploy/aws-s3,omitempty"`
	DeployMinIO        Section[S3Profile]         `json:"deploy/minio,omitempty"`
	DeployRustFS       Section[S3Profile]         `json:"deploy/rustfs,omitempty"`
	DeployR2           Section[S3Profile]         `json:"deploy/r2,omitempty"`
	DeployKustomize    Section[KustomizeProfile]  `json:"deploy/kustomize,omitempty"`
	DeployVercel       Section[VercelProfile]     `json:"deploy/vercel,omitempty"`
	DeployCloudflare   Section[CloudflareProfile] `json:"deploy/cloudflare,omitempty"`
	DeployEdgeOne      Section[EdgeOneProfile]    `json:"deploy/edgeone,omitempty"`
	ContainerDocker    Section[ContainerProfile]  `json:"container/docker,omitempty"`
	ContainerDockerHub Section[ContainerProfile]  `json:"container/dockerhub,omitempty"`
	ContainerGHCR      Section[ContainerProfile]  `json:"container/ghcr,omitempty"`
	ContainerACR       Section[ContainerProfile]  `json:"container/acr,omitempty"`
}

// WorkspaceConfig stores machine-local profile choices for one shared
// workspace. The key in Config.Workspaces is manifest.workspace.id.
// Profiles maps "domain/backend" (for example "env/infisical") to the
// local profile name that should be used in that workspace. Projects
// optionally overrides those choices for a manifest project name.
type WorkspaceConfig struct {
	Name     string                            `json:"name,omitempty"`
	Root     string                            `json:"root,omitempty"`
	Profiles map[string]string                 `json:"profiles,omitempty"`
	Projects map[string]WorkspaceProjectConfig `json:"projects,omitempty"`
}

// IsEmpty reports whether the workspace binding has no useful data.
func (w WorkspaceConfig) IsEmpty() bool {
	return w.Name == "" && w.Root == "" && len(w.Profiles) == 0 && len(w.Projects) == 0
}

// WorkspaceProjectConfig stores per-project machine-local profile
// choices. The key in WorkspaceConfig.Projects is manifest.projects[].name.
type WorkspaceProjectConfig struct {
	Profiles map[string]string `json:"profiles,omitempty"`
}

// IsEmpty reports whether the project binding has no useful data.
func (p WorkspaceProjectConfig) IsEmpty() bool {
	return len(p.Profiles) == 0
}

// S3CompatSection returns the profile-section for one S3-compatible
// deploy backend. Returns nil for unknown kinds. All six sections share
// the same Section[S3Profile] shape so callers that touch any one of
// them ("upsert profile by name", "list profile names", "set default")
// can dispatch through this single accessor instead of duplicating one
// switch arm per kind.
func (c *Config) S3CompatSection(kind string) *Section[S3Profile] {
	switch kind {
	case "aliyun-oss":
		return &c.DeployAliyunOSS
	case "tencent-cos":
		return &c.DeployTencentCOS
	case "aws-s3":
		return &c.DeployAWSS3
	case "minio":
		return &c.DeployMinIO
	case "rustfs":
		return &c.DeployRustFS
	case "r2":
		return &c.DeployR2
	}
	return nil
}

// S3CompatKinds is the canonical ordered list of S3-compatible deploy
// backend ids — display order across help text, list output, marshal,
// and merge/extract. Listed once here so callers can range over the
// same order without re-typing the literals.
func S3CompatKinds() []string {
	return []string{"aliyun-oss", "tencent-cos", "aws-s3", "minio", "rustfs", "r2"}
}

// IsS3Compatible reports whether `backend` is one of the six
// S3-protocol-compatible deploy backend ids. Mirrored from
// workspace.IsS3CompatibleDeploy so the profile package does not import
// workspace.
func IsS3Compatible(backend string) bool {
	switch backend {
	case "aliyun-oss", "tencent-cos", "aws-s3", "minio", "rustfs", "r2":
		return true
	}
	return false
}

// ContainerKindSection returns the profile-section for one container
// backend kind. All four kinds share Section[ContainerProfile]; this
// dispatcher lets callers (mutate / store / resolver) reach the right
// bucket by kind without per-kind switch arms.
func (c *Config) ContainerKindSection(kind string) *Section[ContainerProfile] {
	switch kind {
	case "docker":
		return &c.ContainerDocker
	case "dockerhub":
		return &c.ContainerDockerHub
	case "ghcr":
		return &c.ContainerGHCR
	case "acr":
		return &c.ContainerACR
	}
	return nil
}

// ContainerKinds is the canonical ordered list of container backend
// ids — display order across help text, list output, marshal, and
// merge/extract. Listed once here so callers can range over the same
// order without re-typing the literals.
func ContainerKinds() []string {
	return []string{"docker", "dockerhub", "ghcr", "acr"}
}

// IsContainerKind reports whether `backend` is a recognised container
// backend id. Mirrors IsS3Compatible's contract.
func IsContainerKind(backend string) bool {
	switch backend {
	case "docker", "dockerhub", "ghcr", "acr":
		return true
	}
	return false
}

// MarshalJSON drops empty sections from the output so a fresh config
// renders as just `{"version":1}` instead of all five (domain/backend)
// keys with empty `{}` values. Encoding/json's `omitempty` doesn't fire
// for non-pointer struct fields, so we build the object manually.
func (c Config) MarshalJSON() ([]byte, error) {
	return marshalConfig(c)
}

// CredentialsFile is the root document persisted to
// ~/.config/one/credentials.json. It mirrors Config but only carries
// secret fields, indexed by the same (domain/backend, profile-name)
// keys. Sections without secrets (env/dotenv, deploy/kustomize) don't
// appear here.
type CredentialsFile struct {
	Version            int                                `json:"version"`
	EnvInfisical       CredSection[InfisicalCredentials]  `json:"env/infisical,omitempty"`
	DeployAliyunOSS    CredSection[S3Credentials]         `json:"deploy/aliyun-oss,omitempty"`
	DeployTencentCOS   CredSection[S3Credentials]         `json:"deploy/tencent-cos,omitempty"`
	DeployAWSS3        CredSection[S3Credentials]         `json:"deploy/aws-s3,omitempty"`
	DeployMinIO        CredSection[S3Credentials]         `json:"deploy/minio,omitempty"`
	DeployRustFS       CredSection[S3Credentials]         `json:"deploy/rustfs,omitempty"`
	DeployR2           CredSection[S3Credentials]         `json:"deploy/r2,omitempty"`
	DeployVercel       CredSection[VercelCredentials]     `json:"deploy/vercel,omitempty"`
	DeployCloudflare   CredSection[CloudflareCredentials] `json:"deploy/cloudflare,omitempty"`
	DeployEdgeOne      CredSection[EdgeOneCredentials]    `json:"deploy/edgeone,omitempty"`
	ContainerDocker    CredSection[ContainerCredentials]  `json:"container/docker,omitempty"`
	ContainerDockerHub CredSection[ContainerCredentials]  `json:"container/dockerhub,omitempty"`
	ContainerGHCR      CredSection[ContainerCredentials]  `json:"container/ghcr,omitempty"`
	ContainerACR       CredSection[ContainerCredentials]  `json:"container/acr,omitempty"`
}

// S3CompatCredSection returns the credentials-section for one
// S3-compatible deploy backend. Sibling of Config.S3CompatSection;
// same dispatch model.
func (c *CredentialsFile) S3CompatCredSection(kind string) *CredSection[S3Credentials] {
	switch kind {
	case "aliyun-oss":
		return &c.DeployAliyunOSS
	case "tencent-cos":
		return &c.DeployTencentCOS
	case "aws-s3":
		return &c.DeployAWSS3
	case "minio":
		return &c.DeployMinIO
	case "rustfs":
		return &c.DeployRustFS
	case "r2":
		return &c.DeployR2
	}
	return nil
}

// ContainerKindCredSection returns the credentials-section for one
// container backend kind. Sibling of Config.ContainerKindSection;
// same dispatch model.
func (c *CredentialsFile) ContainerKindCredSection(kind string) *CredSection[ContainerCredentials] {
	switch kind {
	case "docker":
		return &c.ContainerDocker
	case "dockerhub":
		return &c.ContainerDockerHub
	case "ghcr":
		return &c.ContainerGHCR
	case "acr":
		return &c.ContainerACR
	}
	return nil
}

// MarshalJSON omits empty sections, same trick as Config.
func (c CredentialsFile) MarshalJSON() ([]byte, error) {
	return marshalCredentialsFile(c)
}

// Section is one (domain/backend) bucket in config.json: a default
// pointer + a name-keyed map of typed profiles. The default pointer
// lives per-section (not per-domain) — `deploy/aws-s3.default = "web-prod"`
// and `deploy/kustomize.default = "prod-k8s"` coexist without conflict.
type Section[T any] struct {
	Default  string       `json:"default,omitempty"`
	Profiles map[string]T `json:"profiles,omitempty"`
}

// IsEmpty reports whether the section has no default pointer and no
// profiles. Used by Config.MarshalJSON to suppress empty sections.
func (s Section[T]) IsEmpty() bool {
	return s.Default == "" && len(s.Profiles) == 0
}

// CredSection is the credentials.json sibling of Section. No `default`
// pointer here — default selection is purely a config-side concern.
type CredSection[T any] struct {
	Profiles map[string]T `json:"profiles,omitempty"`
}

// IsEmpty reports whether the credentials section has no entries.
func (s CredSection[T]) IsEmpty() bool {
	return len(s.Profiles) == 0
}

// Profile is the resolver's return shape — a discriminated union over
// every backend type we support. Storage no longer uses this struct
// (sections store backend-typed profiles directly); it survives only
// as the in-memory shape `Resolve` hands callers, so consumers
// (envcmd / deploycmd / containercmd) keep reading
// `resolved.Profile.S3` etc. without churn.
//
// S3 is reused as the in-memory slot for all six S3-compatible deploy
// backends (aliyun-oss / tencent-cos / aws-s3 / minio / rustfs / r2).
// They share the same S3Profile shape; Backend discriminates which one
// the resolver matched.
type Profile struct {
	Backend    string             `json:"backend,omitempty"`
	Infisical  *InfisicalProfile  `json:"infisical,omitempty"`
	Dotenv     *DotenvProfile     `json:"dotenv,omitempty"`
	Kustomize  *KustomizeProfile  `json:"kustomize,omitempty"`
	S3         *S3Profile         `json:"s3,omitempty"`
	Vercel     *VercelProfile     `json:"vercel,omitempty"`
	Cloudflare *CloudflareProfile `json:"cloudflare,omitempty"`
	EdgeOne    *EdgeOneProfile    `json:"edgeone,omitempty"`
	Container  *ContainerProfile  `json:"container,omitempty"`
}

// CredentialSource discriminates where the resolver should fetch a
// profile's secrets from. The current implementation only supports SourceFile; the rest are
// sentinels reserved for future wiring (env / external command /
// system keyring).
const (
	SourceFile    = "file"
	SourceEnv     = "env"
	SourceCommand = "command:" // prefix; full value e.g. "command:op-cli read ..."
	SourceKeyring = "keyring"
)

// IsFileSource reports whether `s` selects the file-backed credential
// loader. Empty string is treated as file (default for newly-added
// profiles that don't set the field explicitly).
func IsFileSource(s string) bool {
	return s == "" || s == SourceFile
}

// InfisicalProfile carries the machine-level Infisical-instance
// identity: which site (saas vs self-hosted) + the credentials to
// authenticate as. Project-level fields (projectId, environments,
// rootPath) live in the workspace's one.manifest.json#env block — a
// single profile drives many workspaces.
//
// In-memory: Credentials is populated by Load (read from
// credentials.json). On-disk: store.go's configForDisk zeroes
// Credentials before serializing config.json so secrets never leak
// into the non-sensitive file.
type InfisicalProfile struct {
	SiteURL          string                `json:"siteUrl"`
	CredentialSource string                `json:"credentialSource,omitempty"`
	Credentials      *InfisicalCredentials `json:"credentials,omitempty"`
}

// InfisicalCredentials holds Universal Auth machine-identity creds.
type InfisicalCredentials struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// DotenvProfile is intentionally minimal — dotenv has no remote /
// no schema, so a "profile" for dotenv is just a name. Useful for
// users who want a uniform `one env --profile <name>` UX.
//
// Has no credentials.
type DotenvProfile struct{}

// KustomizeProfile carries the Kubernetes connection file used by a
// `deploy/kustomize` deployment target. K8s cluster auth lives in
// ~/.kube/config; this profile only points at which kubeconfig +
// context to use, so it has no inline credentials.
type KustomizeProfile struct {
	KubeconfigPath    string `json:"kubeconfigPath,omitempty"`
	KubeconfigContext string `json:"kubeconfigContext,omitempty"`
}

// S3Profile is the machine-level config shared by all six S3-compatible
// deploy backends (deploy/aliyun-oss, deploy/tencent-cos, deploy/aws-s3,
// deploy/minio, deploy/rustfs, deploy/r2). They all speak the standard
// S3 protocol with AccessKey-pair auth; the only difference is which
// vendor's endpoint they point at, surfaced via the user-facing backend
// id rather than via the profile schema.
//
// Bucket is per-subproject (projects[i].deploy.bucket) — one credential
// reaches many buckets, so binding bucket to the profile would force
// one profile per bucket.
type S3Profile struct {
	Endpoint         string         `json:"endpoint,omitempty"`
	Region           string         `json:"region,omitempty"`
	ForcePathStyle   bool           `json:"forcePathStyle,omitempty"`
	CredentialSource string         `json:"credentialSource,omitempty"`
	Credentials      *S3Credentials `json:"credentials,omitempty"`
}

// S3Credentials is the AccessKey pair shared by every S3-compatible
// deploy backend — Aliyun OSS / Tencent COS / AWS S3 / MinIO / RustFS /
// Cloudflare R2 all authenticate with AKID + secret.
type S3Credentials struct {
	AccessKeyID     string `json:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret"`
}

// VercelProfile is the deploy/vercel backend's machine-level config.
// Single profile drives many workspaces — one Vercel personal token /
// team-scoped token reaches every project under that account / team.
//
// Per-project Vercel project linkage (Project ID + Project name)
// lives in manifest.projects[i].deploy.vercel and is set up by
// `vercel link` / `vercel pull` the first time a project deploys.
//
// Team is the optional org slug passed via `--scope`; empty means
// "personal scope" (the token owner's account).
type VercelProfile struct {
	Team             string             `json:"team,omitempty"`
	CredentialSource string             `json:"credentialSource,omitempty"`
	Credentials      *VercelCredentials `json:"credentials,omitempty"`
}

// VercelCredentials holds the Vercel API token. Created in
// vercel.com → Account Settings → Tokens, scoped to either a personal
// account or a team. The token is the only credential — Vercel's CLI
// authenticates entirely via this token.
type VercelCredentials struct {
	APIToken string `json:"apiToken"`
}

// CloudflareProfile is the deploy/cloudflare backend's machine-level
// config. Single profile drives many workspaces — one Cloudflare API
// token reaches every Worker / static asset bundle under the account it
// scopes to.
//
// Per-project Worker / Pages name lives in
// manifest.projects[i].deploy.cloudflare; wrangler.toml inside the
// project is the source of truth wrangler itself reads.
//
// AccountID is the optional account scope. wrangler will read
// CLOUDFLARE_ACCOUNT_ID from the environment when set; required only on
// multi-account tokens. Empty means "use the token's only account".
type CloudflareProfile struct {
	AccountID        string                 `json:"accountId,omitempty"`
	CredentialSource string                 `json:"credentialSource,omitempty"`
	Credentials      *CloudflareCredentials `json:"credentials,omitempty"`
}

// CloudflareCredentials holds the Cloudflare API token. Created in
// dash.cloudflare.com → My Profile → API Tokens, with at minimum the
// Edit Workers permission. wrangler reads it from CLOUDFLARE_API_TOKEN
// at exec time so the token never appears on argv.
type CloudflareCredentials struct {
	APIToken string `json:"apiToken"`
}

// EdgeOneProfile is the deploy/edgeone backend's machine-level config.
// EdgeOne is Tencent Cloud's edge platform — single profile drives
// many workspaces under one Tencent account.
//
// Region is the optional Tencent Cloud region slug (e.g. ap-guangzhou,
// ap-shanghai). Empty defers to whatever the edgeone CLI picks based
// on the project's binding.
type EdgeOneProfile struct {
	Region           string              `json:"region,omitempty"`
	CredentialSource string              `json:"credentialSource,omitempty"`
	Credentials      *EdgeOneCredentials `json:"credentials,omitempty"`
}

// EdgeOneCredentials holds the EdgeOne Pages API token used by the
// upstream `edgeone pages deploy --token` flow.
type EdgeOneCredentials struct {
	APIToken string `json:"apiToken"`
}

// ContainerProfile carries a container-registry endpoint + push
// credentials. Single shape covers every registry that speaks the
// standard registry protocol with HTTP Basic auth (username + token).
// Four backend kinds share this shape today:
//   - "docker" — user-supplied Registry host (Harbor / self-hosted / etc.)
//   - "dockerhub" — host fixed to "index.docker.io"; Registry is ignored
//   - "ghcr" — host fixed to "ghcr.io"; Registry is ignored
//   - "acr" — Aliyun ACR; host derived from Region as
//     "registry.<region>.aliyuncs.com"
//
// The host-derivation logic lives in infra/docker.ResolveRegistry; the
// profile only stores the user-supplied raw inputs.
type ContainerProfile struct {
	Registry         string                `json:"registry,omitempty"`
	Region           string                `json:"region,omitempty"`
	Namespace        string                `json:"namespace,omitempty"`
	CredentialSource string                `json:"credentialSource,omitempty"`
	Credentials      *ContainerCredentials `json:"credentials,omitempty"`
}

// ContainerCredentials holds the registry login pair. Username is the
// account / RAM AKID / robot name; Password is the PAT / RAM secret /
// access token used by `docker login --password-stdin`.
type ContainerCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Domain identifies the top-level grouping for a backend. Every
// concrete (domain, backend) pair maps to one Section in Config.
type Domain string

const (
	DomainEnv       Domain = "env"
	DomainDeploy    Domain = "deploy"
	DomainContainer Domain = "container"
)

// SupportedDomains returns the order in which CRUD commands iterate
// the config. Add a new domain here when you add a new (domain,
// backend) section.
func SupportedDomains() []Domain {
	return []Domain{DomainEnv, DomainDeploy, DomainContainer}
}

// BackendsForDomain returns the list of backend names the schema
// recognises under the given domain. Used by validation + by the
// CRUD `add` command's interactive backend picker.
func BackendsForDomain(domain Domain) []string {
	switch domain {
	case DomainEnv:
		return []string{"infisical", "dotenv"}
	case DomainDeploy:
		return []string{
			"aliyun-oss", "tencent-cos", "aws-s3", "minio", "rustfs", "r2",
			"kustomize", "vercel", "cloudflare", "edgeone",
		}
	case DomainContainer:
		return ContainerKinds()
	}
	return nil
}

// BackendDomain returns the domain that owns a bare backend name.
// "" for unknown values.
func BackendDomain(backend string) Domain {
	switch backend {
	case "dotenv", "infisical":
		return DomainEnv
	case "kustomize", "vercel", "cloudflare", "edgeone":
		return DomainDeploy
	}
	if IsS3Compatible(backend) {
		return DomainDeploy
	}
	if IsContainerKind(backend) {
		return DomainContainer
	}
	return ""
}

// SectionKey is the top-level JSON key for a (domain, backend) pair,
// e.g. "env/infisical", "deploy/aws-s3". Useful for diagnostics + error
// messages so users can grep their config.json by the same string
// they see in the error envelope.
func SectionKey(domain Domain, backend string) string {
	return string(domain) + "/" + backend
}
