// Package backend owns the canonical backend vocabulary used by every One
// CLI transport and application service. It is intentionally data-only: it
// knows what a backend is capable of, but it neither stores credentials nor
// imports concrete backend implementations.
package backend

import (
	"encoding/json"
	"strings"
)

// Domain groups backends by the product capability they implement. Domain is
// an internal routing concept; the user-facing noun is Backend.
type Domain string

const (
	DomainEnv       Domain = "env"
	DomainDeploy    Domain = "deploy"
	DomainContainer Domain = "container"
)

// Built-in backend names are declared beside the Catalog so other packages do
// not create competing identity vocabularies. A package may still branch on a
// name when it owns genuinely different compiled behavior.
const (
	EnvDotenv    = "dotenv"
	EnvInfisical = "infisical"

	DeployAliyunOSS  = "aliyun-oss"
	DeployTencentCOS = "tencent-cos"
	DeployAWSS3      = "aws-s3"
	DeployMinIO      = "minio"
	DeployRustFS     = "rustfs"
	DeployR2         = "r2"
	DeployKustomize  = "kustomize"
	DeployVercel     = "vercel"
	DeployCloudflare = "cloudflare"
	DeployEdgeOne    = "edgeone"

	ContainerDocker    = "docker"
	ContainerDockerHub = "dockerhub"
	ContainerGHCR      = "ghcr"
	ContainerACR       = "acr"
)

// Domains is the stable product display order.
func Domains() []Domain {
	return []Domain{DomainEnv, DomainDeploy, DomainContainer}
}

// BackendID is the canonical identity of one backend. String renders the
// compatibility pair used by profile storage and configure routes.
type BackendID struct {
	Domain Domain `json:"domain"`
	Name   string `json:"name"`
}

func (id BackendID) String() string {
	if id.Domain == "" || id.Name == "" {
		return ""
	}
	return string(id.Domain) + "/" + id.Name
}

// ParseBackendID parses a domain/backend pair without consulting a Catalog.
// Call Catalog.LookupPair when membership validation is also required.
func ParseBackendID(pair string) (BackendID, bool) {
	domain, name, ok := strings.Cut(strings.TrimSpace(pair), "/")
	if !ok || domain == "" || name == "" || strings.Contains(name, "/") {
		return BackendID{}, false
	}
	return BackendID{Domain: Domain(domain), Name: name}, true
}

// Capability is one operation a backend can participate in. Interfaces stay
// capability-specific; this metadata lets transports and application services
// discover support without type switches.
type Capability string

const (
	CapabilityEnvGet         Capability = "env/get"
	CapabilityEnvSet         Capability = "env/set"
	CapabilityEnvList        Capability = "env/list"
	CapabilityEnvPull        Capability = "env/pull"
	CapabilityEnvInject      Capability = "env/inject"
	CapabilityScaffold       Capability = "scaffold"
	CapabilityContainerInfo  Capability = "container/info"
	CapabilityContainerBuild Capability = "container/build"
	CapabilityContainerPush  Capability = "container/push"
	CapabilityDeploy         Capability = "deploy"
)

// Trait describes a shared wire/protocol family that is orthogonal to a
// capability. It lets typed storage and adapters share implementation details
// without maintaining a second backend identity list.
type Trait string

const (
	TraitS3Compatible Trait = "s3-compatible"
	TraitOCIRegistry  Trait = "oci-registry"
)

// RequirementKind describes a dependency that must be satisfied before a
// backend operation begins.
type RequirementKind string

const (
	RequirementBinary     RequirementKind = "binary"
	RequirementCapability RequirementKind = "capability"
	RequirementProfile    RequirementKind = "profile"
)

// Requirement is a declarative coeffect. The first implementation validates
// it before dispatch; long-running commands may later react to changes.
type Requirement struct {
	Kind     RequirementKind `json:"kind"`
	Name     string          `json:"name"`
	Optional bool            `json:"optional,omitempty"`
}

// FieldType is the transport-neutral form control for a profile field.
type FieldType string

const (
	FieldString  FieldType = "string"
	FieldSecret  FieldType = "secret"
	FieldBoolean FieldType = "boolean"
)

// FieldSpec describes a leaf in the existing typed profile JSON shape. Path
// uses slash-separated JSON keys so credentials remain nested on the wire;
// InputName is the stable transport input name used by CLI flags and other
// clients that need a non-localized field identifier.
type FieldSpec struct {
	Path        string    `json:"path"`
	InputName   string    `json:"input_name"`
	Type        FieldType `json:"type"`
	LabelKey    string    `json:"label_key"`
	Required    bool      `json:"required,omitempty"`
	Placeholder string    `json:"placeholder,omitempty"`
	Default     any       `json:"default,omitempty"`
}

// ProfileType identifies the typed profile shape used by a backend. It is an
// internal schema discriminator, not a user-facing backend identity. Multiple
// backends can share one type (for example every S3-compatible backend), which
// lets profile workflows dispatch once per shape instead of once per backend.
type ProfileType string

const (
	ProfileTypeDotenv     ProfileType = "dotenv"
	ProfileTypeInfisical  ProfileType = "infisical"
	ProfileTypeS3         ProfileType = "s3"
	ProfileTypeKustomize  ProfileType = "kustomize"
	ProfileTypeVercel     ProfileType = "vercel"
	ProfileTypeCloudflare ProfileType = "cloudflare"
	ProfileTypeEdgeOne    ProfileType = "edgeone"
	ProfileTypeContainer  ProfileType = "container"
)

// ProfileSpec describes whether and how a machine profile is configured for
// a backend. It contains schema metadata only, never profile values.
type ProfileSpec struct {
	Configurable bool        `json:"configurable"`
	Type         ProfileType `json:"-"`
	Fields       []FieldSpec `json:"fields,omitempty"`
}

// ProjectFieldType is the transport-neutral control used to edit one
// backend-owned value in projects[i].domains.<domain>.config. It is separate
// from FieldType because project settings are safe workspace metadata, while
// profile fields may contain machine-local credentials.
type ProjectFieldType string

const (
	ProjectFieldString      ProjectFieldType = "string"
	ProjectFieldEnvironment ProjectFieldType = "environment"
)

// ProjectFieldSpec describes a safe, backend-owned project setting. Path is
// slash-separated and relative to the backend config object; InputName is a
// stable, non-localized form identifier. Environment fields are rendered from
// the workspace's declared environment names by clients such as Dashboard.
type ProjectFieldSpec struct {
	Path        string           `json:"path"`
	InputName   string           `json:"input_name"`
	Type        ProjectFieldType `json:"type"`
	LabelKey    string           `json:"label_key"`
	Required    bool             `json:"required,omitempty"`
	Placeholder string           `json:"placeholder,omitempty"`
}

// ProjectSpec describes the backend-owned fields that may be persisted in a
// project's manifest config. It contains schema metadata only, never values or
// credentials. Container's common kind/image/namespace settings deliberately
// stay outside this backend-specific schema.
type ProjectSpec struct {
	Configurable bool               `json:"configurable"`
	Fields       []ProjectFieldSpec `json:"fields,omitempty"`
}

// BackendSpec is the immutable descriptor shared by CLI, HTTP, Dashboard,
// validation, and backend dispatch.
type BackendSpec struct {
	ID           BackendID     `json:"-"`
	Pair         string        `json:"id"`
	Capabilities []Capability  `json:"capabilities"`
	Traits       []Trait       `json:"traits,omitempty"`
	Requirements []Requirement `json:"requirements,omitempty"`
	Profile      ProfileSpec   `json:"profile"`
	Project      ProjectSpec   `json:"project"`
}

// MarshalJSON exposes the normalized ID components without storing a second,
// potentially inconsistent copy on BackendSpec. Pair remains the compatibility
// identity used by profile storage; domain and name make the catalog directly
// consumable by transports such as the Dashboard.
func (s BackendSpec) MarshalJSON() ([]byte, error) {
	type wireBackendSpec struct {
		Pair         string        `json:"id"`
		Domain       Domain        `json:"domain"`
		Name         string        `json:"name"`
		Capabilities []Capability  `json:"capabilities"`
		Traits       []Trait       `json:"traits,omitempty"`
		Requirements []Requirement `json:"requirements,omitempty"`
		Profile      ProfileSpec   `json:"profile"`
		Project      ProjectSpec   `json:"project"`
	}
	return json.Marshal(wireBackendSpec{
		Pair:         s.Pair,
		Domain:       s.ID.Domain,
		Name:         s.ID.Name,
		Capabilities: s.Capabilities,
		Traits:       s.Traits,
		Requirements: s.Requirements,
		Profile:      s.Profile,
		Project:      s.Project,
	})
}

// Has reports whether the backend declares capability.
func (s BackendSpec) Has(capability Capability) bool {
	for _, item := range s.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

// HasTrait reports whether the backend belongs to a shared protocol family.
func (s BackendSpec) HasTrait(trait Trait) bool {
	for _, item := range s.Traits {
		if item == trait {
			return true
		}
	}
	return false
}
