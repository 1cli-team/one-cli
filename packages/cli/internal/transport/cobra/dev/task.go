package devcmd

import (
	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/tasks"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
	"strconv"
)

// TaskRunner keeps one run dev and one dev on the same supervisor path.
func TaskRunner(provider runtimeport.Provider, loaders *secrets.Registry) func(*cobra.Command, tasks.Options) error {
	return func(parent *cobra.Command, opts tasks.Options) error {
		cmd := newDevCmd(provider, loaders)
		cmd.SetContext(parent.Context())
		cmd.SetIn(parent.InOrStdin())
		cmd.SetOut(parent.OutOrStdout())
		cmd.SetErr(parent.ErrOrStderr())
		dry, _ := parent.Flags().GetBool("dry-run")
		_ = cmd.Flags().Set("dry-run", strconv.FormatBool(dry))
		_ = cmd.Flags().Set("ui", opts.UI)
		if opts.Environment != "" {
			_ = cmd.Flags().Set("env", opts.Environment)
		}
		return cmd.RunE(cmd, opts.Projects)
	}
}
