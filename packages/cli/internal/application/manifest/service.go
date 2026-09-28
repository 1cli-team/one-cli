// Package manifest owns the Dashboard's narrow, review-before-publish write
// boundary for one.manifest.json. Workspace projections remain read-only.
package manifest

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

const (
	ManifestApplySchema   = "one-cli/workspace-manifest-apply/v1"
	ManifestPreviewSchema = "one-cli/workspace-manifest-preview/v1"
)

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

// ProjectManifestPatch is intentionally a whitelist rather than a partial
// Manifest. Browser clients can only update the user-facing project settings
// represented here; identity, paths, toolchains and unknown backend config
// never cross the write boundary.
type ProjectManifestPatch struct {
	Project     string                   `json:"project"`
	General     *ProjectGeneralPatch     `json:"general,omitempty"`
	Environment *ProjectEnvironmentPatch `json:"environment,omitempty"`
}

type WorkspaceEnvironmentPatch struct {
	Backend     string  `json:"backend"`
	ProjectID   *string `json:"projectId,omitempty"`
	ProjectName *string `json:"projectName,omitempty"`
	SiteURL     *string `json:"siteUrl,omitempty"`
}

type WorkspaceManifestPatch struct {
	Environment *WorkspaceEnvironmentPatch `json:"environment,omitempty"`
}

type ApplyManifestInput struct {
	Workspace *WorkspaceManifestPatch `json:"workspace,omitempty"`
	Revision  string                  `json:"revision"`
	Changes   []ProjectManifestPatch  `json:"changes"`
}

type ApplyManifestResult struct {
	Schema   string `json:"schema"`
	Revision string `json:"revision"`
	Applied  int    `json:"applied"`
}

type PreviewManifestInput struct {
	Revision  string                  `json:"revision"`
	Workspace *WorkspaceManifestPatch `json:"workspace,omitempty"`
	Changes   []ProjectManifestPatch  `json:"changes"`
}

type PreviewManifestResult struct {
	Schema   string `json:"schema"`
	Revision string `json:"revision"`
	Before   string `json:"before"`
	After    string `json:"after"`
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

	hasWorkspaceChange := input.Workspace != nil && input.Workspace.Environment != nil
	if strings.TrimSpace(input.Revision) == "" || (!hasWorkspaceChange && len(input.Changes) == 0) {
		return ApplyManifestResult{}, fmt.Errorf("%w: revision and at least one change are required", ErrInvalidInput)
	}
	manifest, currentRevision, err := workspacecore.ReadManifestSnapshot(root)
	if err != nil {
		return ApplyManifestResult{}, err
	}
	if input.Revision != currentRevision {
		return ApplyManifestResult{}, &ManifestConflict{Expected: input.Revision, Current: currentRevision}
	}

	if hasWorkspaceChange {
		if err := applyWorkspaceEnvironmentPatch(manifest, input.Workspace.Environment); err != nil {
			return ApplyManifestResult{}, err
		}
	}
	applied, err := s.applyProjectChanges(ctx, manifest, input.Changes)
	if err != nil {
		return ApplyManifestResult{}, err
	}

	if hasWorkspaceChange {
		applied++
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

// PreviewManifestDraft renders the complete current and proposed manifest
// without writing either version. It shares the same project patch path as
// ApplyManifestDraft so the Dashboard can show a faithful full-file diff.
func (s *Service) PreviewManifestDraft(
	ctx context.Context,
	root string,
	input PreviewManifestInput,
) (PreviewManifestResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	hasWorkspaceChange := input.Workspace != nil && input.Workspace.Environment != nil
	if strings.TrimSpace(input.Revision) == "" || (!hasWorkspaceChange && len(input.Changes) == 0) {
		return PreviewManifestResult{}, fmt.Errorf("%w: revision and at least one change (workspace or project) are required", ErrInvalidInput)
	}
	manifest, currentRevision, err := workspacecore.ReadManifestSnapshot(root)
	if err != nil {
		return PreviewManifestResult{}, err
	}
	if input.Revision != currentRevision {
		return PreviewManifestResult{}, &ManifestConflict{Expected: input.Revision, Current: currentRevision}
	}
	before, err := workspacecore.MarshalManifest(manifest)
	if err != nil {
		return PreviewManifestResult{}, err
	}
	if hasWorkspaceChange {
		if err := applyWorkspaceEnvironmentPatch(manifest, input.Workspace.Environment); err != nil {
			return PreviewManifestResult{}, err
		}
	}
	if _, err := s.applyProjectChanges(ctx, manifest, input.Changes); err != nil {
		return PreviewManifestResult{}, err
	}
	after, err := workspacecore.MarshalManifest(manifest)
	if err != nil {
		return PreviewManifestResult{}, err
	}
	return PreviewManifestResult{
		Schema: ManifestPreviewSchema, Revision: currentRevision,
		Before: string(before), After: string(after),
	}, nil
}

func applyWorkspaceEnvironmentPatch(manifest *workspacecore.Manifest, patch *WorkspaceEnvironmentPatch) error {
	backend := strings.TrimSpace(patch.Backend)
	if backend != workspacecore.EnvBackendInfisical {
		return fmt.Errorf("%w: unknown environment backend %q", ErrInvalidInput, backend)
	}
	if manifest.Env == nil {
		manifest.Env = &workspacecore.EnvironmentConfig{}
	}
	if patch.ProjectID != nil {
		if strings.TrimSpace(*patch.ProjectID) == "" {
			return fmt.Errorf("%w: projectId is required", ErrInvalidInput)
		}
		manifest.Env.ProjectID = strings.TrimSpace(*patch.ProjectID)
	}
	if patch.ProjectName != nil {
		manifest.Env.ProjectName = *patch.ProjectName
	}
	if patch.SiteURL != nil {
		u, err := url.Parse(*patch.SiteURL)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
			return fmt.Errorf("%w: invalid Infisical instance URL", ErrInvalidInput)
		}
		manifest.Env.SiteURL = u.String()
	}
	return nil
}

func (s *Service) applyProjectChanges(
	ctx context.Context,
	manifest *workspacecore.Manifest,
	changes []ProjectManifestPatch,
) (int, error) {
	seen := make(map[string]struct{}, len(changes))
	applied := 0
	for _, change := range changes {
		name := strings.TrimSpace(change.Project)
		if name == "" {
			return 0, fmt.Errorf("%w: project is required", ErrInvalidInput)
		}
		if _, duplicate := seen[name]; duplicate {
			return 0, fmt.Errorf("%w: duplicate project patch %q", ErrInvalidInput, name)
		}
		seen[name] = struct{}{}
		project := findProject(manifest, name)
		if project == nil {
			return 0, fmt.Errorf("%w: %s", ErrProjectNotFound, name)
		}
		if change.General == nil && change.Environment == nil {
			return 0, fmt.Errorf("%w: project %q has no changes", ErrInvalidInput, name)
		}

		if change.General != nil {
			project.BuildVersion = workspacecore.NormalizeBuildVersion(change.General.BuildVersion)
			command := strings.TrimSpace(change.General.DevCommand)
			if command == "" {
				project.Dev = nil
			} else {
				project.Dev = &workspacecore.ProjectDevOverride{Command: command}
			}
			applied++
		}
		if change.Environment != nil {
			if strings.Contains(change.Environment.Path, "\x00") || unsafeSecretPath(change.Environment.Path) {
				return 0, fmt.Errorf("%w: project %q has an unsafe environment path", ErrInvalidInput, name)
			}
			inherits := change.Environment.Inherits
			keys := []string(nil)
			if project.Env != nil {
				keys = append(keys, project.Env.Keys...)
			}
			project.Env = &workspacecore.ProjectEnvOverride{
				Path: strings.TrimSpace(change.Environment.Path), Inherits: &inherits,
				Disabled: change.Environment.Disabled, Keys: keys,
			}
			applied++
		}
	}
	return applied, nil
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
