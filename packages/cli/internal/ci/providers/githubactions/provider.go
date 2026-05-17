// Package githubactions implements pkg/ci.Provider for GitHub Actions.
// It wraps the existing toolchain.Adapter.RenderWorkflow logic so the
// migration to backend form is purely additive — same output, same path
// scheme, registered as a provider.
package githubactions

import (
	"path/filepath"
	"regexp"
	"strings"

	pkgci "github.com/torchstellar-team/one-cli/packages/cli/pkg/ci"
)

const (
	id             = "ci/github-actions"
	workflowPrefix = "ci-"
)

// Provider is the bundled GitHub Actions CI provider.
type Provider struct{}

// ID returns the namespaced provider id.
func (Provider) ID() string { return id }

// WorkflowFilename returns the canonical .github/workflows/ci-<id>.yml
// path (slash form, relative to ProjectRoot) for the given input.
func (Provider) WorkflowFilename(in pkgci.Input) string {
	rel := in.RelativeDir
	if rel == "" {
		// Fall back to deriving from absolute paths when RelativeDir
		// wasn't pre-filled. Mirrors the old behavior in internal/ci.
		if r, err := filepath.Rel(in.ProjectRoot, in.TargetDir); err == nil {
			rel = filepath.ToSlash(r)
		} else {
			rel = in.TargetDir
		}
	}
	return ".github/workflows/" + workflowPrefix + workflowID(rel) + ".yml"
}

// Render produces the workflow content via the toolchain adapter — the
// existing rendering logic from internal/ci, unchanged. Future Helm/ko/
// release-flow integration extends WorkflowInput on the toolchain side
// rather than here.
func (Provider) Render(in pkgci.Input) string {
	if in.Adapter == nil {
		return ""
	}
	return in.Adapter.RenderWorkflow(workflowInputFor(in))
}

// init registers the provider so a blank import wires it into pkg/ci.
func init() {
	pkgci.Register(Provider{})
}

var (
	pathSepRE  = regexp.MustCompile(`[\\/]+`)
	nonAllowed = regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	manyDashes = regexp.MustCompile(`-+`)
)

func workflowID(rel string) string {
	id := pathSepRE.ReplaceAllString(rel, "-")
	id = nonAllowed.ReplaceAllString(id, "-")
	id = manyDashes.ReplaceAllString(id, "-")
	return strings.ToLower(id)
}
