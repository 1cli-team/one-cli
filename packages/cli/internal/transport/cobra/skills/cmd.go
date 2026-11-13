package skillscmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/skills"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
)

func ConfigureCommand(initializer skillsmodule.Initializer) *cobra.Command {
	var dryRun bool
	var names []string
	cmd := &cobra.Command{
		Use: "skills", Args: i18n.NoArgs,
		Example: "  one init skills --dry-run -o json\n  one init skills\n  one init skills --skill one-go\n  one skills update --project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, err := execution.ResolveWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			result, err := initializer.Init(cmd.Context(), workspace.Root(), names, dryRun)
			if err != nil {
				return err
			}
			output.Emit(result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.T("skills.init_dry_run"))
	cmd.Flags().StringSliceVar(&names, "skill", nil, i18n.T("skills.init_names"))
	i18n.MarkFlagUsage(cmd, "dry-run", "skills.init_dry_run")
	i18n.MarkFlagUsage(cmd, "skill", "skills.init_names")
	i18n.MarkShort(cmd, "skills.init_short")
	i18n.MarkLong(cmd, "skills.init_tip")
	return cmd
}

func Commands(run skillsmodule.RunFunc) []*cobra.Command {
	cmd := &cobra.Command{
		Use: "skills [args...]", DisableFlagParsing: true,
		Example: "  one skills find react\n  one skills add owner/repo --skill skill-name\n  one skills list\n  one skills update",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			return run(cmd.Context(), dir, args, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	i18n.MarkShort(cmd, "skills.short")
	i18n.MarkLong(cmd, "skills.tip")
	return []*cobra.Command{cmd}
}
