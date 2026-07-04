// Package scaffold writes the workspace skeleton produced by `one
// create`. The package is deliberately stateless — every function
// takes the absolute target dir and writes side-effects relative to
// it. Callers (internal/cli) own the prompts, error handling, and JSON
// payload assembly.
package scaffold

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// Options mirrors the TS CreateOptions. The package manager is fixed to pnpm
// for now (one-cli is opinionated); add new options when the TS side does.
type Options struct {
	ProjectName    string
	PackageManager string // "pnpm"
	Docker         bool
	K8s            bool
}

// Names of bundled files that the workspace ships with. Centralised so a
// future drift between core packages and scaffolder is caught at compile
// time, not at runtime.
const (
	WorkspaceFilename = "pnpm-workspace.yaml"
	ManifestFilename  = "one.manifest.json"
)

// Generate writes the full workspace skeleton into targetDir. If options.Docker
// is true, also produces docker-compose.yml. Same for k8s. Caller is expected
// to have already validated the project name and ensured the target directory
// is empty (or does not yet exist).
func Generate(targetDir string, opts Options) error {
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

	if opts.Docker {
		if err := os.WriteFile(filepath.Join(targetDir, "docker-compose.yml"), []byte(dockerComposeContent), 0o644); err != nil {
			return err
		}
	}
	if opts.K8s {
		if err := os.MkdirAll(filepath.Join(targetDir, "k8s"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(targetDir, "k8s", "deployment.yaml"), []byte(k8sDeploymentContent), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// IsDirectoryEmpty returns true if targetDir does not exist OR exists
// and contains no entries.
func IsDirectoryEmpty(targetDir string) (bool, error) {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	return len(entries) == 0, nil
}

// InitGitRepo runs `git init` in cwd. Mirrors initGitRepo in TS but we use
// os/exec instead of node:child_process. Returns a non-nil error if git is
// missing or the subprocess exits non-zero.
func InitGitRepo(cwd string) error {
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
