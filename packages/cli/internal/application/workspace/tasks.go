package workspace

import "context"

// TaskReader projects the effective mise catalog without executing tasks.
type TaskReader func(context.Context, string, string) ([]ProjectTask, error)
type ProjectTask struct {
	Name         string   `json:"name"`
	Operation    string   `json:"operation"`
	Source       string   `json:"source"`
	Dependencies []string `json:"depends"`
	Outputs      []string `json:"outputs"`
	CacheEnabled bool     `json:"cacheEnabled"`
}
type ProjectTasks struct {
	Status  string        `json:"status"`
	Entries []ProjectTask `json:"entries"`
}

func (s *Service) projectTasks(ctx context.Context, root, project string) *ProjectTasks {
	if s.tasks == nil {
		return nil
	}
	entries, err := s.tasks(ctx, root, project)
	if err != nil {
		return &ProjectTasks{Status: "unavailable", Entries: []ProjectTask{}}
	}
	return &ProjectTasks{Status: "ready", Entries: entries}
}
