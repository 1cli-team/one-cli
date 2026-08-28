// Package devcmd contributes `one dev` to the root command via cliexts.
// Today there is one dev runner (Procfile-based local processes); this
// command calls into internal/localorch/process directly.
package devcmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	processorch "github.com/torchstellar-team/one-cli/packages/cli/internal/localorch/process"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func init() {
	cliexts.Register("dev", buildContributions)
}

func buildContributions() []*cobra.Command {
	return []*cobra.Command{newDevCmd()}
}

// requireDev is retained for v4 backwards compatibility but is now a
// no-op: current workspaces always have the local dev runner enabled — the
// Procfile.dev synchronisation runs unconditionally during `one create`
// and `one add`, so any project root with a valid manifest is dev-ready.
func requireDev(projectRoot string) error {
	if _, err := workspace.ReadManifest(projectRoot); err != nil {
		return err
	}
	return nil
}

// _ keeps the cliErrors import even when requireDev no longer uses it,
// in case future hardening reintroduces an explicit gate.
var _ = cliErrors.BACKEND_NOT_ENABLED

func newDevCmd() *cobra.Command {
	var (
		project string
		dryRun  bool
	)
	cmd := &cobra.Command{
		Use:     "dev [project]",
		Long:    i18n.T("dev.tip"),
		Example: "  one dev\n  one dev web",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			positional := ""
			if len(args) > 0 {
				positional = args[0]
			}
			if positional != "" && project != "" && positional != project {
				return cliErrors.New(cliErrors.ONE_CLI_ERROR,
					i18n.T("dev.selector_conflict"))
			}
			if positional != "" {
				project = positional
			}
			root, err := workspace.ResolveProjectRoot("")
			if err != nil {
				return err
			}
			if err := requireDev(root); err != nil {
				return err
			}
			processName, err := resolveProcessSelector(root, project)
			if err != nil {
				return err
			}
			if !dryRun {
				if err := ensureDependencies(cmd, root, processName); err != nil {
					return err
				}
			}
			res, err := processorch.Start(cmd.Context(), processorch.StartInput{
				ProjectRoot: root,
				DryRun:      dryRun,
				Process:     processName,
			})
			if err != nil {
				return err
			}
			if dryRun && res != nil {
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(res.Argv, " "))
				return nil
			}
			if res != nil {
				output.Emit(res)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.T("dev.flag.dry_run"))
	cmd.Flags().StringVarP(&project, "project", "p", "", i18n.T("dev.flag.project"))
	i18n.MarkFlagUsage(cmd, "dry-run", "dev.flag.dry_run")
	i18n.MarkFlagUsage(cmd, "project", "dev.flag.project")
	helpui.MarkAdvanced(cmd, "project")
	i18n.MarkShort(cmd, "dev.short")
	i18n.MarkLong(cmd, "dev.tip")
	return cmd
}

func ensureDependencies(cmd *cobra.Command, root, projectName string) error {
	m, err := workspace.ReadManifest(root)
	if err != nil {
		return err
	}
	needsInstall := false
	packageManager := ""
	for i := range m.Projects {
		p := &m.Projects[i]
		if projectName != "" && p.Name != projectName {
			continue
		}
		if p.Toolchain != "node" || strings.TrimSpace(workspace.ProjectDev(m, p.Name)) == "" {
			continue
		}
		projectDir := filepath.Join(root, filepath.FromSlash(p.RelativeDir))
		if !workspace.ProjectDependenciesInstalled(root, projectDir, p.Toolchain) {
			needsInstall = true
		}
		if packageManager == "" {
			packageManager = strings.TrimSpace(p.PackageManager)
		}
	}
	if !needsInstall {
		return nil
	}
	install := dependencyInstallCommand(root, packageManager)
	installLine := strings.Join(install, " ")
	if !output.CanPrompt() {
		return cliErrors.New(cliErrors.DEPENDENCIES_NOT_INSTALLED,
			i18n.T("dev.dependencies_missing")).
			WithRemediation(output.Remediation{
				Action:  "install-dependencies",
				Hint:    i18n.T("dev.install_hint"),
				Command: installLine,
			})
	}
	ok, err := prompt.Confirm(i18n.Tf("dev.install_confirm", installLine), true,
		i18n.T("common.install_continue"), i18n.T("common.cancel"))
	if err != nil {
		return err
	}
	if !ok {
		return cliErrors.New(cliErrors.PROMPT_CANCELLED, i18n.T("common.cancelled")).WithExit0()
	}
	if _, err := exec.LookPath(install[0]); err != nil {
		return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND,
			i18n.Tf("dev.package_manager_missing", install[0]))
	}
	child := exec.CommandContext(cmd.Context(), install[0], install[1:]...)
	child.Dir = root
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Run(); err != nil {
		return cliErrors.New(cliErrors.ONE_CLI_ERROR,
			i18n.Tf("dev.install_failed", installLine, err))
	}
	return nil
}

func dependencyInstallCommand(root, projectPackageManager string) []string {
	manager := strings.TrimSpace(projectPackageManager)
	if pkg, err := workspace.ReadPackageJSON(root); err == nil && pkg != nil && pkg.PackageManager != "" {
		manager = pkg.PackageManager
	}
	if i := strings.Index(manager, "@"); i > 0 {
		manager = manager[:i]
	}
	if manager == "" {
		switch {
		case fileExists(filepath.Join(root, "bun.lock")), fileExists(filepath.Join(root, "bun.lockb")):
			manager = "bun"
		case fileExists(filepath.Join(root, "yarn.lock")):
			manager = "yarn"
		case fileExists(filepath.Join(root, "package-lock.json")):
			manager = "npm"
		default:
			manager = "pnpm"
		}
	}
	return []string{manager, "install"}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// resolveProcessSelector turns the user-facing -p value into a manifest
// project name (which equals the Procfile.dev entry name). Empty input
// yields empty output, meaning "all processes".
func resolveProcessSelector(projectRoot, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", nil
	}
	sub, err := workspace.ResolveProjectFromSelector(projectRoot, selector)
	if err != nil {
		return "", err
	}
	if sub == nil {
		m, _ := workspace.ReadManifest(projectRoot)
		return "", cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			fmt.Sprintf("没有名为 %s 的 project", selector)).
			WithContext(map[string]any{
				"selector":           selector,
				"available_projects": workspace.ProjectNames(m),
			})
	}
	return sub.Name, nil
}
