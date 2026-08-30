package creation

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

type workspaceFilesOptions struct {
	ProjectName    string
	PackageManager string // "pnpm"
}

// Names of bundled files that the workspace ships with. Centralised so a
// future drift between core packages and scaffolder is caught at compile
// time, not at runtime.
const (
	WorkspaceFilename = "pnpm-workspace.yaml"
	ManifestFilename  = "one.manifest.json"
)

// generateWorkspaceFiles writes the workspace skeleton. Project-level
// container and deployment artifacts are materialised from Templates later.
func generateWorkspaceFiles(targetDir string, opts workspaceFilesOptions) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	for _, sub := range []string{".changeset", ".husky", "apps", "services", "packages"} {
		if err := os.MkdirAll(filepath.Join(targetDir, sub), 0o755); err != nil {
			return err
		}
	}

	pkg := buildPackageJSON(opts.ProjectName)
	if err := writeJSON(filepath.Join(targetDir, "package.json"), pkg); err != nil {
		return err
	}

	if err := writeJSON(filepath.Join(targetDir, ManifestFilename), emptyManifest(opts.ProjectName)); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(targetDir, WorkspaceFilename), []byte(pnpmWorkspaceContent), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(targetDir, ".gitignore"), []byte(gitignoreContent), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(targetDir, "commitlint.config.js"), []byte(commitlintConfigContent), 0o644); err != nil {
		return err
	}
	// Workspace-level agent harness. AGENTS.md is canonical; CLAUDE.md is
	// a thin pointer so tool-specific files do not drift.
	if err := os.WriteFile(filepath.Join(targetDir, "AGENTS.md"), []byte(buildRootAgentsMd(opts.ProjectName)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(targetDir, "CLAUDE.md"), []byte(buildClaudeMdPointer()), 0o644); err != nil {
		return err
	}
	agentsDir := filepath.Join(targetDir, ".one", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "conventions.md"), []byte(buildAgentsConventionsMd()), 0o644); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(targetDir, ".changeset", "config.json"), changesetConfig); err != nil {
		return err
	}

	huskyPreCommit := filepath.Join(targetDir, ".husky", "pre-commit")
	huskyCommitMsg := filepath.Join(targetDir, ".husky", "commit-msg")
	if err := os.WriteFile(huskyPreCommit, []byte(huskyPreCommitContent), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(huskyCommitMsg, []byte(huskyCommitMsgContent), 0o755); err != nil {
		return err
	}
	// Re-chmod explicitly: WriteFile may apply umask which masks 0o755.
	if err := os.Chmod(huskyPreCommit, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(huskyCommitMsg, 0o755); err != nil {
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
