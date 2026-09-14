package hooks

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
)

const launcherHeader = "# Managed by One CLI hooks/v1; sha256="

type InstallPlan struct {
	Root         string
	Files        *fsutil.FilePlan
	previousPath string
	nextPath     string
}

// PlanInstall never invokes hk or downloads a tool. Git's machine-local
// launchers enter through One so GUI clients need no activated mise shell.
func PlanInstall(ctx context.Context, root, binary string, migrateHusky bool) (*InstallPlan, error) {
	top, err := gitOutput(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("initialize Git, then run one configure hooks: %w", err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	canonicalTop, err := filepath.EvalSymlinks(top)
	if err != nil || canonicalRoot != canonicalTop {
		return nil, conflict(root, "the One workspace must be the Git repository root")
	}
	common, err := gitOutput(ctx, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil {
		return nil, err
	}
	configured, err := hooksPath(ctx, root)
	if err != nil {
		return nil, err
	}
	defaultPath := filepath.Join(common, "hooks")
	if configured != "" {
		resolved := configured
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(root, resolved)
		}
		if filepath.Clean(resolved) != defaultPath && !(migrateHusky && (filepath.ToSlash(configured) == ".husky/_" || filepath.ToSlash(configured) == ".husky")) {
			return nil, conflict("core.hooksPath", "existing hook directory "+configured+" is user-managed; preserve and integrate its checks before changing the hook setup")
		}
	}
	if binary == "" {
		binary, err = os.Executable()
		if err != nil {
			return nil, err
		}
	}
	binary, err = filepath.Abs(binary)
	if err != nil {
		return nil, err
	}
	files := fsutil.NewFilePlan(common)
	for _, hook := range []string{"pre-commit", "commit-msg"} {
		path := "hooks/" + hook
		before, err := files.Read(path)
		if err != nil {
			return nil, err
		}
		if before != nil {
			if err := validateManaged(path, []byte(strings.TrimPrefix(string(before), "#!/bin/sh\n")), launcherHeader); err != nil {
				return nil, err
			}
		}
		body := "[ -f hk.pkl ] || exit 0\n" +
			"if [ -x " + shellQuote(filepath.ToSlash(binary)) + " ]; then\n  exec " + shellQuote(filepath.ToSlash(binary)) + " hk run " + hook + " \"$@\"\nfi\n" +
			"exec one hk run " + hook + " \"$@\"\n"
		content := fmt.Sprintf("#!/bin/sh\n%s%x\n# One supplies mise; no shell activation is needed.\n%s", launcherHeader, sha256.Sum256([]byte(body)), body)
		if err := files.Set(path, []byte(content), 0o755); err != nil {
			return nil, err
		}
	}
	nextPath := ""
	if configured != "" && filepath.Clean(configured) != defaultPath {
		// A local absolute path overrides the previous Husky/global setting
		// without changing any user-global Git configuration.
		nextPath = filepath.ToSlash(defaultPath)
	}
	return &InstallPlan{Root: root, Files: files, previousPath: configured, nextPath: nextPath}, nil
}

func (p *InstallPlan) Apply(ctx context.Context) error {
	unlock, err := fsutil.WorkspaceLock(ctx, p.Files.Root, "hooks-install")
	if err != nil {
		return err
	}
	defer unlock()
	current, err := hooksPath(ctx, p.Root)
	if err != nil {
		return err
	}
	if current != p.previousPath {
		return conflict("core.hooksPath", "changed while configuring hooks; retry")
	}
	if err := p.Files.Apply(ctx); err != nil {
		return err
	}
	// Repair executable bits without changing user-written hook contents.
	for _, hook := range []string{"pre-commit", "commit-msg"} {
		if err := os.Chmod(filepath.Join(p.Files.Root, "hooks", hook), 0o755); err != nil {
			return err
		}
	}
	if p.nextPath != "" {
		_, err = gitOutput(ctx, p.Root, "config", "--local", "core.hooksPath", p.nextPath)
	}
	return err
}

func hooksPath(ctx context.Context, root string) (string, error) {
	value, err := gitOutput(ctx, root, "config", "--get", "core.hooksPath")
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return "", nil
	}
	return value, err
}

func gitOutput(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
