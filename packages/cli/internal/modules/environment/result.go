package environment

type Summary struct {
	Schema                string   `json:"schema"`
	Source                string   `json:"source"`
	DefaultEnvironment    string   `json:"default_environment"`
	AvailableEnvironments []string `json:"available_environments"`
	Scope                 string   `json:"scope"`
	Project               string   `json:"project,omitempty"`
	Commands              []string `json:"commands"`
}

type GetResult struct {
	Schema      string `json:"schema"`
	Source      string `json:"source,omitempty"`
	Environment string `json:"env,omitempty"`
	Path        string `json:"path,omitempty"`
	Key         string `json:"key"`
	Value       string `json:"value"`
}

type ListResult struct {
	Schema      string   `json:"schema"`
	Sources     []string `json:"sources,omitempty"`
	Environment string   `json:"env,omitempty"`
	Path        string   `json:"path,omitempty"`
	Keys        []string `json:"keys"`
	Total       *int     `json:"total,omitempty"`
}

type SetResult struct {
	Schema             string `json:"schema"`
	Source             string `json:"source,omitempty"`
	Environment        string `json:"env,omitempty"`
	Path               string `json:"path,omitempty"`
	Key                string `json:"key"`
	Action             string `json:"action"`
	CreatedEnvironment bool   `json:"created_environment,omitempty"`
}

type PullEntry struct {
	Name          string   `json:"name"`
	RelativeDir   string   `json:"relative_dir"`
	InfisicalPath string   `json:"infisical_path"`
	EnvFilePath   string   `json:"env_file_path"`
	Status        string   `json:"status"`
	Reason        string   `json:"reason,omitempty"`
	KeysWritten   []string `json:"keys_written,omitempty"`
}

type PullResult struct {
	Schema        string      `json:"schema"`
	Environment   string      `json:"env"`
	DryRun        bool        `json:"dry_run"`
	WrittenCount  int         `json:"written_count"`
	SkippedCount  int         `json:"skipped_count"`
	PerSubproject []PullEntry `json:"per_subproject"`
}

type SwitchResult struct {
	Schema       string `json:"schema"`
	From         string `json:"from"`
	To           string `json:"to"`
	ManifestPath string `json:"manifest_path"`
	Synced       int    `json:"synced,omitempty"`
	Conflicts    int    `json:"conflicts,omitempty"`
	SkippedSync  bool   `json:"skipped_sync,omitempty"`
}
