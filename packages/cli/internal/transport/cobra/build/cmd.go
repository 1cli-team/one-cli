// Package buildcmd exposes finite project builds through one build.
package buildcmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	buildmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/build"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/dependencies"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/taskrun"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"strings"
)

func Commands(provider runtimeport.Provider) []*cobra.Command {
	var project, environment string
	var dryRun bool
	var ui string
	var concurrency int
	cmd := &cobra.Command{
		Use: "build [projects...]", Args: cobra.ArbitraryArgs,
		Example: "  one build\n  one build web\n  one build apps/web --dry-run\n  one build web --env prod",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := execution.ResolveWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			selectors, err := w.SelectProjects(args, project)
			if err != nil {
				return err
			}
			if concurrency < 1 {
				return fmt.Errorf("--concurrency must be at least 1")
			}
			plan, err := buildmodule.NewPlanForProjects(w, selectors, environment)
			if err != nil {
				return err
			}
			count := 0
			for _, task := range plan.Tasks {
				if task.Status == "pending" {
					count++
				}
			}
			mode, err := taskrun.ResolveMode(ui, count)
			if err != nil {
				return err
			}
			if dryRun {
				output.Emit(plan)
				return nil
			}
			service := buildmodule.Service{UI: mode, Concurrency: concurrency, Prepare: (dependencies.Service{Provider: provider}).Prepare}
			result, err := service.Execute(cmd.Context(), w, plan, cmd.ErrOrStderr())
			if result != nil {
				output.Emit(result)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&ui, "ui", "auto", i18n.T("task.flag.ui"))
	cmd.Flags().IntVar(&concurrency, "concurrency", 1, i18n.T("build.flag.concurrency"))
	i18n.MarkFlagUsage(cmd, "ui", "task.flag.ui")
	i18n.MarkFlagUsage(cmd, "concurrency", "build.flag.concurrency")
	cmd.Flags().StringVarP(&project, "project", "p", "", i18n.T("build.flag.project"))
	cmd.Flags().StringVar(&environment, "env", "", i18n.T("build.flag.env"))
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.T("build.flag.dry_run"))
	for _, flag := range []string{"project", "env", "dry-run"} {
		key := strings.ReplaceAll(flag, "-", "_")
		i18n.MarkFlagUsage(cmd, flag, "build.flag."+key)
	}
	helpui.MarkAdvanced(cmd, "project", "env")
	i18n.MarkShort(cmd, "build.short")
	i18n.MarkLong(cmd, "build.tip")
	return []*cobra.Command{cmd}
}
