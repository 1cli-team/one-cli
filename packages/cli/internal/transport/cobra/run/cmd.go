// Package runcmd exposes named workspace tasks through mise.
package runcmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/tasks"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

func Commands(loaders *secrets.Registry, provider runtimeport.Provider) []*cobra.Command {
	service := tasks.Service{Provider: provider, Loaders: loaders}
	return []*cobra.Command{command(service), internal(false), internal(true), serviceWorker(service)}
}
func command(service tasks.Service) *cobra.Command {
	opts := tasks.Options{Jobs: 1, Cache: "local-only", UI: "auto"}
	var list, dry bool
	cmd := &cobra.Command{Use: "run [task]", Args: cobra.ArbitraryArgs, Example: "  one run\n  one run build -p web\n  one run test -p api\n  one run build --dry-run", RunE: func(cmd *cobra.Command, args []string) (resultErr error) {
		dash := cmd.ArgsLenAtDash()
		if dash >= 0 {
			opts.Arguments = args[dash:]
			args = args[:dash]
		}
		if len(args) > 1 {
			return i18n.Errorf("tasks.one_name")
		}
		if len(args) == 1 {
			opts.Name = args[0]
		}
		w, err := execution.ResolveWorkspace(cmd.Context())
		if err != nil {
			return err
		}
		if list || opts.Name == "" {
			if len(opts.Arguments) > 0 {
				return i18n.Errorf("tasks.one_name")
			}
			catalog, _, err := tasks.Catalog(w)
			if err != nil {
				return err
			}
			selected := map[string]bool{}
			if len(opts.Projects) > 0 {
				names, err := w.SelectProjects(opts.Projects, "")
				if err != nil {
					return err
				}
				for _, name := range names {
					selected[name] = true
				}
			}
			visible := []tasks.Task{}
			for _, task := range catalog {
				if len(selected) == 0 || selected[task.Project] {
					visible = append(visible, task)
				}
			}
			if output.IsStructured() {
				output.Emit(tasks.MarshalCatalog(visible))
			} else {
				tasks.RenderCatalog(cmd.OutOrStdout(), visible)
			}
			return nil
		}
		if err := tasks.ValidateOptions(opts); err != nil {
			return err
		}
		ctx, stop := process.SignalContext(cmd.Context())
		defer stop()
		defer func() {
			if ctx.Err() != nil {
				resultErr = &process.ExitStatus{Code: process.ExitCode(context.Cause(ctx))}
			}
		}()
		opts.JobsExplicit = cmd.Flags().Changed("concurrency")
		var plan *tasks.Plan
		if dry {
			plan, err = tasks.NewPlan(w, opts)
		} else {
			plan, err = service.Plan(ctx, w, opts)
		}
		if err != nil {
			return err
		}
		if dry {
			output.Emit(plan)
			return nil
		}
		result, err := service.Execute(ctx, w, plan, opts, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if result != nil {
			output.Emit(result)
		}
		return err
	}}
	cmd.Flags().StringArrayVarP(&opts.Projects, "project", "p", nil, "")
	cmd.Flags().StringVar(&opts.Environment, "env", "", "")
	cmd.Flags().StringVar(&opts.Cache, "cache", "local-only", "")
	cmd.Flags().StringVar(&opts.UI, "ui", "auto", "")
	cmd.Flags().IntVar(&opts.Jobs, "concurrency", 1, "")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "")
	cmd.Flags().BoolVar(&list, "list", false, "")
	for _, name := range []string{"project", "env", "cache", "ui", "concurrency", "force", "dry-run", "list"} {
		i18n.MarkFlagUsage(cmd, name, "tasks.flag."+strings.ReplaceAll(name, "-", "_"))
	}
	i18n.MarkShort(cmd, "tasks.run.short")
	i18n.MarkLong(cmd, "tasks.run.long")
	return cmd
}
func internal(fingerprint bool) *cobra.Command {
	var project, operation string
	name := "__task"
	if fingerprint {
		name = "__task-input"
	}
	cmd := &cobra.Command{Use: name, Hidden: true, RunE: func(cmd *cobra.Command, args []string) error {
		w, err := execution.ResolveWorkspace(cmd.Context())
		if err != nil {
			return err
		}
		if project == "" || operation == "" {
			return i18n.Errorf("tasks.context_invalid")
		}
		if fingerprint {
			value, err := tasks.InputFingerprint(cmd.Context(), w, project, operation)
			if err == nil {
				fmt.Fprintln(cmd.OutOrStdout(), value)
			}
			return err
		}
		if cmd.ArgsLenAtDash() != 0 {
			return i18n.Errorf("exec.internal_command_required")
		}
		return tasks.ExecuteLeaf(cmd.Context(), w, project, operation, args, os.Stdin, os.Stdout, os.Stderr)
	}}
	cmd.Flags().StringVar(&project, "project", "", "")
	cmd.Flags().StringVar(&operation, "task", "", "")
	return cmd
}
