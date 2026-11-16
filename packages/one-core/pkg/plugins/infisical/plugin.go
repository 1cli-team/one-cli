// Package infisical is the built-in Infisical integration. It owns remote
// secrets, browser authentication, shared credentials and workspace binding.
package infisical

import (
	"context"
	"slices"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/backend"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins"
	remote "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/infisical/internal/remote"
	session "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/infisical/session"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/secrets"
)

type Plugin struct{}

func New() *Plugin         { return &Plugin{} }
func (*Plugin) ID() string { return backend.EnvInfisical }

func (p *Plugin) Contributions() plugins.Contributions {
	return plugins.Contributions{
		Backends: []backend.BackendSpec{{
			ID: backend.BackendID{Domain: backend.DomainEnv, Name: p.ID()},
			Capabilities: []backend.Capability{backend.CapabilityEnvGet, backend.CapabilityEnvSet,
				backend.CapabilityEnvDelete, backend.CapabilityEnvList, backend.CapabilityEnvInject, backend.CapabilityScaffold},
		}},
		Environments: []environment.Backend{p},
		Loaders:      []secrets.Loader{remote.Loader()},
		Global:       p,
	}
}

func (*Plugin) Require(root string) error {
	_, err := remote.RequireWorkspaceConfig(root)
	return err
}

func credentials() (*remote.WorkspaceConfig, *remote.Credentials, error) {
	current, err := session.Require()
	if err != nil {
		return nil, nil, err
	}
	return &remote.WorkspaceConfig{SiteURL: current.SiteURL}, &remote.Credentials{AccessToken: current.Token}, nil
}

func (*Plugin) Get(ctx context.Context, root, env, path, key string) (*environment.GetResult, error) {
	cfg, creds, err := credentials()
	if err != nil {
		return nil, err
	}
	result, err := remote.Get(ctx, root, remote.GetInput{Env: env, Path: path, Key: key, Cfg: cfg, Creds: creds})
	if result == nil || err != nil {
		return nil, err
	}
	return &environment.GetResult{Schema: result.Schema, Environment: result.Env, Path: result.Path, Key: result.Key, Value: result.Value}, nil
}

func (*Plugin) List(ctx context.Context, root, env, path string) (*environment.ListResult, error) {
	cfg, creds, err := credentials()
	if err != nil {
		return nil, err
	}
	result, err := remote.List(ctx, root, remote.ListInput{Env: env, Path: path, Cfg: cfg, Creds: creds})
	if result == nil || err != nil {
		return nil, err
	}
	total := result.Total
	return &environment.ListResult{Schema: result.Schema, Environment: result.Env, Path: result.Path, Keys: result.Keys, Total: &total}, nil
}

func (*Plugin) Set(ctx context.Context, root, env, path, key, value string, overwrite bool) (*environment.SetResult, error) {
	cfg, creds, err := credentials()
	if err != nil {
		return nil, err
	}
	result, err := remote.Set(ctx, root, remote.SetInput{Env: env, Path: path, Key: key, Value: value, Overwrite: overwrite, Cfg: cfg, Creds: creds})
	if result == nil || err != nil {
		return nil, err
	}
	return &environment.SetResult{Schema: result.Schema, Environment: result.Env, Path: result.Path, Key: result.Key, Action: result.Action}, nil
}

func (*Plugin) Delete(ctx context.Context, root, env, path, key string) (*environment.DeleteResult, error) {
	cfg, creds, err := credentials()
	if err != nil {
		return nil, err
	}
	result, err := remote.Delete(ctx, root, remote.DeleteInput{Env: env, Path: path, Key: key, Cfg: cfg, Creds: creds})
	if result == nil || err != nil {
		return nil, err
	}
	return &environment.DeleteResult{Schema: result.Schema, Env: result.Env, Path: result.Path, Key: result.Key, Status: result.Status}, nil
}

func contains(values []string, target string) bool { return slices.Contains(values, target) }

var _ plugins.Plugin = (*Plugin)(nil)
var _ environment.Backend = (*Plugin)(nil)

func (*Plugin) ValidatePath(path string) (string, error) { return remote.ValidateGlobalPath(path) }
func (*Plugin) Values(ctx context.Context, env, path string, keys []string) (map[string]string, error) {
	return remote.GlobalValues(ctx, env, path, keys)
}

var _ secrets.GlobalLoader = (*Plugin)(nil)
