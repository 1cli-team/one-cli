// Package devcmd contributes `one dev` to the explicit root command.
// Today there is one dev runner (Procfile-based local processes); this
// command calls into internal/modules/development/process directly.
package devcmd

import (
	"context"
	"fmt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/taskrun"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/dependencies"
	processorch "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/development/process"
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
		project                   string
		dryRun                    bool
		ui                        string
		selectProjects, keepGoing bool
	)
	cmd := &cobra.Command{
		Use:     "dev [projects...]",
		Long:    i18n.T("dev.tip"),
		Example: "  one dev\n  one dev web\n  one dev web api\n  one dev --select",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := taskrun.SignalContext(cmd.Context())
			defer stop()
			cmd.SetContext(ctx)
			activeWorkspace, err := execution.ResolveWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			root := activeWorkspace.Root()
			runtimeKind, err := execution.RuntimeKind(root)
			if err != nil {
				return err
			}
			processNames, err := activeWorkspace.SelectProjects(args, project)
			if err != nil {
				return err
			}
			if selectProjects {
				if len(processNames) > 0 {
					return fmt.Errorf("--select cannot be combined with project arguments")
				}
				if !output.CanPrompt() || dryRun {
					return fmt.Errorf("--select requires an interactive terminal without --dry-run")
				}
				available, _ := processorch.EntriesForProjects(activeWorkspace.Manifest(), nil)
				options := make([]prompt.Option[string], 0, len(available))
				for _, entry := range available {
					p, _ := activeWorkspace.Project(entry.Name)
					options = append(options, prompt.Option[string]{Label: entry.Name, Description: p.RelativeDir, Value: entry.Name})
				}
				if len(options) == 0 {
					return fmt.Errorf("no projects have a dev command")
				}
				processNames, err = prompt.MultiSelect(i18n.T("dev.select_title"), options, nil)
				if err != nil {
					return err
				}
				if len(processNames) == 0 {
					return fmt.Errorf("select at least one project")
				}
			}
			entries, err := processorch.EntriesForProjects(activeWorkspace.Manifest(), processNames)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return fmt.Errorf("no projects have a dev command")
			}
			mode, err := taskrun.ResolveMode(ui, len(entries))
			if err != nil {
				return err
			}
			selected := make([]string, 0, len(entries))
			for _, entry := range entries {
				selected = append(selected, entry.Name)
			}
			if !dryRun {
				if err := (dependencies.Service{Provider: provider}).Prepare(cmd.Context(), dependencies.Input{Root: root, Manifest: activeWorkspace.Manifest(), Projects: selected, Runtime: runtimeKind, Log: cmd.ErrOrStderr()}); err != nil {
					if ctx.Err() != nil {
						return context.Cause(ctx)
					}
					return err
				}
			}
			res, err := processorch.Start(cmd.Context(), processorch.StartInput{
				Runtime:     runtimeKind,
				ProjectRoot: root,
				DryRun:      dryRun,
				Processes:   processNames,
				UI:          mode,
				KeepGoing:   keepGoing,
			})
			if err != nil {
				return err
			}
			if dryRun && res != nil {
				if output.IsStructured() {
					output.Emit(res)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), strings.Join(res.Argv, " "))
				}
				return nil
			}
			if res != nil && output.IsStructured() {
				output.Emit(res)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ui, "ui", "auto", i18n.T("task.flag.ui"))
	cmd.Flags().BoolVar(&selectProjects, "select", false, i18n.T("dev.flag.select"))
	cmd.Flags().BoolVar(&keepGoing, "keep-going", false, i18n.T("dev.flag.keep_going"))
	i18n.MarkFlagUsage(cmd, "ui", "task.flag.ui")
	i18n.MarkFlagUsage(cmd, "select", "dev.flag.select")
	i18n.MarkFlagUsage(cmd, "keep-going", "dev.flag.keep_going")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.T("dev.flag.dry_run"))
	cmd.Flags().StringVarP(&project, "project", "p", "", i18n.T("dev.flag.project"))
	i18n.MarkFlagUsage(cmd, "dry-run", "dev.flag.dry_run")
	i18n.MarkFlagUsage(cmd, "project", "dev.flag.project")
	helpui.MarkAdvanced(cmd, "project")
	i18n.MarkShort(cmd, "dev.short")
	i18n.MarkLong(cmd, "dev.tip")
	return cmd
}
