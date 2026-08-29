package configure

import (
	"encoding/json"
	"fmt"
	"strings"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
)

const MaskedCredential = "********"

// MaskConfig applies the HTTP disclosure policy declared by the Catalog:
// fields marked secret are hidden, while non-secret account identifiers stay
// visible. JSON rewriting is deliberate here—the Catalog paths are JSON paths,
// so masking validates the same shape consumed by the Dashboard.
func (s *ProfileService) MaskConfig(config profile.Config) (profile.Config, error) {
	document, err := jsonObject(config)
	if err != nil {
		return profile.Config{}, fmt.Errorf("application: encode profile config for masking: %w", err)
	}
	for _, spec := range s.catalog.All() {
		paths := profileFieldPaths(spec.Profile.Fields, func(field catalog.FieldSpec) bool {
			return field.Type == catalog.FieldSecret
		})
		if len(paths) == 0 {
			continue
		}
		section, ok := objectValue(document[spec.Pair])
		if !ok {
			continue
		}
		profiles, ok := objectValue(section["profiles"])
		if !ok {
			continue
		}
		for _, value := range profiles {
			payload, ok := objectValue(value)
			if !ok {
				continue
			}
			maskJSONPaths(payload, paths)
		}
	}

	raw, err := json.Marshal(document)
	if err != nil {
		return profile.Config{}, fmt.Errorf("application: encode masked profile config: %w", err)
	}
	var masked profile.Config
	if err := json.Unmarshal(raw, &masked); err != nil {
		return profile.Config{}, fmt.Errorf("application: decode masked profile config: %w", err)
	}
	return masked, nil
}

// MaskProfile applies the stricter CLI `configure show` policy. Every field
// nested below credentials is hidden, including account identifiers. The set
// of paths still comes from the Catalog rather than from backend switches.
func (s *ProfileService) MaskProfile(value profile.Profile) (profile.Profile, error) {
	result := value
	seen := make(map[catalog.ProfileType]struct{})
	for _, spec := range s.catalog.All() {
		profileType := spec.Profile.Type
		if _, ok := seen[profileType]; ok {
			continue
		}
		seen[profileType] = struct{}{}
		payload, ok := profile.Payload(spec, result)
		if !ok {
			continue
		}
		paths := s.profileTypePaths(profileType, func(field catalog.FieldSpec) bool {
			return strings.HasPrefix(field.Path, "credentials/")
		})
		if len(paths) == 0 {
			continue
		}
		document, err := jsonObject(payload)
		if err != nil {
			return profile.Profile{}, fmt.Errorf("application: encode %s profile for masking: %w", profileType, err)
		}
		maskJSONPaths(document, paths)
		raw, err := json.Marshal(document)
		if err != nil {
			return profile.Profile{}, fmt.Errorf("application: encode masked %s profile: %w", profileType, err)
		}
		if err := profile.ReplacePayload(spec, &result, raw); err != nil {
			return profile.Profile{}, fmt.Errorf("application: decode masked %s profile: %w", profileType, err)
		}
	}
	return result, nil
}

func (s *ProfileService) containsMaskedCredential(
	spec catalog.BackendSpec,
	value profile.Profile,
) (bool, error) {
	payload, ok := profile.Payload(spec, value)
	if !ok {
		return false, nil
	}
	document, err := jsonObject(payload)
	if err != nil {
		return false, fmt.Errorf("application: encode %s profile: %w", spec.Pair, err)
	}
	for _, path := range profileFieldPaths(spec.Profile.Fields, func(field catalog.FieldSpec) bool {
		return field.Type == catalog.FieldSecret
	}) {
		if current, ok := jsonPathValue(document, path); ok && current == MaskedCredential {
			return true, nil
		}
	}
	return false, nil
}

func (s *ProfileService) preserveMaskedCredentials(
	config *profile.Config,
	spec catalog.BackendSpec,
	name string,
	value *profile.Profile,
) error {
	if value == nil {
		return nil
	}
	incoming, ok := profile.Payload(spec, *value)
	if !ok {
		return nil
	}
	existingProfile, ok := profile.LookupStored(config, spec, name)
	if !ok {
		return nil
	}
	existing, ok := profile.Payload(spec, existingProfile)
	if !ok {
		return nil
	}
	incomingDocument, err := jsonObject(incoming)
	if err != nil {
		return fmt.Errorf("application: encode incoming %s profile: %w", spec.Pair, err)
	}
	existingDocument, err := jsonObject(existing)
	if err != nil {
		return fmt.Errorf("application: encode existing %s profile: %w", spec.Pair, err)
	}
	changed := false
	for _, path := range profileFieldPaths(spec.Profile.Fields, func(field catalog.FieldSpec) bool {
		return field.Type == catalog.FieldSecret
	}) {
		current, exists := jsonPathValue(incomingDocument, path)
		if !exists || current != MaskedCredential {
			continue
		}
		stored, exists := jsonPathValue(existingDocument, path)
		if !exists {
			continue
		}
		if replaceJSONPath(incomingDocument, path, stored) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	raw, err := json.Marshal(incomingDocument)
	if err != nil {
		return fmt.Errorf("application: encode preserved %s profile: %w", spec.Pair, err)
	}
	if err := profile.ReplacePayload(spec, value, raw); err != nil {
		return fmt.Errorf("application: decode preserved %s profile: %w", spec.Pair, err)
	}
	return nil
}

func (s *ProfileService) profileTypePaths(
	profileType catalog.ProfileType,
	include func(catalog.FieldSpec) bool,
) []string {
	seen := map[string]struct{}{}
	var paths []string
	for _, spec := range s.catalog.All() {
		if spec.Profile.Type != profileType {
			continue
		}
		for _, path := range profileFieldPaths(spec.Profile.Fields, include) {
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			paths = append(paths, path)
		}
	}
	return paths
}

func profileFieldPaths(
	fields []catalog.FieldSpec,
	include func(catalog.FieldSpec) bool,
) []string {
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		if include(field) {
			paths = append(paths, field.Path)
		}
	}
	return paths
}

func jsonObject(value any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	return document, nil
}

func objectValue(value any) (map[string]any, bool) {
	document, ok := value.(map[string]any)
	return document, ok
}

func maskJSONPaths(document map[string]any, paths []string) {
	for _, path := range paths {
		replaceJSONPath(document, path, MaskedCredential)
	}
}

func jsonPathValue(document map[string]any, path string) (any, bool) {
	parts := strings.Split(path, "/")
	current := document
	for index, part := range parts {
		value, ok := current[part]
		if !ok {
			return nil, false
		}
		if index == len(parts)-1 {
			return value, true
		}
		current, ok = objectValue(value)
		if !ok {
			return nil, false
		}
	}
	return nil, false
}

func replaceJSONPath(document map[string]any, path string, replacement any) bool {
	parts := strings.Split(path, "/")
	current := document
	for index, part := range parts {
		if index == len(parts)-1 {
			if _, ok := current[part]; !ok {
				return false
			}
			current[part] = replacement
			return true
		}
		next, ok := objectValue(current[part])
		if !ok {
			return false
		}
		current = next
	}
	return false
}
