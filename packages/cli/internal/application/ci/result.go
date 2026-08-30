package ci

type StatusResult struct {
	Schema      string          `json:"schema"`
	Configured  bool            `json:"configured"`
	Provider    string          `json:"provider,omitempty"`
	Projects    []ProjectStatus `json:"projects"`
	NextCommand string          `json:"next_command"`
}

type ProjectStatus struct {
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	Provider     string `json:"provider,omitempty"`
	WorkflowPath string `json:"workflow_path"`
}

type ActionResult struct {
	Schema      string          `json:"schema"`
	Action      string          `json:"action"`
	Provider    string          `json:"provider"`
	Projects    []ActionProject `json:"projects"`
	NextCommand string          `json:"next_command,omitempty"`
}

type ActionProject struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	Enabled      bool   `json:"enabled"`
	Provider     string `json:"provider,omitempty"`
	WorkflowPath string `json:"workflow_path"`
	Changed      bool   `json:"changed"`
}
