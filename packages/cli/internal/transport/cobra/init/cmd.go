package initcmd

import (
	"github.com/spf13/cobra"

	hooks "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/hooks"
	mise "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/mise"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func Command() *cobra.Command {
	c := &cobra.Command{Use: "init", Short: i18n.T("init.short")}
	i18n.MarkShort(c, "init.short")
	c.AddCommand(mise.Commands()...)
	c.AddCommand(hooks.ConfigureCommand())
	return c
}
