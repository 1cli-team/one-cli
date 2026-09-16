package skills

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/resources/bundled"
)

const Name = "one-cli"

// Install copies the single bundled skill into each distinct target directory.
// It needs neither a workspace nor a network connection. On partial failure the
// returned paths identify the targets that were already installed successfully.
func Install(ctx context.Context, targets []Agent) ([]string, error) {
	installed := []string{}
	if len(targets) == 0 {
		return installed, errors.New("no target agents selected")
	}
	seen := map[string]bool{}
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return installed, err
		}
		root := filepath.Clean(target.GlobalPath)
		if !filepath.IsAbs(root) {
			return installed, fmt.Errorf("agent %s: installation directory must be absolute", target.ID)
		}
		if seen[root] {
			continue
		}
		if err := installAt(root); err != nil {
			return installed, fmt.Errorf("install %s for %s at %s: %w", Name, target.ID, root, err)
		}
		seen[root] = true
		installed = append(installed, root)
	}
	return installed, nil
}

func installAt(root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	// Stage a complete directory beside the destination. Renaming an old symlink
	// moves the link itself; its shared store (and other agents using it) is never
	// traversed or modified. Backups stay available until publication succeeds.
	stage, err := os.MkdirTemp(root, ".one-cli-install-*")
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(stage)
		}
	}()
	next := filepath.Join(stage, "next")
	if err := os.Mkdir(next, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(next, "SKILL.md"), bundled.OneCLISkill, 0o644); err != nil {
		return err
	}
	dest := filepath.Join(root, Name)
	backup := filepath.Join(stage, "previous")
	hadPrevious := false
	if info, err := os.Lstat(dest); err == nil {
		if !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("destination is not a skill directory: %s", dest)
		}
		if err := os.Rename(dest, backup); err != nil {
			return err
		}
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(next, dest); err != nil {
		if hadPrevious {
			if restoreErr := os.Rename(backup, dest); restoreErr != nil {
				// Preserve the only remaining old copy if restoration also fails.
				cleanup = false
				return fmt.Errorf("publish: %w; restore failed: %v; previous installation is at %s", err, restoreErr, backup)
			}
		}
		return err
	}
	return nil
}
