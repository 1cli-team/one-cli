package initcmd

import (
	"github.com/spf13/cobra"
	hooks "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/hooks"
	mise "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/mise"
)

func Command() *cobra.Command {
	c := &cobra.Command{Use: "init", Short: "生成工作区工具配置"}
	c.AddCommand(mise.Commands()...)
	c.AddCommand(hooks.ConfigureCommand())
	return c
}
