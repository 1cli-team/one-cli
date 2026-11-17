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

	workspacecore "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	registryport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/workspaceregistry"
)

const WorkspaceRegistrySchema = "one-cli/workspaces/v1"

const (
	WorkspaceStatusReady            = "ready"
	WorkspaceStatusUninitialized    = "uninitialized"
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
	EntryID      string                        `json:"entryId"`
	ID           string                        `json:"id,omitempty"`
	Name         string                        `json:"name"`
	Root         string                        `json:"root"`
	Status       string                        `json:"status"`
	ProjectCount int                           `json:"projectCount"`
	LastSeenAt   time.Time                     `json:"lastSeenAt"`
	GitCommonDir string                        `json:"gitCommonDir,omitempty"`
	WorktreeID   string                        `json:"worktreeId,omitempty"`
	Worktrees    []RegisteredWorktree          `json:"worktrees,omitempty"`
	Aliases      []workspacecore.RegistryAlias `json:"aliases,omitempty"`
}

type RegisteredWorktree struct {
	ID           string `json:"id"`
	Root         string `json:"root"`
	Branch       string `json:"branch"`
	Main         bool   `json:"main"`
	Status       string `json:"status"`
	ProjectCount int    `json:"projectCount"`
}

// ResolvedWorkspace is the trusted server-side result of resolving an opaque
// entry ID. HTTP callers use Root for existing Workspace use cases and may
// reuse Manifest to avoid accepting or rediscovering a client-supplied path.
type ResolvedWorkspace struct {
	RegisteredWorkspace
	Manifest *workspacecore.Manifest `json:"-"`
}

// RegistryService owns the machine-local Workspace observation, listing and
// trusted-resolution use cases.
type RegistryService struct {
	repository       registryport.Repository
	worktreeIdentity func(string) (string, error)
	git              registryport.GitInspector
	legacyGit        func() ([]registryport.LegacyRepository, error)
}

// RegistryOptions supplies optional repository identity discovery.
type RegistryOptions struct {
	WorktreeIdentity func(string) (string, error)
	Git              registryport.GitInspector
	LegacyGit        func() ([]registryport.LegacyRepository, error)
}

// NewRegistryService constructs the Workspace registry use-case boundary.
func NewRegistryService(
	repository registryport.Repository,
	options ...RegistryOptions,
) (*RegistryService, error) {
	if repository == nil {
		return nil, errors.New("workspace registry: repository is required")
	}
	service := &RegistryService{repository: repository}
	if len(options) > 0 {
		service.worktreeIdentity = options[0].WorktreeIdentity
		service.git = options[0].Git
		service.legacyGit = options[0].LegacyGit
	}
	return service, nil
}

// Observe validates root as a One Workspace and atomically records that it
// was seen by source (for example "create" or "desktop"). Existing paths are
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
	if _, err := s.load(ctx); err != nil {
		return RegisteredWorkspace{}, err
	}
	inspection := inspectWorkspaceRoot(canonicalRoot)
	gitRoot := false
	if s.git != nil {
		_, err := s.git(ctx, canonicalRoot)
		gitRoot = err == nil
	}
	if inspection.status == WorkspaceStatusMissing || (inspection.status == WorkspaceStatusInvalid && !gitRoot) || (inspection.status == WorkspaceStatusUninitialized && !gitRoot) {
		return RegisteredWorkspace{}, fmt.Errorf(
			"%w: workspace %q has status %s",
			ErrRegistryEntryUnavailable,
			canonicalRoot,
			inspection.status,
		)
	}

	now := time.Now().UTC()
	workspaceID := inspection.id
	name := inspection.name
	if name == "" {
		name = fallbackWorkspaceName(canonicalRoot)
	}
	var observedEntryID string
	err = s.repository.Update(ctx, func(registry *workspacecore.Registry) error {
		registry.Version = workspacecore.RegistrySchemaVersion

		for index := range registry.Workspaces {
			for _, tree := range registry.Workspaces[index].Worktrees {
				if rootsReferToSamePath(tree.Root, canonicalRoot) {
					registry.Workspaces[index].LastSeenAt = now
					if registry.Workspaces[index].WorkspaceID == "" && workspaceID != "" {
						registry.Workspaces[index].WorkspaceID = workspaceID
					}
					registry.Workspaces[index].LastSeenBy = source
					observedEntryID = registry.Workspaces[index].EntryID
					return nil
				}
			}
		}
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

	registry, err := s.load(ctx)
	if err != nil {
		return RegisteredWorkspace{}, err
	}
	entry, checkout, ok := findRegistryEntry(registry, observedEntryID)
	if !ok {
		return RegisteredWorkspace{}, ErrRegistryEntryNotFound
	}
	longest := 0
	for _, tree := range entry.Worktrees {
		relative, err := filepath.Rel(tree.Root, canonicalRoot)
		if err == nil && filepath.IsLocal(relative) && len(tree.Root) > longest {
			checkout = tree.ID
			longest = len(tree.Root)
		}
	}
	return s.location(ctx, entry.EntryID+"~"+checkout)
}

// List returns every registered Workspace with a live status computed from
// disk. Entries whose roots are confirmed missing on an available volume are
// pruned from the machine-local registry. Pruning only updates workspaces.json;
// it never deletes or mutates anything under an entry Root.
func (s *RegistryService) List(
	ctx context.Context,
	currentRoot string,
) (WorkspaceRegistryResponse, error) {
	registry, err := s.load(ctx)
	if err != nil {
		return WorkspaceRegistryResponse{}, err
	}
	if registryHasDefinitelyMissingRoot(registry) {
		var updated workspacecore.Registry
		err = s.repository.Update(ctx, func(latest *workspacecore.Registry) error {
			pruneDefinitelyMissingRoots(latest)
			updated = cloneWorkspaceRegistry(*latest)
			return nil
		})
		if err != nil {
			return WorkspaceRegistryResponse{}, err
		}
		registry = updated
	}
	canonicalCurrent := ""
	if strings.TrimSpace(currentRoot) != "" {
		canonicalCurrent, err = canonicalRegistryRoot(currentRoot)
		if err != nil {
			return WorkspaceRegistryResponse{}, err
		}
	}
	response, _ := s.inspectRegistry(ctx, registry, canonicalCurrent)
	return response, nil
}

func registryHasDefinitelyMissingRoot(registry workspacecore.Registry) bool {
	for _, entry := range registry.Workspaces {
		if entry.GitCommonDir == "" && registryRootDefinitelyMissing(entry.Root) {
			return true
		}
	}
	return false
}

func pruneDefinitelyMissingRoots(registry *workspacecore.Registry) bool {
	retained := make([]workspacecore.RegistryEntry, 0, len(registry.Workspaces))
	for _, entry := range registry.Workspaces {
		if entry.GitCommonDir == "" && registryRootDefinitelyMissing(entry.Root) {
			continue
		}
		retained = append(retained, entry)
	}
	if len(retained) == len(registry.Workspaces) {
		return false
	}
	registry.Workspaces = retained
	return true
}

func cloneWorkspaceRegistry(registry workspacecore.Registry) workspacecore.Registry {
	cloned := registry
	cloned.Workspaces = append([]workspacecore.RegistryEntry(nil), registry.Workspaces...)
	for index := range cloned.Workspaces {
		cloned.Workspaces[index].Worktrees = append([]workspacecore.RegistryWorktree(nil), registry.Workspaces[index].Worktrees...)
		cloned.Workspaces[index].Aliases = append([]workspacecore.RegistryAlias(nil), registry.Workspaces[index].Aliases...)
	}
	if cloned.Workspaces == nil {
		cloned.Workspaces = []workspacecore.RegistryEntry{}
	}
	return cloned
}

func registryRootDefinitelyMissing(root string) bool {
	return registryRootDefinitelyMissingWithStat(root, func(path string) error {
		_, err := os.Stat(path)
		return err
	})
}

func registryRootDefinitelyMissingWithStat(root string, stat func(string) error) bool {
	root = strings.TrimSpace(root)
	if root == "" || !filepath.IsAbs(root) || stat == nil {
		return false
	}
	if err := stat(root); !errors.Is(err, os.ErrNotExist) {
		return false
	}

	volumeRoot := string(filepath.Separator)
	if volume := filepath.VolumeName(root); volume != "" {
		volumeRoot = volume + string(filepath.Separator)
	}
	return stat(volumeRoot) == nil
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

	location, err := s.location(ctx, entryID)
	if err != nil {
		return ResolvedWorkspace{}, err
	}
	if location.Status == WorkspaceStatusIdentityConflict && !allowIdentityConflict {
		return ResolvedWorkspace{}, fmt.Errorf("%w: workspace %q at %s", ErrRegistryEntryIdentityConflict, entryID, location.Root)
	}
	if location.Status != WorkspaceStatusReady && !(allowIdentityConflict && location.Status == WorkspaceStatusIdentityConflict) {
		return ResolvedWorkspace{}, fmt.Errorf("%w: workspace %q at %s has status %s", ErrRegistryEntryUnavailable, entryID, location.Root, location.Status)
	}
	manifest, err := workspacecore.ReadManifest(location.Root)
	if err != nil {
		return ResolvedWorkspace{}, err
	}
	return ResolvedWorkspace{RegisteredWorkspace: location, Manifest: manifest}, nil
}

// Forget removes only the local registration. It never deletes or mutates the
// Workspace directory.
func (s *RegistryService) Forget(ctx context.Context, entryID string) error {
	registry, err := s.load(ctx)
	if err != nil {
		return err
	}
	entry, _, ok := findRegistryEntry(registry, entryID)
	if !ok {
		return ErrRegistryEntryNotFound
	}
	entryID = entry.EntryID
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
		entryID, err := generateRegistryEntryID()
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
		if errors.Is(err, os.ErrNotExist) {
			return rootInspection{status: WorkspaceStatusUninitialized}
		}
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

func (s *RegistryService) inspectRegistry(
	ctx context.Context,
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
			GitCommonDir: entry.GitCommonDir,
			Aliases:      entry.Aliases,
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
		branches := make(map[string]registryport.GitWorktree)
		if s.git != nil && entry.GitCommonDir != "" {
			info, err := s.git(ctx, entry.GitCommonDir)
			if err == nil {
				for _, tree := range info.Worktrees {
					branches[tree.ID] = tree
				}
			}
		}
		for index, tree := range entry.Worktrees {
			state := inspectWorkspaceRoot(tree.Root)
			if state.id != "" && entry.WorkspaceID != "" && state.id != entry.WorkspaceID {
				state.status = WorkspaceStatusIdentityConflict
			}
			gitTree := branches[tree.ID]
			count := 0
			if state.manifest != nil {
				count = len(state.manifest.Projects)
			}
			registered.Worktrees = append(registered.Worktrees, RegisteredWorktree{ID: tree.ID, Root: tree.Root, Branch: gitTree.Branch, Main: index == 0, Status: state.status, ProjectCount: count})
		}
		if len(registered.Worktrees) > 0 {
			selected := registered.Worktrees[0]
			if selected.Status == WorkspaceStatusMissing {
				for _, tree := range registered.Worktrees {
					if tree.Status == WorkspaceStatusReady {
						selected = tree
						break
					}
				}
			}
			registered.WorktreeID = selected.ID
			registered.Root = selected.Root
			registered.Status = selected.Status
			registered.ProjectCount = selected.ProjectCount
			if selected.Root != entry.Root {
				manifests[entry.EntryID] = inspectWorkspaceRoot(selected.Root).manifest
			}
		}
		if currentRoot != "" {
			for _, tree := range entry.Worktrees {
				if rootsReferToSamePath(tree.Root, currentRoot) {
					response.CurrentEntryID = entry.EntryID
				}
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
		if s.sharedWorktreeIdentity(response.Workspaces, indexes) {
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

// Only Git-verified worktrees at the same manifest location can share identity.
func (s *RegistryService) sharedWorktreeIdentity(entries []RegisteredWorkspace, indexes []int) bool {
	if s.worktreeIdentity == nil {
		return false
	}
	identity := ""
	for _, index := range indexes {
		entry := entries[index]
		if entry.Status != WorkspaceStatusReady {
			return false
		}
		current, err := s.worktreeIdentity(entry.Root)
		if err != nil || current == "" || (identity != "" && current != identity) {
			return false
		}
		identity = current
	}
	return true
}

func (s *RegistryService) location(ctx context.Context, requested string) (RegisteredWorkspace, error) {
	registry, err := s.load(ctx)
	if err != nil {
		return RegisteredWorkspace{}, err
	}
	entry, checkout, ok := findRegistryEntry(registry, requested)
	if !ok {
		return RegisteredWorkspace{}, ErrRegistryEntryNotFound
	}
	response, _ := s.inspectRegistry(ctx, registry, "")
	for _, item := range response.Workspaces {
		if item.EntryID != entry.EntryID {
			continue
		}
		if checkout == "" {
			return item, nil
		}
		for _, tree := range item.Worktrees {
			if tree.ID != checkout {
				continue
			}
			item.Root, item.ProjectCount, item.WorktreeID = tree.Root, tree.ProjectCount, tree.ID
			if item.Status != WorkspaceStatusIdentityConflict {
				item.Status = tree.Status
			}
			return item, nil
		}
		return RegisteredWorkspace{}, ErrRegistryEntryUnavailable
	}
	return RegisteredWorkspace{}, ErrRegistryEntryNotFound
}

func (s *RegistryService) Initialize(ctx context.Context, id, name string) (RegisteredWorkspace, error) {
	location, err := s.location(ctx, id)
	if err != nil {
		return RegisteredWorkspace{}, err
	}
	if location.Status != WorkspaceStatusUninitialized {
		return location, nil
	}
	if s.git != nil {
		info, err := s.git(ctx, location.Root)
		if err == nil {
			for _, tree := range info.Worktrees {
				if tree.Bare && rootsReferToSamePath(tree.Root, location.Root) {
					return RegisteredWorkspace{}, ErrRegistryEntryUnavailable
				}
			}
		}
	}
	workspaceID := location.ID
	if workspaceID == "" {
		workspaceID, err = generateRegistryEntryID()
		if err != nil {
			return RegisteredWorkspace{}, err
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = location.Name
	}
	manifest := &workspacecore.Manifest{Version: workspacecore.ManifestVersion, Workspace: &workspacecore.ManifestWorkspace{ID: workspaceID, Name: name}, Projects: []workspacecore.ManifestProject{}}
	data, err := workspacecore.MarshalManifest(manifest)
	if err != nil {
		return RegisteredWorkspace{}, err
	}
	path := workspacecore.ManifestPath(location.Root)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if errors.Is(err, os.ErrExist) {
		return s.Observe(ctx, location.Root, "initialize")
	}
	if err != nil {
		return RegisteredWorkspace{}, err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		os.Remove(path)
		return RegisteredWorkspace{}, err
	}
	if err := file.Close(); err != nil {
		return RegisteredWorkspace{}, err
	}
	return s.Observe(ctx, location.Root, "initialize")
}
