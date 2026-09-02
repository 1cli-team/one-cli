// Package manifest owns the Dashboard's narrow, review-before-publish write
// boundary for one.manifest.json. Workspace projections remain read-only.
package manifest

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

const ManifestApplySchema = "one-cli/workspace-manifest-apply/v1"

var (
	ErrInvalidInput     = fmt.Errorf("manifest: invalid input")
	ErrProjectNotFound  = fmt.Errorf("manifest: project not found")
	ErrManifestConflict = fmt.Errorf("manifest: revision conflict")
)

type Service struct {
	catalog *catalog.Catalog
	mu      sync.Mutex
}

func NewService(backendCatalog *catalog.Catalog) (*Service, error) {
	if backendCatalog == nil {
		return nil, fmt.Errorf("manifest: backend catalog is required")
	}
	return &Service{catalog: backendCatalog}, nil
}

type ManifestConflict struct {
	Expected string
	Current  string
}

func (e *ManifestConflict) Error() string {
	return fmt.Sprintf("%v: expected %s, current %s", ErrManifestConflict, e.Expected, e.Current)
}

func (e *ManifestConflict) Unwrap() error { return ErrManifestConflict }

type ProjectGeneralPatch struct {
	BuildVersion string `json:"buildVersion"`
	DevCommand   string `json:"devCommand"`
}

type ProjectEnvironmentPatch struct {
	Path     string `json:"path"`
	Inherits bool   `json:"inherits"`
	Disabled bool   `json:"disabled"`
}

type ProjectContainerPatch struct {
	Enabled   bool   `json:"enabled"`
	Backend   string `json:"backend"`
	Image     string `json:"image"`
	Namespace string `json:"namespace"`
}

type ProjectDeployPatch struct {
	Backend string         `json:"backend"`
	Config  map[string]any `json:"config"`
}

// ProjectManifestPatch is intentionally a whitelist rather than a partial
// Manifest. Browser clients can only update the user-facing project settings
// represented here; identity, paths, toolchains and unknown backend config
// never cross the write boundary.
type ProjectManifestPatch struct {
	Project     string                   `json:"project"`
	General     *ProjectGeneralPatch     `json:"general,omitempty"`
	Environment *ProjectEnvironmentPatch `json:"environment,omitempty"`
	Container   *ProjectContainerPatch   `json:"container,omitempty"`
	Deploy      *ProjectDeployPatch      `json:"deploy,omitempty"`
}

type ApplyManifestInput struct {
	Revision string                 `json:"revision"`
	Changes  []ProjectManifestPatch `json:"changes"`
}

type ApplyManifestResult struct {
	Schema   string `json:"schema"`
	Revision string `json:"revision"`
	Applied  int    `json:"applied"`
}

// ApplyManifestDraft validates and publishes one browser draft in a single
// atomic manifest write. Revision comparison happens immediately before the
// in-memory patch is applied, so stale Dashboard tabs fail closed.
func (s *Service) ApplyManifestDraft(
	ctx context.Context,
	root string,
	input ApplyManifestInput,
) (ApplyManifestResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(input.Revision) == "" || len(input.Changes) == 0 {
		return ApplyManifestResult{}, fmt.Errorf("%w: revision and at least one change are required", ErrInvalidInput)
	}
	manifest, currentRevision, err := workspacecore.ReadManifestSnapshot(root)
	if err != nil {
		return ApplyManifestResult{}, err
	}
	if input.Revision != currentRevision {
		return ApplyManifestResult{}, &ManifestConflict{Expected: input.Revision, Current: currentRevision}
	}

	var registry *template.Registry
	for _, change := range input.Changes {
		if change.Deploy != nil {
			registry, err = template.Fetch(ctx, "")
			if err != nil {
				return ApplyManifestResult{}, err
			}
			break
		}
	}

	seen := make(map[string]struct{}, len(input.Changes))
	applied := 0
	for _, change := range input.Changes {
		name := strings.TrimSpace(change.Project)
		if name == "" {
			return ApplyManifestResult{}, fmt.Errorf("%w: project is required", ErrInvalidInput)
		}
		if _, duplicate := seen[name]; duplicate {
			return ApplyManifestResult{}, fmt.Errorf("%w: duplicate project patch %q", ErrInvalidInput, name)
		}
		seen[name] = struct{}{}
		project := findProject(manifest, name)
		if project == nil {
			return ApplyManifestResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, name)
		}
		if change.General == nil && change.Environment == nil && change.Container == nil && change.Deploy == nil {
			return ApplyManifestResult{}, fmt.Errorf("%w: project %q has no changes", ErrInvalidInput, name)
		}

		if change.General != nil {
			ensureProjectDomains(project)
			project.BuildVersion = workspacecore.NormalizeBuildVersion(change.General.BuildVersion)
			command := strings.TrimSpace(change.General.DevCommand)
			if command == "" {
				project.Domains.Dev = nil
			} else {
				project.Domains.Dev = &workspacecore.ProjectDevOverride{Command: command}
			}
			applied++
		}
		if change.Environment != nil {
			if strings.Contains(change.Environment.Path, "\x00") || unsafeSecretPath(change.Environment.Path) {
				return ApplyManifestResult{}, fmt.Errorf("%w: project %q has an unsafe environment path", ErrInvalidInput, name)
			}
			ensureProjectDomains(project)
			inherits := change.Environment.Inherits
			keys := []string(nil)
			if project.Domains.Env != nil {
				keys = append(keys, project.Domains.Env.Keys...)
			}
			project.Domains.Env = &workspacecore.ProjectEnvOverride{
				Path: strings.TrimSpace(change.Environment.Path), Inherits: &inherits,
				Disabled: change.Environment.Disabled, Keys: keys,
			}
			applied++
		}
		if change.Container != nil {
			if err := s.applyContainerPatch(project, change.Container); err != nil {
				return ApplyManifestResult{}, err
			}
			applied++
		}
		if change.Deploy != nil {
			if err := s.applyDeployPatch(manifest, project, change.Deploy, registry); err != nil {
				return ApplyManifestResult{}, err
			}
			applied++
		}
	}

	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		return ApplyManifestResult{}, err
	}
	_, revision, err := workspacecore.ReadManifestSnapshot(root)
	if err != nil {
		return ApplyManifestResult{}, err
	}
	return ApplyManifestResult{Schema: ManifestApplySchema, Revision: revision, Applied: applied}, nil
}

func ensureProjectDomains(project *workspacecore.ManifestProject) {
	if project.Domains == nil {
		project.Domains = &workspacecore.ProjectDomains{}
	}
}

func findProject(manifest *workspacecore.Manifest, name string) *workspacecore.ManifestProject {
	if manifest == nil {
		return nil
	}
	for index := range manifest.Projects {
		if manifest.Projects[index].Name == name {
			return &manifest.Projects[index]
		}
	}
	return nil
}

func unsafeSecretPath(value string) bool {
	for _, part := range strings.Split(strings.ReplaceAll(value, "\\", "/"), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func (s *Service) applyContainerPatch(
	project *workspacecore.ManifestProject,
	patch *ProjectContainerPatch,
) error {
	ensureProjectDomains(project)
	if !patch.Enabled {
		project.Domains.Container = nil
		return nil
	}
	backend := strings.TrimSpace(patch.Backend)
	if _, ok := s.catalog.Lookup(catalog.DomainContainer, backend); !ok {
		return fmt.Errorf("%w: unknown container backend %q", ErrInvalidInput, backend)
	}
	project.Domains.Container = &workspacecore.ProjectContainerOverride{
		Kind: backend, Image: strings.TrimSpace(patch.Image), Namespace: strings.TrimSpace(patch.Namespace),
	}
	return nil
}

func (s *Service) applyDeployPatch(
	manifest *workspacecore.Manifest,
	project *workspacecore.ManifestProject,
	patch *ProjectDeployPatch,
	registry *template.Registry,
) error {
	ensureProjectDomains(project)
	backend := strings.TrimSpace(patch.Backend)
	if backend == "" {
		project.Domains.Deploy = nil
		return nil
	}
	spec, ok := s.catalog.Lookup(catalog.DomainDeploy, backend)
	if !ok || !spec.Project.Configurable {
		return fmt.Errorf("%w: unknown or non-configurable deploy backend %q", ErrInvalidInput, backend)
	}
	compatible := projectCompatibleDeployTargets(registry, project.TemplateID)
	if len(compatible) > 0 && !containsString(compatible, backend) {
		return fmt.Errorf("%w: deploy backend %q is incompatible with project %q", ErrInvalidInput, backend, project.Name)
	}
	existing := projectDeployConfig(project)
	if project.Domains.Deploy == nil || strings.TrimSpace(project.Domains.Deploy.Kind) != backend {
		existing = nil
	}
	raw, err := mergeProjectConfig(existing, patch.Config, spec.Project.Fields, manifest)
	if err != nil {
		return fmt.Errorf("%w: project %q deploy config: %v", ErrInvalidInput, project.Name, err)
	}
	project.Domains.Deploy = &workspacecore.ProjectDeployBackend{Kind: backend, Config: raw}
	return nil
}

func projectCompatibleDeployTargets(registry *template.Registry, templateID string) []string {
	if registry == nil {
		return nil
	}
	for _, entry := range registry.Templates {
		if entry.ID == templateID {
			return append([]string(nil), entry.Compat[string(catalog.DomainDeploy)]...)
		}
	}
	return nil
}

func projectDeployConfig(project *workspacecore.ManifestProject) json.RawMessage {
	if project.Domains == nil || project.Domains.Deploy == nil {
		return nil
	}
	return project.Domains.Deploy.Config
}

func mergeProjectConfig(
	existing json.RawMessage,
	input map[string]any,
	fields []catalog.ProjectFieldSpec,
	manifest *workspacecore.Manifest,
) (json.RawMessage, error) {
	object := map[string]any{}
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &object); err != nil || object == nil {
			return nil, fmt.Errorf("existing config is not an object")
		}
	}
	allowed := make(map[string]catalog.ProjectFieldSpec, len(fields))
	for _, field := range fields {
		allowed[field.Path] = field
	}
	flat := map[string]any{}
	flattenConfig("", input, flat)
	for path, value := range flat {
		field, ok := allowed[path]
		if !ok {
			return nil, fmt.Errorf("field %q is not configurable", path)
		}
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("field %q must be a string", path)
		}
		text = strings.TrimSpace(text)
		if field.Required && text == "" {
			return nil, fmt.Errorf("field %q is required", path)
		}
		if field.Type == catalog.ProjectFieldEnvironment && text != "" && !manifestHasEnvironment(manifest, text) {
			return nil, fmt.Errorf("environment %q is not declared", text)
		}
		if text == "" {
			deleteConfigPath(object, path)
		} else {
			setConfigPath(object, path, text)
		}
	}
	if len(object) == 0 {
		return nil, nil
	}
	return json.Marshal(object)
}

func flattenConfig(prefix string, input map[string]any, out map[string]any) {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		path := key
		if prefix != "" {
			path = prefix + "/" + key
		}
		if child, ok := input[key].(map[string]any); ok {
			flattenConfig(path, child, out)
			continue
		}
		out[path] = input[key]
	}
}

func setConfigPath(object map[string]any, path string, value string) {
	parts := strings.Split(path, "/")
	current := object
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func deleteConfigPath(object map[string]any, path string) {
	parts := strings.Split(path, "/")
	current := object
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			return
		}
		current = next
	}
	delete(current, parts[len(parts)-1])
}

func manifestHasEnvironment(manifest *workspacecore.Manifest, value string) bool {
	if manifest == nil || manifest.Environments == nil {
		return containsString(workspacecore.DefaultEnvironments, value)
	}
	return containsString(manifest.Environments.Names, value)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
