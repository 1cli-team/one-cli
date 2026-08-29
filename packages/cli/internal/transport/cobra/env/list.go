package envcmd

import (
	"github.com/spf13/cobra"

	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func newListCmd(deps Dependencies) *cobra.Command {
	var project, environment, profile string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有 KEY",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := deps.Service.List(cmd.Context(), environmentmodule.ListInput{
				Scope: commandScope(cmd), Environment: environment,
				Project: project, Profile: profile,
			})
			if err != nil {
				return err
			}
			output.Emit(listOutput{result})
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", i18n.T("env.flag.project"))
	cmd.Flags().StringVar(&environment, "env", "", i18n.T("env.flag.environment"))
	cmd.Flags().StringVar(&profile, "profile", "", i18n.T("env.flag.profile"))
	markEnvFlagUsage(cmd, "project", "env", "profile")
	i18n.MarkShort(cmd, "env.list.short")
	return cmd
}
