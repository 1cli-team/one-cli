// Package devcmd contributes `one dev` to the explicit root command.
// Today there is one dev runner (Procfile-based local processes); this
// command calls into internal/modules/development/process directly.
package devcmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/dependencies"
	processorch "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/development/process"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

func Commands(provider runtimeport.Provider) []*cobra.Command { return buildContributions(provider) }

func buildContributions(provider runtimeport.Provider) []*cobra.Command {
	return []*cobra.Command{newDevCmd(provider)}
}

func newDevCmd(provider runtimeport.Provider) *cobra.Command {
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
			activeWorkspace, err := execution.ResolveWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			root := activeWorkspace.Root()
			runtimeKind, err := execution.RuntimeKind(root)
			if err != nil {
				return err
			}
			processName, err := resolveProcessSelector(activeWorkspace, project)
			if err != nil {
				return err
			}
			if !dryRun {
				if err := (dependencies.Service{Provider: provider}).Prepare(cmd.Context(), dependencies.Input{Root: root, Manifest: activeWorkspace.Manifest(), Project: processName, Runtime: runtimeKind, Log: cmd.ErrOrStderr()}); err != nil {
					return err
				}
			}
			res, err := processorch.Start(cmd.Context(), processorch.StartInput{
				Runtime:     runtimeKind,
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

// resolveProcessSelector turns the user-facing -p value into a manifest
// project name (which equals the Procfile.dev entry name). Empty input
// yields empty output, meaning "all processes".
func resolveProcessSelector(activeWorkspace execution.Workspace, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", nil
	}
	project, ok := activeWorkspace.Project(selector)
	if !ok {
		return "", cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			fmt.Sprintf("没有名为 %s 的 project", selector)).
			WithContext(map[string]any{
				"selector":           selector,
				"available_projects": activeWorkspace.ProjectNames(),
			})
	}
	return project.Name, nil
}
