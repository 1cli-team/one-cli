package workspace

import "time"

// RegistrySchemaVersion is the current on-disk workspaces.json schema.
const RegistrySchemaVersion = 1

// Registry stores logical workspaces and their checkout locations. Each
// checkout manifest remains the source of truth for projects and configuration.
type Registry struct {
	Version                 int             `json:"version"`
	Workspaces              []RegistryEntry `json:"workspaces"`
	GitRepositoriesImported bool            `json:"gitRepositoriesImported,omitempty"`
}

// RegistryEntry is one durable pointer to a workspace on this machine.
// EntryID is a machine-local opaque identifier, while WorkspaceID comes from
// one.manifest.toml and may be empty for legacy manifests.
type RegistryEntry struct {
	EntryID          string             `json:"entryId"`
	WorkspaceID      string             `json:"workspaceId,omitempty"`
	Name             string             `json:"name,omitempty"`
	Root             string             `json:"root"`
	RegisteredAt     time.Time          `json:"registeredAt"`
	LastSeenAt       time.Time          `json:"lastSeenAt"`
	LastSeenBy       string             `json:"lastSeenBy,omitempty"`
	GitCommonDir     string             `json:"gitCommonDir,omitempty"`
	GitWorkspacePath string             `json:"gitWorkspacePath,omitempty"`
	Worktrees        []RegistryWorktree `json:"worktrees,omitempty"`
	Aliases          []RegistryAlias    `json:"aliases,omitempty"`
}

type RegistryWorktree struct {
	ID   string `json:"id"`
	Root string `json:"root"`
}

type RegistryAlias struct {
	EntryID    string `json:"entryId"`
	WorktreeID string `json:"worktreeId,omitempty"`
}
