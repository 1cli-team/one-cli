// Package ci is the public contract for one-cli's CI providers.
//
// A Provider renders a CI workflow file (e.g. .github/workflows/ci-X.yml)
// for one subproject. It is the user-selectable axis under the
// CI domain. Multiple provider
// implementations can register; the user picks one in their manifest
// selection (manifest.ci section).
//
// Stability: Provider is a public type. New methods can be added with
// default-implementation helpers but existing methods are stable.
package ci

import "github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"

// Input describes one subproject's CI workflow needs. Mirrors the
// previous internal/ci.SyncOptions but exposed publicly so out-of-tree
// providers can implement Render against a stable shape.
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
	// ProjectRoot) where this provider would write the workflow for
	// the given input. Used by the dispatcher to short-circuit a
	// rewrite when the file is up to date.
	WorkflowFilename(in Input) string
	// Render produces the workflow file contents.
	Render(in Input) string
}

// providers holds registered CI providers.
var providers []Provider

// Register adds a provider to the active set. Called from each
// provider package's init().
func Register(p Provider) {
	if p == nil {
		return
	}
	providers = append(providers, p)
}

// Providers returns the registered providers in registration order.
func Providers() []Provider {
	return providers
}

// Lookup returns the provider with the matching ID, or nil. Used by
// the dispatcher in internal/ci to honor manifest.ci.
func Lookup(id string) Provider {
	for _, p := range providers {
		if p.ID() == id {
			return p
		}
	}
	return nil
}

// DefaultProviderID is the provider used when manifest.ci is empty.
// GitHub Actions is the only bundled provider; out-of-tree providers
// can register via Register and be selected via manifest.ci.
const DefaultProviderID = "ci/github-actions"
