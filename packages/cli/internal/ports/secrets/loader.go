// Package secrets supplies the explicit process-injection contract used by commands and tasks.
package secrets

import (
	"context"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// Loader fetches variables for a workspace project from its configured source.
type Loader interface {
	ID() string
	Load(ctx context.Context, projectRoot, relativeDir, envName string) (map[string]string, error)
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
