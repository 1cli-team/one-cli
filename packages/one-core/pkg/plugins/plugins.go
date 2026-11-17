// Package plugins composes built-in integrations without global registration.
package plugins

import (
	"fmt"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/backend"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/environment"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/secrets"
)

// Plugin describes capabilities contributed by one compiled integration.
// Constructors and Contributions must not open stores or contact remote APIs.
type Plugin interface {
	ID() string
	Contributions() Contributions
}

type Contributions struct {
	Git          gitmodule.Backend
	Backends     []backend.BackendSpec
	Environments []environment.Backend
	Loaders      []secrets.Loader
	Global       secrets.GlobalLoader
}

type Registry struct {
	Git          gitmodule.Backend
	Catalog      *backend.Catalog
	Environments *environment.Service
	Loaders      *secrets.Registry
	Global       secrets.GlobalLoader
}

func New(installed ...Plugin) (*Registry, error) {
	seen := make(map[string]bool, len(installed))
	var specs []backend.BackendSpec
	var environments []environment.Backend
	var loaders []secrets.Loader
	var global secrets.GlobalLoader
	var gitBackend gitmodule.Backend
	for _, plugin := range installed {
		if plugin == nil || plugin.ID() == "" {
			return nil, fmt.Errorf("plugins: missing identity")
		}
		if seen[plugin.ID()] {
			return nil, fmt.Errorf("plugins: duplicate plugin %q", plugin.ID())
		}
		seen[plugin.ID()] = true
		contributions := plugin.Contributions()
		if contributions.Git != nil {
			if gitBackend != nil {
				return nil, fmt.Errorf("plugins: Git backend registered twice")
			}
			gitBackend = contributions.Git
		}
		specs = append(specs, contributions.Backends...)
		environments = append(environments, contributions.Environments...)
		loaders = append(loaders, contributions.Loaders...)
		if contributions.Global != nil {
			if global != nil {
				return nil, fmt.Errorf("plugins: shared credentials registered twice")
			}
			global = contributions.Global
		}
	}
	catalog, err := backend.New(specs...)
	if err != nil {
		return nil, err
	}
	environmentService, err := environment.NewService(catalog, environments...)
	if err != nil {
		return nil, err
	}
	registry, err := secrets.NewRegistry(loaders...)
	if err != nil {
		return nil, err
	}
	return &Registry{Git: gitBackend, Catalog: catalog, Environments: environmentService, Loaders: registry, Global: global}, nil
}
