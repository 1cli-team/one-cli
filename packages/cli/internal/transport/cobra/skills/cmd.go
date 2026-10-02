package skillscmd

import (
	"os"

	"github.com/spf13/cobra"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/skills"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

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
