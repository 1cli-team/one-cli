// Package ci is the public contract for one-cli's CI providers.
//
// A Provider renders a CI workflow file (e.g. .github/workflows/ci-X.yml)
// for one project. Multiple provider implementations can register; callers
// select one explicitly or use DefaultProviderID. Provider selection is not
// persisted in one.manifest.json.
//
// Stability: Provider is a public type. New methods can be added with
// default-implementation helpers but existing methods are stable.
package ci

import (
	"fmt"
	"strings"
	"sync"

	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

// Input describes one subproject's CI workflow needs so out-of-tree providers
// can render against a stable shape.
type Input struct {
	// ProjectRoot is the absolute path to the workspace root.
	ProjectRoot string
	// TargetDir is the absolute path to the subproject directory.
	TargetDir string
	// RelativeDir is TargetDir relative to ProjectRoot in slash form.
	RelativeDir string
	// ProjectName is the user-facing name (used in workflow display).
	ProjectName string
	// Toolchain identifies which language adapter is in play.
	Toolchain toolchain.Toolchain
	// PackageManager is the chosen pm for Node subprojects; empty for Go.
	PackageManager toolchain.PackageManager
	// Scripts is the parsed package.json#scripts map; empty for Go.
	Scripts map[string]string
	// Adapter is the toolchain adapter for any language-specific
	// rendering the provider wants to delegate (default GitHub Actions
	// implementation calls Adapter.RenderWorkflow).
	Adapter toolchain.Adapter
	// WorkflowFilePath is the relative path the workflow file will
	// occupy under ProjectRoot, in slash form. Filled by the dispatcher
	// from Provider.WorkflowFilename.
	WorkflowFilePath string
}

// Provider is the CI provider contract.
type Provider interface {
	// ID returns a stable namespaced identifier, e.g.
	// "ci/github-actions".
	ID() string
	// WorkflowFilename returns the path (slash form, relative to
	// ProjectRoot) where this provider writes the workflow for the given input.
	// The application uses the same path for status and removal.
	WorkflowFilename(in Input) string
	// Render produces the workflow file contents.
	Render(in Input) string
}

// Registry is an immutable-by-convention provider set owned by one caller.
type Registry struct {
	providers []Provider
}

func NewRegistry(providers ...Provider) (*Registry, error) {
	seen := make(map[string]struct{}, len(providers))
	copyOfProviders := make([]Provider, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			return nil, fmt.Errorf("ci: nil provider")
		}
		id := strings.TrimSpace(provider.ID())
		if id == "" {
			return nil, fmt.Errorf("ci: provider with empty ID")
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("ci: provider %q already registered", id)
		}
		seen[id] = struct{}{}
		copyOfProviders = append(copyOfProviders, provider)
	}
	return &Registry{providers: copyOfProviders}, nil
}

func MustRegistry(providers ...Provider) *Registry {
	registry, err := NewRegistry(providers...)
	if err != nil {
		panic(err)
	}
	return registry
}

// Providers returns a defensive copy in construction order.
func (r *Registry) Providers() []Provider {
	if r == nil {
		return nil
	}
	out := make([]Provider, len(r.providers))
	copy(out, r.providers)
	return out
}

// Lookup returns the provider with the matching ID, or nil.
func (r *Registry) Lookup(id string) Provider {
	if r == nil {
		return nil
	}
	for _, p := range r.providers {
		if p.ID() == id {
			return p
		}
	}
	return nil
}

// The functions below preserve the original public extension API. One CLI's
// internal composition does not use this process-global compatibility set;
// new in-process applications should prefer NewRegistry.
var compatibilityProviders struct {
	sync.RWMutex
	providers []Provider
}

// Register adds a provider to the legacy public compatibility set.
func Register(provider Provider) {
	if provider == nil {
		return
	}
	compatibilityProviders.Lock()
	compatibilityProviders.providers = append(compatibilityProviders.providers, provider)
	compatibilityProviders.Unlock()
}

// Providers returns the legacy public compatibility providers.
func Providers() []Provider {
	compatibilityProviders.RLock()
	defer compatibilityProviders.RUnlock()
	out := make([]Provider, len(compatibilityProviders.providers))
	copy(out, compatibilityProviders.providers)
	return out
}

// Lookup finds a provider in the legacy public compatibility set.
func Lookup(id string) Provider {
	compatibilityProviders.RLock()
	defer compatibilityProviders.RUnlock()
	for _, provider := range compatibilityProviders.providers {
		if provider.ID() == id {
			return provider
		}
	}
	return nil
}

// DefaultProviderID is used when a caller does not select a provider.
// GitHub Actions is the only bundled provider.
const DefaultProviderID = "ci/github-actions"
