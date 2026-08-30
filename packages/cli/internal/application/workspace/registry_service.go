package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	registryport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/workspaceregistry"
)

const WorkspaceRegistrySchema = "one-cli/workspaces/v1"

const (
	WorkspaceStatusReady            = "ready"
	WorkspaceStatusMissing          = "missing"
	WorkspaceStatusInvalid          = "invalid"
	WorkspaceStatusIdentityMissing  = "identity-missing"
	WorkspaceStatusIdentityConflict = "identity-conflict"
)

var (
	ErrRegistryEntryNotFound         = errors.New("workspace registry: entry not found")
	ErrRegistryEntryUnavailable      = errors.New("workspace registry: entry unavailable")
	ErrRegistryEntryIdentityConflict = errors.New("workspace registry: identity conflict")

	// Short aliases keep callers readable while preserving the explicit names
	// above for code that wants to distinguish registry errors from other
	// Workspace use-case errors.
	ErrRegistryUnavailable      = ErrRegistryEntryUnavailable
	ErrRegistryIdentityConflict = ErrRegistryEntryIdentityConflict
)

// WorkspaceRegistryResponse is the safe Dashboard projection of the local
// registry. Project configuration is intentionally omitted and must be read
// from each workspace's manifest after Resolve.
type WorkspaceRegistryResponse struct {
	Schema         string                `json:"schema"`
	CurrentEntryID string                `json:"currentEntryId,omitempty"`
	Workspaces     []RegisteredWorkspace `json:"workspaces"`
}

// RegisteredWorkspace is one inspected registry entry. ID, Name, Status and
// ProjectCount reflect the current manifest when it can be read; LastSeenAt
// remains the durable observation timestamp.
type RegisteredWorkspace struct {
	EntryID      string    `json:"entryId"`
	ID           string    `json:"id,omitempty"`
	Name         string    `json:"name"`
	Root         string    `json:"root"`
	Status       string    `json:"status"`
	ProjectCount int       `json:"projectCount"`
	LastSeenAt   time.Time `json:"lastSeenAt"`
}

// ResolvedWorkspace is the trusted server-side result of resolving an opaque
// entry ID. HTTP callers use Root for existing Workspace use cases and may
// reuse Manifest to avoid accepting or rediscovering a client-supplied path.
type ResolvedWorkspace struct {
	RegisteredWorkspace
	Manifest *workspacecore.Manifest `json:"-"`
}

// RegistryServiceOption customizes deterministic dependencies for tests.
type RegistryServiceOption func(*RegistryService)

// WithRegistryClock replaces the wall clock used for observation timestamps.
func WithRegistryClock(clock func() time.Time) RegistryServiceOption {
	return func(service *RegistryService) {
		service.now = clock
	}
}

// WithRegistryEntryIDGenerator replaces the opaque entry-ID generator.
func WithRegistryEntryIDGenerator(generator func() (string, error)) RegistryServiceOption {
	return func(service *RegistryService) {
		service.newEntryID = generator
	}
}

// RegistryService owns the machine-local Workspace observation, listing and
// trusted-resolution use cases.
type RegistryService struct {
	repository registryport.Repository
	now        func() time.Time
	newEntryID func() (string, error)
}

// NewRegistryService constructs the Workspace registry use-case boundary.
func NewRegistryService(
	repository registryport.Repository,
	options ...RegistryServiceOption,
) (*RegistryService, error) {
	if repository == nil {
		return nil, errors.New("workspace registry: repository is required")
	}
	service := &RegistryService{
		repository: repository,
		now:        time.Now,
		newEntryID: generateRegistryEntryID,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	if service.now == nil {
		return nil, errors.New("workspace registry: clock is required")
	}
	if service.newEntryID == nil {
		return nil, errors.New("workspace registry: entry ID generator is required")
	}
	return service, nil
}

// Observe validates root as a One Workspace and atomically records that it
// was seen by source (for example "create" or "serve"). Existing paths are
// idempotent. When a uniquely matching identity has disappeared from its old
// root, the entry is moved and keeps its EntryID.
func (s *RegistryService) Observe(
	ctx context.Context,
	root string,
	source string,
) (RegisteredWorkspace, error) {
	canonicalRoot, err := canonicalRegistryRoot(root)
	if err != nil {
		return RegisteredWorkspace{}, fmt.Errorf("%w: %v", ErrRegistryEntryUnavailable, err)
	}
	inspection := inspectWorkspaceRoot(canonicalRoot)
	if inspection.status == WorkspaceStatusMissing || inspection.status == WorkspaceStatusInvalid {
		return RegisteredWorkspace{}, fmt.Errorf(
			"%w: workspace %q has status %s",
			ErrRegistryEntryUnavailable,
			canonicalRoot,
			inspection.status,
		)
	}

	now := s.now().UTC()
	workspaceID := inspection.id
	name := inspection.name
	if name == "" {
		name = fallbackWorkspaceName(canonicalRoot)
	}
	var observedEntryID string
	err = s.repository.Update(ctx, func(registry *workspacecore.Registry) error {
		registry.Version = workspacecore.RegistrySchemaVersion

		for index := range registry.Workspaces {
			if rootsReferToSamePath(registry.Workspaces[index].Root, canonicalRoot) {
				updateObservedEntry(&registry.Workspaces[index], canonicalRoot, workspaceID, name, source, now)
				observedEntryID = registry.Workspaces[index].EntryID
				return nil
			}
		}

		if workspaceID != "" {
			matchingIndexes := make([]int, 0, 1)
			for index := range registry.Workspaces {
				if strings.TrimSpace(registry.Workspaces[index].WorkspaceID) == workspaceID {
					matchingIndexes = append(matchingIndexes, index)
				}
			}
			if len(matchingIndexes) == 1 {
				index := matchingIndexes[0]
				if registryRootMissing(registry.Workspaces[index].Root) {
					updateObservedEntry(&registry.Workspaces[index], canonicalRoot, workspaceID, name, source, now)
					observedEntryID = registry.Workspaces[index].EntryID
					return nil
				}
			}
		}

		entryID, idErr := s.uniqueEntryID(registry.Workspaces)
		if idErr != nil {
			return idErr
		}
		entry := workspacecore.RegistryEntry{
			EntryID:      entryID,
			WorkspaceID:  workspaceID,
			Name:         name,
			Root:         canonicalRoot,
			RegisteredAt: now,
			LastSeenAt:   now,
			LastSeenBy:   strings.TrimSpace(source),
		}
		registry.Workspaces = append(registry.Workspaces, entry)
		observedEntryID = entry.EntryID
		return nil
	})
	if err != nil {
		return RegisteredWorkspace{}, err
	}

	registry, err := s.repository.Load(ctx)
	if err != nil {
		return RegisteredWorkspace{}, err
	}
	response, _ := inspectRegistry(registry, canonicalRoot)
	for _, registered := range response.Workspaces {
		if registered.EntryID == observedEntryID {
			return registered, nil
		}
	}
	return RegisteredWorkspace{}, fmt.Errorf("%w: %s", ErrRegistryEntryNotFound, observedEntryID)
}

// List returns every registered Workspace with a live status computed from
// disk. It never silently removes stale paths.
func (s *RegistryService) List(
	ctx context.Context,
	currentRoot string,
) (WorkspaceRegistryResponse, error) {
	registry, err := s.repository.Load(ctx)
	if err != nil {
		return WorkspaceRegistryResponse{}, err
	}
	canonicalCurrent := ""
	if strings.TrimSpace(currentRoot) != "" {
		canonicalCurrent, err = canonicalRegistryRoot(currentRoot)
		if err != nil {
			return WorkspaceRegistryResponse{}, err
		}
	}
	response, _ := inspectRegistry(registry, canonicalCurrent)
	return response, nil
}

// Resolve turns only an opaque registered entry ID into a trusted root and
// freshly read manifest. Missing, invalid, legacy identity-less and
// conflicting entries are rejected.
func (s *RegistryService) Resolve(
	ctx context.Context,
	entryID string,
) (ResolvedWorkspace, error) {
	return s.resolve(ctx, entryID, false)
}

// ResolveRead resolves an entry for a read-only projection. Identity
// conflicts are safe to inspect because entryId still selects one canonical
// root; mutations continue to use Resolve and fail closed until the duplicate
// manifest identity is explicitly repaired.
func (s *RegistryService) ResolveRead(
	ctx context.Context,
	entryID string,
) (ResolvedWorkspace, error) {
	return s.resolve(ctx, entryID, true)
}

func (s *RegistryService) resolve(
	ctx context.Context,
	entryID string,
	allowIdentityConflict bool,
) (ResolvedWorkspace, error) {
	entryID = strings.TrimSpace(entryID)
	registry, err := s.repository.Load(ctx)
	if err != nil {
		return ResolvedWorkspace{}, err
	}
	response, manifests := inspectRegistry(registry, "")
	for _, registered := range response.Workspaces {
		if registered.EntryID != entryID {
			continue
		}
		switch registered.Status {
		case WorkspaceStatusIdentityConflict:
			if !allowIdentityConflict {
				return ResolvedWorkspace{}, fmt.Errorf(
					"%w: registry entry %q",
					ErrRegistryEntryIdentityConflict,
					entryID,
				)
			}
			fallthrough
		case WorkspaceStatusReady:
			manifest := manifests[entryID]
			if manifest == nil {
				return ResolvedWorkspace{}, fmt.Errorf(
					"%w: registry entry %q has no manifest",
					ErrRegistryEntryUnavailable,
					entryID,
				)
			}
			return ResolvedWorkspace{RegisteredWorkspace: registered, Manifest: manifest}, nil
		default:
			return ResolvedWorkspace{}, fmt.Errorf(
				"%w: registry entry %q has status %s",
				ErrRegistryEntryUnavailable,
				entryID,
				registered.Status,
			)
		}
	}
	return ResolvedWorkspace{}, fmt.Errorf("%w: %s", ErrRegistryEntryNotFound, entryID)
}

// Forget removes only the local registration. It never deletes or mutates the
// Workspace directory.
func (s *RegistryService) Forget(ctx context.Context, entryID string) error {
	entryID = strings.TrimSpace(entryID)
	return s.repository.Update(ctx, func(registry *workspacecore.Registry) error {
		for index := range registry.Workspaces {
			if registry.Workspaces[index].EntryID != entryID {
				continue
			}
			registry.Workspaces = append(registry.Workspaces[:index], registry.Workspaces[index+1:]...)
			return nil
		}
		return fmt.Errorf("%w: %s", ErrRegistryEntryNotFound, entryID)
	})
}

func generateRegistryEntryID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate workspace registry entry ID: %w", err)
	}
	return "wsr_" + hex.EncodeToString(random[:]), nil
}

func (s *RegistryService) uniqueEntryID(entries []workspacecore.RegistryEntry) (string, error) {
	existing := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		existing[entry.EntryID] = struct{}{}
	}
	for attempt := 0; attempt < 16; attempt++ {
		entryID, err := s.newEntryID()
		if err != nil {
			return "", err
		}
		entryID = strings.TrimSpace(entryID)
		if entryID == "" {
			continue
		}
		if _, duplicate := existing[entryID]; !duplicate {
			return entryID, nil
		}
	}
	return "", errors.New("workspace registry: entry ID generator did not produce a unique non-empty ID")
}

func updateObservedEntry(
	entry *workspacecore.RegistryEntry,
	root string,
	workspaceID string,
	name string,
	source string,
	now time.Time,
) {
	entry.WorkspaceID = workspaceID
	entry.Name = name
	entry.Root = root
	if entry.RegisteredAt.IsZero() {
		entry.RegisteredAt = now
	}
	entry.LastSeenAt = now
	entry.LastSeenBy = strings.TrimSpace(source)
}

type rootInspection struct {
	status   string
	id       string
	name     string
	manifest *workspacecore.Manifest
}

func inspectWorkspaceRoot(root string) rootInspection {
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return rootInspection{status: WorkspaceStatusMissing}
		}
		return rootInspection{status: WorkspaceStatusInvalid}
	}
	if !info.IsDir() {
		return rootInspection{status: WorkspaceStatusInvalid}
	}
	if _, err := os.Stat(workspacecore.ManifestPath(root)); err != nil {
		return rootInspection{status: WorkspaceStatusInvalid}
	}
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil || manifest == nil {
		return rootInspection{status: WorkspaceStatusInvalid}
	}
	inspection := rootInspection{
		status:   WorkspaceStatusIdentityMissing,
		manifest: manifest,
	}
	if manifest.Workspace != nil {
		inspection.id = strings.TrimSpace(manifest.Workspace.ID)
		inspection.name = strings.TrimSpace(manifest.Workspace.Name)
	}
	if inspection.id != "" {
		inspection.status = WorkspaceStatusReady
	}
	return inspection
}

func inspectRegistry(
	registry workspacecore.Registry,
	currentRoot string,
) (WorkspaceRegistryResponse, map[string]*workspacecore.Manifest) {
	response := WorkspaceRegistryResponse{
		Schema:     WorkspaceRegistrySchema,
		Workspaces: make([]RegisteredWorkspace, 0, len(registry.Workspaces)),
	}
	manifests := make(map[string]*workspacecore.Manifest, len(registry.Workspaces))
	identityIndexes := make(map[string][]int)

	for _, entry := range registry.Workspaces {
		inspection := inspectWorkspaceRoot(entry.Root)
		registered := RegisteredWorkspace{
			EntryID:      entry.EntryID,
			ID:           strings.TrimSpace(entry.WorkspaceID),
			Name:         strings.TrimSpace(entry.Name),
			Root:         entry.Root,
			Status:       inspection.status,
			LastSeenAt:   entry.LastSeenAt,
			ProjectCount: 0,
		}
		if registered.Name == "" {
			registered.Name = fallbackWorkspaceName(entry.Root)
		}
		if inspection.manifest != nil {
			registered.ID = inspection.id
			if inspection.name != "" {
				registered.Name = inspection.name
			}
			registered.ProjectCount = len(inspection.manifest.Projects)
			manifests[entry.EntryID] = inspection.manifest
			if inspection.id != "" {
				index := len(response.Workspaces)
				identityIndexes[inspection.id] = append(identityIndexes[inspection.id], index)
			}
			storedID := strings.TrimSpace(entry.WorkspaceID)
			if storedID != "" && inspection.id != "" && storedID != inspection.id {
				registered.Status = WorkspaceStatusIdentityConflict
			}
		}
		if currentRoot != "" && rootsReferToSamePath(entry.Root, currentRoot) {
			if response.CurrentEntryID == "" {
				response.CurrentEntryID = entry.EntryID
			}
		}
		response.Workspaces = append(response.Workspaces, registered)
	}

	for _, indexes := range identityIndexes {
		if len(indexes) < 2 {
			continue
		}
		for _, index := range indexes {
			response.Workspaces[index].Status = WorkspaceStatusIdentityConflict
		}
	}

	sort.SliceStable(response.Workspaces, func(i, j int) bool {
		left := response.Workspaces[i]
		right := response.Workspaces[j]
		if !left.LastSeenAt.Equal(right.LastSeenAt) {
			return left.LastSeenAt.After(right.LastSeenAt)
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.Root != right.Root {
			return left.Root < right.Root
		}
		return left.EntryID < right.EntryID
	})
	return response, manifests
}

func canonicalRegistryRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", errors.New("workspace registry: root is required")
	}
	abs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	if _, err := os.Lstat(abs); err == nil {
		resolved, resolveErr := filepath.EvalSymlinks(abs)
		if resolveErr != nil {
			return "", fmt.Errorf("resolve workspace root symlinks: %w", resolveErr)
		}
		return filepath.Clean(resolved), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect workspace root: %w", err)
	}
	return abs, nil
}

func rootsReferToSamePath(left string, right string) bool {
	leftCanonical, leftErr := canonicalRegistryRoot(left)
	rightCanonical, rightErr := canonicalRegistryRoot(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	if leftCanonical == rightCanonical {
		return true
	}
	leftInfo, leftStatErr := os.Stat(leftCanonical)
	rightInfo, rightStatErr := os.Stat(rightCanonical)
	return leftStatErr == nil && rightStatErr == nil && os.SameFile(leftInfo, rightInfo)
}

func registryRootMissing(root string) bool {
	_, err := os.Stat(root)
	return errors.Is(err, os.ErrNotExist)
}

func fallbackWorkspaceName(root string) string {
	name := filepath.Base(filepath.Clean(root))
	if name == "." || name == string(filepath.Separator) {
		return root
	}
	return name
}
