package environment

import "context"

// Backend supplies remote operations. Workspace selection, validation and
// publication policy remain in Service; implementations own authentication.
type Backend interface {
	ID() string
	Require(root string) error
	BindWorkspace(context.Context, BindWorkspaceInput) (*WorkspaceBindingResult, error)
	Get(ctx context.Context, root, env, path, key string) (*GetResult, error)
	List(ctx context.Context, root, env, path string) (*ListResult, error)
	Set(ctx context.Context, root, env, path, key, value string, overwrite bool) (*SetResult, error)
	Delete(ctx context.Context, root, env, path, key string) (*DeleteResult, error)
}

type DeleteResult struct {
	Schema string `json:"schema"`
	Env    string `json:"env"`
	Path   string `json:"path"`
	Key    string `json:"key"`
	Status string `json:"status"`
}
