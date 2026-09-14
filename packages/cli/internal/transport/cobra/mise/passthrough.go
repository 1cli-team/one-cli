package misecmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

// RuntimeCommands exposes maintenance of the same mise executable used by
// one run/dev. In particular, trust must work before mise exec can load config.
func RuntimeCommands(provider runtimeport.Provider) []*cobra.Command {
	cmd := &cobra.Command{
		Use: "mise [args...]", DisableFlagParsing: true,
		Example: "  one mise trust\n  one mise doctor\n  one mise exec -- pnpm install",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
				return cmd.Help()
			}
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			prepared, err := provider.PrepareCLI(cmd.Context(), runtimeport.Command{Directory: dir, Argv: args, Env: os.Environ()})
			if err != nil {
				return err
			}
			child := platformprocess.Command(prepared.Argv[0], prepared.Argv[1:]...)
			child.Dir, child.Env = prepared.Directory, prepared.Env
			child.Stdin, child.Stdout, child.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
			return platformprocess.RunForwarded(cmd.Context(), child)
		},
	}
	i18n.MarkShort(cmd, "mise.short")
	i18n.MarkLong(cmd, "mise.tip")
	return []*cobra.Command{cmd}
}
