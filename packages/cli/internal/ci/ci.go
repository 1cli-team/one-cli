// Package ci dispatches the per-subproject CI workflow rendering to a
// pkg/ci.Provider. Provider implementations register via init() (see
// internal/ci/providers/*), and this package just chooses which one
// runs based on the workspace's manifest.ci selection (or
// DefaultProviderID when unset).
//
// Public Sync / SyncOptions / SyncResult are kept stable — the move to
// providers is internal.
package ci

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	pkgci "github.com/torchstellar-team/one-cli/packages/cli/pkg/ci"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"

	// Side-effect imports register the bundled providers.
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/ci/providers/githubactions"
)

// SyncOptions bundles inputs from `add`.
type SyncOptions struct {
	ProjectRoot    string
	TargetDir      string
	ProjectName    string
	Toolchain      toolchain.Toolchain
	PackageManager toolchain.PackageManager
	// ProviderID names the CI provider to use. Empty selects
	// pkg/ci.DefaultProviderID.
	ProviderID string
}

// SyncResult tells the caller where the file landed and whether it was a
// fresh write.
type SyncResult struct {
	WorkflowPath string
	Created      bool
}

// Sync writes (or refreshes) the CI workflow file for the given
// subproject via the selected provider.
func Sync(opts SyncOptions) (SyncResult, error) {
	tc := opts.Toolchain
	if tc == "" {
		tc = toolchain.Node
	}
	pm := opts.PackageManager
	if pm == "" && tc == toolchain.Node {
		pm = toolchain.PMpnpm
	}

	provider := resolveProvider(opts.ProviderID)
	if provider == nil {
		return SyncResult{}, errors.New("ci: no provider available; ensure a provider package is imported")
	}

	scripts, err := loadScripts(opts.TargetDir)
	if err != nil {
		return SyncResult{}, err
	}
	relDir, err := filepath.Rel(opts.ProjectRoot, opts.TargetDir)
	if err != nil {
		return SyncResult{}, err
	}
	relDir = filepath.ToSlash(relDir)

	in := pkgci.Input{
		ProjectRoot:    opts.ProjectRoot,
		TargetDir:      opts.TargetDir,
		RelativeDir:    relDir,
		ProjectName:    opts.ProjectName,
		Toolchain:      tc,
		PackageManager: pm,
		Scripts:        scripts,
		Adapter:        toolchain.Get(tc),
	}
	relWorkflow := provider.WorkflowFilename(in)
	in.WorkflowFilePath = relWorkflow

	workflowPath := filepath.Join(opts.ProjectRoot, filepath.FromSlash(relWorkflow))
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		return SyncResult{}, err
	}
	created := !fileExists(workflowPath)

	content := provider.Render(in)
	if err := os.WriteFile(workflowPath, []byte(content), 0o644); err != nil {
		return SyncResult{}, err
	}
	return SyncResult{
		WorkflowPath: workflowPath,
		Created:      created,
	}, nil
}

// ResolvePath returns the canonical workflow path for a subproject
// using the default (or selected) provider. Kept for callers that
// only need the path while status checks for drift.
func ResolvePath(projectRoot, targetDir string) string {
	return ResolvePathFor(projectRoot, targetDir, "")
}

// ResolvePathFor is ResolvePath with an explicit provider id.
func ResolvePathFor(projectRoot, targetDir, providerID string) string {
	provider := resolveProvider(providerID)
	rel, err := filepath.Rel(projectRoot, targetDir)
	if err != nil {
		rel = targetDir
	}
	rel = filepath.ToSlash(rel)
	relWorkflow := provider.WorkflowFilename(pkgci.Input{
		ProjectRoot: projectRoot,
		TargetDir:   targetDir,
		RelativeDir: rel,
	})
	return filepath.Join(projectRoot, filepath.FromSlash(relWorkflow))
}

func resolveProvider(id string) pkgci.Provider {
	if id == "" {
		id = pkgci.DefaultProviderID
	}
	if p := pkgci.Lookup(id); p != nil {
		return p
	}
	// Fall back to the first registered provider so single-provider
	// builds don't blow up if the caller passes an unknown id.
	if all := pkgci.Providers(); len(all) > 0 {
		return all[0]
	}
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func loadScripts(targetDir string) (map[string]string, error) {
	pkgPath := filepath.Join(targetDir, "package.json")
	raw, err := os.ReadFile(pkgPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	type pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	var p pkg
	if err := json.Unmarshal(raw, &p); err != nil {
		return map[string]string{}, nil
	}
	if p.Scripts == nil {
		p.Scripts = map[string]string{}
	}
	return p.Scripts, nil
}
