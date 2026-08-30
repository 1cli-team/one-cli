package envcmd

import (
	"github.com/spf13/cobra"

	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func newGetCmd(deps Dependencies) *cobra.Command {
	var project, environment, profile string
	cmd := &cobra.Command{
		Use:   "get <KEY>",
		Short: "读取一个环境变量值",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := deps.Service.Get(cmd.Context(), environmentmodule.GetInput{
				Scope: commandScope(cmd), Environment: environment,
				Project: project, Profile: profile, Key: args[0],
			})
			if err != nil {
				return err
			}
			output.Emit(getOutput{result})
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", i18n.T("env.flag.project"))
	cmd.Flags().StringVar(&environment, "env", "", i18n.T("env.flag.environment"))
	cmd.Flags().StringVar(&profile, "profile", "", i18n.T("env.flag.profile"))
	markEnvFlagUsage(cmd, "project", "env", "profile")
	i18n.MarkShort(cmd, "env.get.short")
	return cmd
}
