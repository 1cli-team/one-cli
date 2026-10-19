package workspace

import (
	"context"
	"errors"
	"time"

	workspacecore "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
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

// RegistryService resolves logical identities exclusively from management storage.
type RegistryService struct{ store *store.Store }

func NewRegistryService(db *store.Store) *RegistryService { return &RegistryService{store: db} }
func (s *RegistryService) List(ctx context.Context, current string) (WorkspaceRegistryResponse, error) {
	entries, err := s.store.List(ctx)
	if err != nil {
		return WorkspaceRegistryResponse{}, err
	}
	out := WorkspaceRegistryResponse{Schema: WorkspaceRegistrySchema, Workspaces: []RegisteredWorkspace{}}
	for _, e := range entries {
		last, _ := time.Parse(time.RFC3339Nano, e.LastSeenAt)
		out.Workspaces = append(out.Workspaces, RegisteredWorkspace{EntryID: e.ID, ID: e.ID, Name: e.Name, Status: WorkspaceStatusReady, ProjectCount: e.ProjectCount, LastSeenAt: last})
		if e.ID == current {
			out.CurrentEntryID = e.ID
		}
	}
	return out, nil
}
func (s *RegistryService) Resolve(ctx context.Context, id string) (ResolvedWorkspace, error) {
	m, _, err := s.store.Load(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return ResolvedWorkspace{}, ErrRegistryEntryNotFound
	}
	if err != nil {
		return ResolvedWorkspace{}, err
	}
	return ResolvedWorkspace{RegisteredWorkspace: RegisteredWorkspace{EntryID: id, ID: id, Name: m.Workspace.Name, Root: id, Status: WorkspaceStatusReady, ProjectCount: len(m.Projects)}, Manifest: m}, nil
}
func (s *RegistryService) ResolveRead(ctx context.Context, id string) (ResolvedWorkspace, error) {
	return s.Resolve(ctx, id)
}
func (s *RegistryService) Forget(ctx context.Context, id string) error {
	err := s.store.Forget(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return ErrRegistryEntryNotFound
	}
	return err
}
