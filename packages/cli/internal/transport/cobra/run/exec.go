package runcmd

import (
	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

// __exec is the terminal leaf of mise execution. It must never enter mise again.
func newExecCmd(loaders *secrets.Registry) *cobra.Command {
	flags := &runFlags{prepared: true}
	var protocol int
	var operation string
	cmd := &cobra.Command{
		Use: "__exec", Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if protocol != 1 {
				return i18n.Errorf("run.protocol_unsupported", protocol)
			}
			if flags.project == "" {
				return i18n.Errorf("run.internal_project_required")
			}
			if operation != "" {
				if len(args) > 0 {
					return i18n.Errorf("run.operation_arguments")
				}
				w, err := execution.ResolveWorkspace(cmd.Context())
				if err != nil {
					return err
				}
				args, err = execution.OperationArgs(w, flags.project, operation)
				if err != nil {
					return err
				}
			} else if cmd.ArgsLenAtDash() != 0 || len(args) == 0 {
				return i18n.Errorf("run.internal_command_required")
			}
			return runRun(cmd.Context(), loaders, flags, args)
		},
	}
	cmd.Flags().IntVar(&protocol, "protocol", 0, "Execution protocol version")
	cmd.Flags().StringVar(&flags.project, "project", "", "Project selector")
	cmd.Flags().StringVar(&flags.envName, "env", "", "Environment")
	cmd.Flags().StringVar(&flags.envProvider, "env-provider", "", "Environment provider")
	cmd.Flags().StringVar(&operation, "operation", "", "Project operation")
	return cmd
}
