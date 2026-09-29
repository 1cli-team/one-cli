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

type BindingResult struct {
	ProjectID     string `json:"project_id"`
	ProjectName   string `json:"project_name"`
	Created       bool   `json:"created"`
	RequestedName string `json:"requested_name,omitempty"`
}

type SetResult struct {
	Binding            *BindingResult `json:"binding,omitempty"`
	Schema             string         `json:"schema"`
	Source             string         `json:"source,omitempty"`
	Environment        string         `json:"env,omitempty"`
	Path               string         `json:"path,omitempty"`
	Key                string         `json:"key"`
	Action             string         `json:"action"`
	CreatedEnvironment bool           `json:"created_environment,omitempty"`
}
