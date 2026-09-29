// Package secrets supplies the explicit process-injection contract used by commands and tasks.
package secrets

import (
	"context"
	"maps"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// Loader fetches variables for a workspace project from its configured source.
type Loader interface {
	ID() string
	Load(ctx context.Context, projectRoot, relativeDir, envName string) (map[string]string, error)
}

// BatchLoader optionally fetches several projects in one invocation. Results are
// keyed by the requested relative directory. Implementations must return every
// requested project (including empty environments), isolate their maps, and
// never reuse secret snapshots across invocations.
type BatchLoader interface {
	Loader
	LoadProjects(ctx context.Context, projectRoot string, relativeDirs []string, envName string) (map[string]map[string]string, error)
}

// LoadProjects prefers a provider's batch path, with a serial fallback for
// loaders that do not promise concurrency safety.
func LoadProjects(ctx context.Context, loader Loader, root string, dirs []string, env string) (map[string]map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if batch, ok := loader.(BatchLoader); ok {
		return batch.LoadProjects(ctx, root, dirs, env)
	}
	result := make(map[string]map[string]string, len(dirs))
	for _, dir := range dirs {
		if _, ok := result[dir]; ok {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		values, err := loader.Load(ctx, root, dir, env)
		if err != nil {
			return nil, err
		}
		result[dir] = maps.Clone(values)
	}
	return result, nil
}

// Registry owns one command tree's loader set. It is immutable after
// construction and therefore safe to share between commands in that tree.
type Registry struct {
	loaders []Loader
}

func NewRegistry(loaders ...Loader) (*Registry, error) {
	seen := make(map[string]struct{}, len(loaders))
	copyOfLoaders := make([]Loader, 0, len(loaders))
	for _, loader := range loaders {
		if loader == nil {
			return nil, i18n.Errorf("secrets.loader_missing")
		}
		id := strings.TrimSpace(loader.ID())
		if id == "" {
			return nil, i18n.Errorf("secrets.loader_id_empty")
		}
		if _, exists := seen[id]; exists {
			return nil, i18n.Errorf("secrets.loader_duplicate", id)
		}
		seen[id] = struct{}{}
		copyOfLoaders = append(copyOfLoaders, loader)
	}

	return &Registry{loaders: copyOfLoaders}, nil
}

func MustRegistry(loaders ...Loader) *Registry {
	registry, err := NewRegistry(loaders...)
	if err != nil {
		panic(err)
	}
	return registry
}

// Find returns the loader with the given ID, or nil if it is not part of this
// registry. Missing loaders never trigger a fallback.
func (r *Registry) Find(id string) Loader {
	if r == nil {
		return nil
	}
	for _, l := range r.loaders {
		if l.ID() == id {
			return l
		}
	}
	return nil
}
