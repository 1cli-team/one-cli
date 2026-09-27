// Package buildcmd exposes finite project builds through one build.
package buildcmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	buildmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/build"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/dependencies"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

func Commands(provider runtimeport.Provider) []*cobra.Command {
	var project, environment string
	var dryRun bool
	cmd := &cobra.Command{
		Use: "build [project]", Args: cobra.MaximumNArgs(1),
		Example: "  one build\n  one build web\n  one build apps/web --dry-run\n  one build web --env prod",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := execution.ResolveWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			selector := strings.TrimSpace(project)
			if len(args) == 1 {
				if selector != "" {
					a, aOK := w.Project(args[0])
					b, bOK := w.Project(selector)
					if !aOK || !bOK || a.Name != b.Name {
						return fmt.Errorf("%s", i18n.T("build.selector_conflict"))
					}
				}
				selector = strings.TrimSpace(args[0])
			}
			plan, err := buildmodule.NewPlan(w, selector, environment)
			if err != nil {
				return err
			}
			if dryRun {
				output.Emit(plan)
				return nil
			}
			service := buildmodule.Service{Prepare: (dependencies.Service{Provider: provider}).Prepare}
			result, err := service.Execute(cmd.Context(), w, plan, cmd.ErrOrStderr())
			if result != nil {
				output.Emit(result)
			}
			return err
		},
	}
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
