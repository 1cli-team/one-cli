package envcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

func newSwitchCmd(deps Dependencies) *cobra.Command {
	var yes, noSync, overwrite, dryRun bool
	cmd := &cobra.Command{
		Use:   "switch <backend>",
		Short: "切换工作区的 env 后端 (dotenv / infisical)",
		Long:  i18n.T("env.switch.tip"),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := deps.Service.PlanSwitch(commandScope(cmd), args[0])
			if err != nil {
				return err
			}
			sync := !noSync
			if sync && !yes && plan.Entries() > 0 {
				sync, err = prompt.Confirm(
					fmt.Sprintf("发现 %d 个本地 .env 条目要推到 Infisical，继续？", plan.Entries()),
					true, "是，同步并切换", "否，只切 manifest",
				)
				if err != nil {
					return err
				}
			}
			result, err := deps.Service.Switch(cmd.Context(), plan, environmentmodule.SwitchOptions{
				Sync: sync, Overwrite: overwrite, DryRun: dryRun,
			})
			if err != nil {
				return err
			}
			output.Emit(switchOutput{result})
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, i18n.T("env.switch.flag.yes"))
	cmd.Flags().BoolVar(&noSync, "no-sync", false, i18n.T("env.switch.flag.no_sync"))
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, i18n.T("env.switch.flag.overwrite"))
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.T("env.flag.dry_run"))
	i18n.MarkFlagUsage(cmd, "yes", "env.switch.flag.yes")
	i18n.MarkFlagUsage(cmd, "no-sync", "env.switch.flag.no_sync")
	i18n.MarkFlagUsage(cmd, "overwrite", "env.switch.flag.overwrite")
	i18n.MarkFlagUsage(cmd, "dry-run", "env.flag.dry_run")
	i18n.MarkShort(cmd, "env.switch.short")
	i18n.MarkLong(cmd, "env.switch.tip")
	return cmd
}
