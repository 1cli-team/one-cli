// Package envcmd exposes the environment feature module through Cobra. It
// owns flags, prompts, progress presentation, and output only; backend and
// workspace workflows live in modules/environment.
package envcmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

type Dependencies struct {
	Service *environmentmodule.Service
}

func Commands(deps Dependencies) []*cobra.Command {
	parent := &cobra.Command{
		Use:     "env",
		Long:    i18n.T("env.tip"),
		Example: "  one env\n  one env set DATABASE_URL\n  one env list",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			summary, err := deps.Service.Summary(commandScope(cmd))
			if err != nil {
				return err
			}
			output.Emit(summaryOutput{summary})
			return nil
		},
	}
	children := []*cobra.Command{
		newGetCmd(deps), newSetCmd(deps), newListCmd(deps), newPullCmd(deps), newSwitchCmd(deps),
	}
	for _, child := range children {
		helpui.MarkAdvanced(child, "profile")
	}
	parent.AddCommand(children...)
	i18n.MarkShort(parent, "env.short")
	i18n.MarkLong(parent, "env.tip")
	return []*cobra.Command{parent}
}

func commandScope(cmd *cobra.Command) execution.Scope {
	if scope, ok := execution.FromContext(cmd.Context()); ok {
		return scope
	}
	workingDirectory, _ := os.Getwd()
	return execution.NewScope(cmd.Context(), workingDirectory)
}
