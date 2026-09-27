// Package profile stores machine-level environment endpoints and credentials.
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
//	├── config.json         # non-sensitive: endpoints,
//	│                       # default pointers, credentialSource markers
//	├── credentials.json    # sensitive: only fields that are actual
//	│                       # credentials (clientId / clientSecret)
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
// `Credentials *T` field so environment consumers keep
// reading `resolved.Profile.X.Credentials.Y`. The split is purely
// physical at the file boundary: store.go's Save zeroes Credentials
// before serializing config.json, and Load reads both files and merges
// credentials back into the in-memory profile. The HTTP handlers in
// internal/transport/http serialize the in-memory shape directly so the web
// UI continues to receive `{ "credentials": {...} }` inline (subject
// to masking when reveal != 1).

package profile

import catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"

// SchemaVersion is the on-disk schema version Save always writes.
// Bumped on incompatible shape changes.
//
// The current schema uses per-section profile pointers named `default`.
// Shared one.manifest.json no longer stores profile names. This file owns
// global defaults and keeps the legacy, environment-agnostic per-workspace
// overrides readable; new environment-aware choices live independently in
// profile-bindings.json.
const SchemaVersion = 1

// MinSupportedVersion is the oldest on-disk schema this binary still
// reads. Bumped only when an actually-incompatible shape lands.
const MinSupportedVersion = 1

// Config is the root document persisted to ~/.config/one/config.json.
//
// Each (domain/backend) is a top-level JSON key with a literal slash
// (Go's json package treats tag values as opaque strings). Section is
// typed to its backend's profile struct so reads are statically
// checked: storage cannot mix an Infisical profile into the dotenv
// section. The profile's `Credentials *T` field is omitted at the
// file-write boundary by store.go (configForDisk).
type Config struct {
	Version      int                        `json:"version"`
	Workspaces   map[string]WorkspaceConfig `json:"workspaces,omitempty"`
	EnvInfisical Section[InfisicalProfile]  `json:"env/infisical,omitempty"`
	EnvDotenv    Section[DotenvProfile]     `json:"env/dotenv,omitempty"`
}

// WorkspaceConfig stores legacy, environment-agnostic machine-local profile
// choices for one shared workspace. The key in Config.Workspaces is
// manifest.workspace.id.
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

// MarshalJSON drops empty sections from the output so a fresh config renders
// only its version instead of every (domain/backend) key with an empty value.
// Encoding/json's `omitempty` doesn't fire for non-pointer struct fields, so
// the schema policy table emits the object section by section.
func (c Config) MarshalJSON() ([]byte, error) {
	return marshalConfig(c)
}

// CredentialsFile is the root document persisted to
// ~/.config/one/credentials.json. It mirrors Config but only carries
// secret fields, indexed by the same (domain/backend, profile-name)
// keys. Sections without secrets (env/dotenv) don't
// appear here.
type CredentialsFile struct {
	Version      int                               `json:"version"`
	EnvInfisical CredSection[InfisicalCredentials] `json:"env/infisical,omitempty"`
}

// MarshalJSON omits empty sections, same trick as Config.
func (c CredentialsFile) MarshalJSON() ([]byte, error) {
	return marshalCredentialsFile(c)
}

// Section is one (domain/backend) bucket in config.json: a default
// pointer + a name-keyed map of typed profiles. The default pointer
// lives per-section (for example `env/infisical.default = "work"`).
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

// Profile is the resolver's in-memory result for an environment backend.
// Storage uses backend-typed sections; callers receive credentials inline.
type Profile struct {
	Backend   string            `json:"backend,omitempty"`
	Infisical *InfisicalProfile `json:"infisical,omitempty"`
	Dotenv    *DotenvProfile    `json:"dotenv,omitempty"`
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

// Domain identifies the top-level grouping for a backend. Every
// concrete (domain, backend) pair maps to one Section in Config.
type Domain = catalog.Domain

const (
	DomainEnv = catalog.DomainEnv
)

// SupportedDomains returns the Backend Catalog's stable domain order.
func SupportedDomains() []Domain {
	return catalog.Domains()
}

// BackendsForDomain returns the list of backend names the schema
// recognises under the given domain. Used by validation + by the
// CRUD `add` command's interactive backend picker.
func BackendsForDomain(domain Domain) []string {
	specs := catalog.Builtin().ForDomain(domain)
	// Profile mutation historically considers configurable backends before
	// local-only ones (today env/infisical before env/dotenv). Preserve that
	// behavior as a data-driven ordering rule rather than another id list.
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		if spec.Profile.Configurable {
			out = append(out, spec.ID.Name)
		}
	}
	for _, spec := range specs {
		if !spec.Profile.Configurable {
			out = append(out, spec.ID.Name)
		}
	}
	return out
}

// BackendDomain returns the domain that owns a bare backend name.
// "" for unknown values.
func BackendDomain(backend string) Domain {
	for _, domain := range catalog.Domains() {
		if _, ok := catalog.Builtin().Lookup(domain, backend); ok {
			return domain
		}
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
