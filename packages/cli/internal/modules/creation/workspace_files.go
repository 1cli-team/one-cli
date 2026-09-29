package creation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

type workspaceFilesOptions struct {
	ProjectName string
}

// Names of bundled files that the workspace ships with. Centralised so a
// future drift between core packages and scaffolder is caught at compile
// time, not at runtime.
const (
	WorkspaceFilename = "pnpm-workspace.yaml"
	ManifestFilename  = workspace.ManifestFilename
)

// generateWorkspaceFiles writes the workspace skeleton. Project files are
// materialised from templates later.
func generateWorkspaceFiles(targetDir string, opts workspaceFilesOptions) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	for _, sub := range []string{"apps", "services", "packages"} {
		if err := os.MkdirAll(filepath.Join(targetDir, sub), 0o755); err != nil {
			return err
		}
	}
	if err := workspace.WriteManifest(targetDir, emptyManifest(opts.ProjectName)); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(targetDir, ".gitignore"), []byte(gitignoreContent), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(targetDir, "AGENTS.md"), []byte(agentsContent()), 0o644); err != nil {
		return err
	}

	return nil
}

// isDirectoryEmpty returns true if targetDir does not exist OR exists
// and contains no entries.
func isDirectoryEmpty(targetDir string) (bool, error) {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	return len(entries) == 0, nil
}

func initGitRepo(cwd string) error {
	if _, err := os.Lstat(filepath.Join(cwd, ".git")); err == nil {
		return validateGitRoot(cwd)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = cwd
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// writeJSON writes v as 2-space-indented JSON to path, matching fs.writeJSON
// from fs-extra (the TS codebase relies on this exact spacing for file
// readability + diffability). fs-extra appends a trailing newline; we do too,
// otherwise byte-level fixture diffs would flag every JSON file as drifted.
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// validateEmptyTarget allows only a new directory or an empty Git worktree.
// A missing working tree with staged/tracked deletions is not an empty project.
func validateEmptyTarget(targetDir, displayPath string) error {
	entries, err := os.ReadDir(targetDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return i18n.Errorf("creation.read_target_failed", displayPath, err)
	}
	var conflicts []string
	hasGit := false
	for _, entry := range entries {
		if entry.Name() == ".git" {
			hasGit = true
			continue
		}
		conflicts = append(conflicts, entry.Name())
	}
	if len(conflicts) > 0 {
		shown := conflicts
		if len(shown) > 5 {
			shown = append(append([]string(nil), shown[:5]...), "…")
		}
		return cliErrors.New(cliErrors.EXISTING_TARGET_NOT_EMPTY,
			i18n.Tf("creation.target_not_empty", displayPath, strings.Join(shown, ", "))).
			WithContext(map[string]any{"target_path": targetDir, "display_path": displayPath, "conflicting_entries": conflicts})
	}
	if !hasGit {
		return nil
	}
	if err := validateGitRoot(targetDir); err != nil {
		return err
	}
	for _, args := range [][]string{{"ls-files", "-z"}, {"status", "--porcelain", "--untracked-files=all"}} {
		output, err := gitOutput(targetDir, args...)
		if err != nil {
			return i18n.Errorf("creation.git_inspection_failed", displayPath, err)
		}
		if strings.TrimSpace(output) != "" {
			return cliErrors.New(cliErrors.EXISTING_TARGET_NOT_EMPTY,
				i18n.Tf("creation.git_not_empty", displayPath)).
				WithContext(map[string]any{"target_path": targetDir})
		}
	}
	return nil
}

func gitOutput(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	// Reading status must not refresh an existing index on disk.
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func validateGitRoot(root string) error {
	top, err := gitOutput(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return i18n.Errorf("creation.git_inspection_failed", root, err)
	}
	actual, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	expected, err := filepath.EvalSymlinks(top)
	if err != nil {
		return err
	}
	actual, err = filepath.Abs(actual)
	if err != nil {
		return err
	}
	if filepath.Clean(actual) != filepath.Clean(expected) {
		return i18n.Errorf("creation.git_root_mismatch", root, top)
	}
	return nil
}

func hasSharedGitDirectory(root string) (bool, error) {
	dir, err := gitOutput(root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return false, err
	}
	common, err := gitOutput(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	return filepath.Clean(dir) != filepath.Clean(common), err
}
