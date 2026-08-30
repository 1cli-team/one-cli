package profile

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
)

// SectionSnapshot is the transport-neutral view of one typed schema-v1
// section. Payload retains the concrete Section[T] value for JSON rendering;
// callers do not need to know which T belongs to a backend.
type SectionSnapshot struct {
	Payload any
	Names   []string
	Default string
}

// ValidateCatalog verifies that every profile-bearing backend maps to exactly
// one persisted schema-v1 section and that its form paths match the typed Go
// payload. This is the composition-time guard against Catalog/profile drift.
func ValidateCatalog(backendCatalog *catalog.Catalog) error {
	if backendCatalog == nil {
		return fmt.Errorf("profile: backend catalog is required")
	}
	for _, spec := range backendCatalog.All() {
		if spec.Profile.Type == "" {
			if spec.Profile.Configurable {
				return fmt.Errorf("profile: backend %s has no profile type", spec.Pair)
			}
			continue
		}
		policy, ok := policyForSpec(spec)
		if !ok {
			return fmt.Errorf(
				"profile: backend %s has no schema-v1 policy for type %q",
				spec.Pair,
				spec.Profile.Type,
			)
		}
		for _, field := range spec.Profile.Fields {
			leaf, ok := profileJSONPathType(policy.valueType, field.Path)
			if !ok {
				return fmt.Errorf(
					"profile: backend %s field %q is absent from %s",
					spec.Pair,
					field.Path,
					policy.valueType,
				)
			}
			if field.Type == catalog.FieldBoolean && leaf.Kind() != reflect.Bool {
				return fmt.Errorf("profile: backend %s field %q must be boolean", spec.Pair, field.Path)
			}
			if (field.Type == catalog.FieldString || field.Type == catalog.FieldSecret) && leaf.Kind() != reflect.String {
				return fmt.Errorf("profile: backend %s field %q must be string", spec.Pair, field.Path)
			}
		}
	}
	return nil
}

// InspectSection returns the typed persisted section selected by spec without
// making application code repeat the profile-shape dispatch table.
func InspectSection(config *Config, spec catalog.BackendSpec) (SectionSnapshot, bool) {
	if config == nil {
		return SectionSnapshot{}, false
	}
	policy, ok := policyForSpec(spec)
	if !ok {
		return SectionSnapshot{}, false
	}
	return SectionSnapshot{
		Payload: policy.configValue(config),
		Names:   policy.names(config),
		Default: policy.defaultName(config),
	}, true
}

// LookupStored returns one profile from the section selected by spec.
func LookupStored(config *Config, spec catalog.BackendSpec, name string) (Profile, bool) {
	if config == nil {
		return Profile{}, false
	}
	policy, ok := policyForSpec(spec)
	if !ok {
		return Profile{}, false
	}
	return policy.lookup(config, name)
}

// Payload returns the concrete typed payload carried by the profile union.
func Payload(spec catalog.BackendSpec, value Profile) (any, bool) {
	policy, ok := policyForSpec(spec)
	if !ok {
		return nil, false
	}
	return policy.payload(value)
}

// CredentialSource returns the typed credential-source discriminator for a
// profile. Profile shapes without credentials deliberately return empty.
func CredentialSource(spec catalog.BackendSpec, value Profile) string {
	policy, ok := policyForSpec(spec)
	if !ok {
		return ""
	}
	return policy.credentialSource(value)
}

// Decode decodes a backend's JSON payload into the typed profile union.
func Decode(spec catalog.BackendSpec, raw json.RawMessage) (Profile, error) {
	value := Profile{Backend: spec.ID.Name}
	if err := ReplacePayload(spec, &value, raw); err != nil {
		return Profile{}, err
	}
	return value, nil
}

// ReplacePayload decodes and replaces only the payload selected by spec. It
// intentionally preserves Profile.Backend so masking can rewrite a union
// value without changing its selected backend identity.
func ReplacePayload(spec catalog.BackendSpec, value *Profile, raw json.RawMessage) error {
	if value == nil {
		return fmt.Errorf("profile: destination is required")
	}
	policy, ok := policyForSpec(spec)
	if !ok {
		return fmt.Errorf("profile: backend %s has no schema-v1 policy", spec.Pair)
	}
	if err := policy.setPayload(value, raw); err != nil {
		return fmt.Errorf("profile: decode %s payload: %w", spec.Pair, err)
	}
	return nil
}

func policyForSpec(spec catalog.BackendSpec) (*sectionPolicy, bool) {
	policy, ok := schemaPoliciesByPair[spec.Pair]
	return policy, ok && policy.spec.Profile.Type == spec.Profile.Type
}

func profileJSONPathType(root reflect.Type, path string) (reflect.Type, bool) {
	current := root
	for _, part := range strings.Split(path, "/") {
		if part == "" {
			return nil, false
		}
		for current.Kind() == reflect.Pointer {
			current = current.Elem()
		}
		if current.Kind() != reflect.Struct {
			return nil, false
		}
		found := false
		for index := 0; index < current.NumField(); index++ {
			field := current.Field(index)
			if strings.Split(field.Tag.Get("json"), ",")[0] == part {
				current = field.Type
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	for current.Kind() == reflect.Pointer {
		current = current.Elem()
	}
	return current, true
}
