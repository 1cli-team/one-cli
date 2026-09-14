package hooks

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/miseconfig"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
)

type Result struct {
	Schema       string              `json:"schema"`
	Root         string              `json:"root"`
	DryRun       bool                `json:"dry_run"`
	Changes      []fsutil.FileChange `json:"changes"`
	GitChanges   []fsutil.FileChange `json:"git_changes"`
	GitDirectory string              `json:"git_directory"`
	HooksPath    string              `json:"hooks_path,omitempty"`
	Warnings     []string            `json:"warnings,omitempty"`
}

func (r *Result) RenderTTY(w io.Writer) {
	if r.DryRun {
		fmt.Fprintln(w, "Proposed hk configuration and local Git hooks:")
	} else {
		fmt.Fprintln(w, "hk configuration refreshed; Git hooks now run through One.")
	}
	for _, change := range append(append([]fsutil.FileChange{}, r.Changes...), r.GitChanges...) {
		action := "write"
		if change.Remove {
			action = "remove"
		}
		fmt.Fprintf(w, "  %s %s\n", action, change.Path)
	}
	for _, warning := range r.Warnings {
		fmt.Fprintf(w, "  %s\n", warning)
	}
	fmt.Fprintln(w, "Check: one hk check --all\nFix:   one hk fix")
}

// Configure validates the complete migration before publishing any files. It
// neither downloads tools nor installs application dependencies.
func Configure(ctx context.Context, root, binary string, dryRun bool) (*Result, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	unlock, err := fsutil.WorkspaceLock(ctx, root, "creation")
	if err != nil {
		return nil, err
	}
	defer unlock()
	unlockMise, err := fsutil.WorkspaceLock(ctx, root, "mise")
	if err != nil {
		return nil, err
	}
	defer unlockMise()
	p := fsutil.NewFilePlan(root)
	if _, err := p.Read(workspace.ManifestFilename); err != nil {
		return nil, err
	}
	m, err := workspace.ReadManifest(root)
	if err != nil {
		return nil, err
	}
	legacy, err := planLegacy(p)
	if err != nil {
		return nil, err
	}
	if err := PlanFiles(p, m); err != nil {
		return nil, err
	}
	mise, err := miseconfig.BuildWithFiles(root, miseconfig.Options{}, p.Overlay())
	if err != nil {
		return nil, err
	}
	for path, b := range mise.ReadInputs() {
		if err := p.Expect(path, b); err != nil {
			return nil, err
		}
	}
	for _, c := range mise.Changes {
		if err := p.Set(c.Path, []byte(c.After), 0o644); err != nil {
			return nil, err
		}
	}
	install, err := PlanInstall(ctx, root, binary, legacy)
	if err != nil {
		return nil, err
	}
	result := &Result{Schema: "one-cli/hooks-config/v1", Root: root, DryRun: dryRun, Changes: p.Changes(), GitChanges: install.Files.Changes(), GitDirectory: install.Files.Root, HooksPath: install.nextPath}
	if legacy {
		result.Warnings = []string{"Husky/commitlint defaults were migrated. Run your package manager's install through one mise exec to refresh the dependency lockfile."}
	}
	if !dryRun {
		if err := p.Apply(ctx); err != nil {
			return nil, err
		}
		if err := install.Apply(ctx); err != nil {
			return nil, fmt.Errorf("hk configuration was written but Git hook installation failed; resolve the conflict and rerun one configure hooks: %w", err)
		}
	}
	return result, nil
}
