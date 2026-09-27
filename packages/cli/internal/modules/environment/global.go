package environment

import (
	"context"

	remote "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
)

// Global variables are independent of workspace resolution. Both transports
// use these operations, which verify the active session and saved location.
type GlobalLocation = remote.GlobalLocation
type GlobalListing = remote.GlobalListing
type RemoteProject = remote.RemoteProject

func Projects(ctx context.Context) ([]RemoteProject, error)          { return remote.Projects(ctx) }
func Project(ctx context.Context, id string) (*RemoteProject, error) { return remote.Project(ctx, id) }
func LoadGlobalLocation() (*GlobalLocation, error)                   { return remote.LoadGlobalLocation() }
func BindGlobal(ctx context.Context, id, env string) (*GlobalLocation, error) {
	return remote.BindGlobal(ctx, id, env)
}
func ListGlobal(ctx context.Context, env, path string) (*GlobalListing, error) {
	return remote.ListGlobal(ctx, env, path)
}
func GlobalSecret(ctx context.Context, action, env, path, key, value string) (any, error) {
	return remote.GlobalSecret(ctx, action, env, path, key, value)
}
func CreateGlobalFolder(ctx context.Context, env, path, name string) error {
	return remote.CreateGlobalFolder(ctx, env, path, name)
}
func ValidateGlobalPath(path string) (string, error) { return remote.ValidateGlobalPath(path) }
func GlobalValues(ctx context.Context, env, path string, keys []string) (map[string]string, error) {
	return remote.GlobalValues(ctx, env, path, keys)
}

func GlobalSummary(ctx context.Context) (*GlobalLocation, []remote.RemoteEnvironment, error) {
	return remote.GlobalSummary(ctx)
}
