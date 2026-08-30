package workspace

import "time"

// RegistrySchemaVersion is the current on-disk workspaces.json schema.
const RegistrySchemaVersion = 1

// Registry is the machine-local index of One workspaces observed by the CLI.
// It deliberately stores only identity and location metadata; the manifest at
// each Root remains the source of truth for projects and configuration.
type Registry struct {
	Version    int             `json:"version"`
	Workspaces []RegistryEntry `json:"workspaces"`
}

// RegistryEntry is one durable pointer to a workspace on this machine.
// EntryID is a machine-local opaque identifier, while WorkspaceID comes from
// one.manifest.json and may be empty for legacy manifests.
type RegistryEntry struct {
	EntryID      string    `json:"entryId"`
	WorkspaceID  string    `json:"workspaceId,omitempty"`
	Name         string    `json:"name,omitempty"`
	Root         string    `json:"root"`
	RegisteredAt time.Time `json:"registeredAt"`
	LastSeenAt   time.Time `json:"lastSeenAt"`
	LastSeenBy   string    `json:"lastSeenBy,omitempty"`
}
