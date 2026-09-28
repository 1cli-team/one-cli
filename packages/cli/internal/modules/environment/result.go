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
