package runcmd

import (
	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/tasks"
)

func processWorker() *cobra.Command {
	return &cobra.Command{Use: "__process id", Hidden: true, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return tasks.RunProcessLeaf(cmd.Context(), args[0], cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
	}}
}
