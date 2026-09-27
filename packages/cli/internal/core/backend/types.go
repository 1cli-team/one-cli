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
	DomainEnv Domain = "env"
)

// Built-in backend names are declared beside the Catalog so other packages do
// not create competing identity vocabularies. A package may still branch on a
// name when it owns genuinely different compiled behavior.
const (
	EnvDotenv    = "dotenv"
	EnvInfisical = "infisical"
)

// Domains is the stable product display order.
func Domains() []Domain {
	return []Domain{DomainEnv}
}

// BackendID is the canonical identity of one backend. String renders the
// transport identity used by the backend catalog.
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
	CapabilityEnvGet    Capability = "env/get"
	CapabilityEnvSet    Capability = "env/set"
	CapabilityEnvDelete Capability = "env/delete"
	CapabilityEnvList   Capability = "env/list"
	CapabilityEnvPull   Capability = "env/pull"
	CapabilityEnvInject Capability = "env/inject"
	CapabilityScaffold  Capability = "scaffold"
)

// Trait describes a shared wire/protocol family that is orthogonal to a
// capability. It lets typed storage and adapters share implementation details
// without maintaining a second backend identity list.
type Trait string

const ()

// RequirementKind describes a dependency that must be satisfied before a
// backend operation begins.
type RequirementKind string

const (
	RequirementBinary     RequirementKind = "binary"
	RequirementCapability RequirementKind = "capability"
)

// Requirement is a declarative coeffect. The first implementation validates
// it before dispatch; long-running commands may later react to changes.
type Requirement struct {
	Kind     RequirementKind `json:"kind"`
	Name     string          `json:"name"`
	Optional bool            `json:"optional,omitempty"`
}

// ProjectFieldType describes safe workspace metadata.
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
// credentials.
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
	Project      ProjectSpec   `json:"project"`
}

// MarshalJSON exposes the normalized ID components without storing a second,
// potentially inconsistent copy on BackendSpec. Pair remains the compatibility
// identity used by the backend catalog; domain and name make the catalog directly
// consumable by transports such as the Dashboard.
func (s BackendSpec) MarshalJSON() ([]byte, error) {
	type wireBackendSpec struct {
		Pair         string        `json:"id"`
		Domain       Domain        `json:"domain"`
		Name         string        `json:"name"`
		Capabilities []Capability  `json:"capabilities"`
		Traits       []Trait       `json:"traits,omitempty"`
		Requirements []Requirement `json:"requirements,omitempty"`
		Project      ProjectSpec   `json:"project"`
	}
	return json.Marshal(wireBackendSpec{
		Pair:         s.Pair,
		Domain:       s.ID.Domain,
		Name:         s.ID.Name,
		Capabilities: s.Capabilities,
		Traits:       s.Traits,
		Requirements: s.Requirements,
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
