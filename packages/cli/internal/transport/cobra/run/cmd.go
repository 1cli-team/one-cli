// Package runcmd exposes named workspace tasks through mise.
package runcmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/tasks"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type DevRunner func(*cobra.Command, tasks.Options) error

func Commands(loaders *secrets.Registry, provider runtimeport.Provider, dev DevRunner) []*cobra.Command {
	service := tasks.Service{Provider: provider, Loaders: loaders}
	return []*cobra.Command{command("run", service, dev), command("build", service, dev), internal(false), internal(true)}
}
func command(kind string, service tasks.Service, dev DevRunner) *cobra.Command {
	opts := tasks.Options{Jobs: 1, Cache: "local-only", UI: "auto"}
	var list, dry bool
	use := "run [task]"
	if kind == "build" {
		use = "build [projects...]"
	}
	cmd := &cobra.Command{Use: use, Args: cobra.ArbitraryArgs, Example: "  one run\n  one run build -p web\n  one run test -p api\n  one run build --dry-run", RunE: func(cmd *cobra.Command, args []string) error {
		dash := cmd.ArgsLenAtDash()
		if dash >= 0 {
			opts.Arguments = args[dash:]
			args = args[:dash]
		}
		if kind == "build" {
			opts.Name = "build"
			opts.Projects = append(opts.Projects, args...)
		} else {
			if len(args) > 1 {
				return i18n.Errorf("tasks.one_name")
			}
			if len(args) == 1 {
				opts.Name = args[0]
			}
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
		if opts.Name == "dev" && dev != nil {
			if len(opts.Arguments) > 0 {
				return i18n.Errorf("tasks.dev_arguments")
			}
			if dry {
				_ = cmd.Flags().Set("dry-run", "true")
			}
			return dev(cmd, opts)
		}
		for _, arg := range opts.Arguments {
			if arg == "--watch" || arg == "--watch=true" || arg == "--watchAll" || arg == "--watchAll=true" {
				opts.Cache = "off"
				if opts.UI == "auto" {
					opts.UI = "raw"
				}
			}
		}
		if err := tasks.ValidateOptions(opts); err != nil {
			return err
		}
		var plan *tasks.Plan
		if dry {
			plan, err = tasks.NewPlan(w, opts)
		} else {
			plan, err = service.Plan(cmd.Context(), w, opts)
		}
		if err != nil {
			return err
		}
		if dry {
			output.Emit(plan)
			return nil
		}
		result, err := service.Execute(cmd.Context(), w, plan, opts, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		if result != nil {
			output.Emit(result)
		}
		return err
	}}
	if kind == "build" {
		cmd.Example = "  one build\n  one build web api\n  one build web --dry-run"
	}
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
	i18n.MarkShort(cmd, "tasks."+kind+".short")
	i18n.MarkLong(cmd, "tasks."+kind+".long")
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
