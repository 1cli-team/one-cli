package runcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/devservice"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/tasks"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
)

// The private worker isolates Cobra's process-wide state. stdout is a bounded
// lifecycle protocol; stderr contains the task pipeline's original console.
func serviceWorker(service tasks.Service) *cobra.Command {
	var project, environment string
	cmd := &cobra.Command{Use: "__service", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, stop := process.SignalContext(cmd.Context())
		defer stop()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		// This private pipe is owned by the Dashboard, never forwarded to the
		// dev task. EOF cancels preparation/execution even on Windows or if
		// the Dashboard disappears without sending a signal.
		go func() { _, _ = io.Copy(io.Discard, cmd.InOrStdin()); cancel() }()
		work := func() error {
			w, err := execution.ResolveWorkspace(ctx)
			if err != nil {
				return err
			}
			opts := tasks.Options{Name: "dev", Projects: []string{project}, Environment: environment, Jobs: 1, UI: "stream", Cache: "off"}
			if project == "" {
				return i18n.Errorf("tasks.context_invalid")
			}
			fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("tasks.preparing"))
			plan, err := service.Plan(ctx, w, opts)
			if err != nil {
				return err
			}
			for _, task := range plan.Tasks {
				if task.Raw || task.Interactive {
					return i18n.Errorf("devservice.interactive")
				}
			}
			service.OnStarted = func() { _ = json.NewEncoder(cmd.OutOrStdout()).Encode(devservice.Event{Status: "running"}) }
			_, err = service.Execute(ctx, w, plan, opts, nil, cmd.ErrOrStderr(), cmd.ErrOrStderr())
			return err
		}
		if err := work(); err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), err)
			return &process.ExitStatus{Code: process.ExitCode(err)}
		}
		return nil
	}}
	cmd.Flags().StringVar(&project, "project", "", "")
	cmd.Flags().StringVar(&environment, "env", "", "")
	return cmd
}
