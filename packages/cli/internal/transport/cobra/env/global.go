package envcmd

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	remote "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

func configureGlobal(parent *cobra.Command, deps Dependencies) {
	parent.PersistentFlags().Bool("global", false, i18n.T("env.flag.global"))
	i18n.MarkFlagUsage(parent, "global", "env.flag.global")
	parent.PersistentFlags().String("path", "/", i18n.T("env.flag.path"))
	i18n.MarkFlagUsage(parent, "path", "env.flag.path")
	parent.Flags().String("env", "", i18n.T("env.flag.environment_name"))
	i18n.MarkFlagUsage(parent, "env", "env.flag.environment_name")
	bind := &cobra.Command{Use: "bind", Short: i18n.T("env.bind.short"), Args: i18n.NoArgs}
	i18n.MarkShort(bind, "env.bind.short")
	bind.Flags().String("project-id", "", i18n.T("env.flag.project_id"))
	i18n.MarkFlagUsage(bind, "project-id", "env.flag.project_id")
	bind.Flags().String("env", "", i18n.T("env.flag.default_environment"))
	i18n.MarkFlagUsage(bind, "env", "env.flag.default_environment")
	bind.RunE = func(c *cobra.Command, _ []string) error {
		global, _ := c.Flags().GetBool("global")
		if !global {
			return i18n.Errorf("env.bind.global_required")
		}
		id, _ := c.Flags().GetString("project-id")
		env, _ := c.Flags().GetString("env")
		if id == "" && output.CanPrompt() {
			ps, e := remote.Projects(c.Context())
			if e != nil {
				return e
			}
			if len(ps) == 0 {
				return i18n.Errorf("env.bind.no_projects")
			}
			options := []prompt.Option[string]{}
			for _, p := range ps {
				options = append(options, prompt.Option[string]{Label: p.Name, Value: p.ID})
			}
			id, e = prompt.Select(i18n.T("env.bind.select_project"), options)
			if e != nil {
				return e
			}
		}
		if env == "" && output.CanPrompt() {
			p, e := remote.Project(c.Context(), id)
			if e != nil {
				return e
			}
			options := []prompt.Option[string]{}
			for _, v := range p.Environments {
				options = append(options, prompt.Option[string]{Label: v.Name + " (" + v.Slug + ")", Value: v.Slug})
			}
			if len(options) == 0 {
				return i18n.Errorf("env.bind.no_environments")
			}
			env, e = prompt.Select(i18n.T("env.bind.select_environment"), options)
			if e != nil {
				return e
			}
		}
		l, e := remote.BindGlobal(c.Context(), id, env)
		if e != nil {
			return e
		}
		output.Emit(l)
		return nil
	}
	unset := &cobra.Command{Use: "unset <KEY>", Short: i18n.T("env.unset.short"), Args: i18n.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		env, _ := c.Flags().GetString("env")
		project, _ := c.Flags().GetString("project")
		r, e := deps.Service.Delete(c.Context(), environmentmodule.DeleteInput{Scope: commandScope(c), Environment: env, Project: project, Key: args[0]})
		if e != nil {
			return e
		}
		output.Emit(r)
		return nil
	}}
	i18n.MarkShort(unset, "env.unset.short")
	unset.Flags().String("env", "", i18n.T("env.flag.environment_name"))
	i18n.MarkFlagUsage(unset, "env", "env.flag.environment_name")
	unset.Flags().StringP("project", "p", "", i18n.T("env.flag.project_selector"))
	i18n.MarkFlagUsage(unset, "project", "env.flag.project_selector")
	parent.AddCommand(unset, bind)
	for _, c := range append([]*cobra.Command{parent}, parent.Commands()...) {
		if c == bind {
			continue
		}
		if c.Name() == "get" {
			c.Flags().Bool("reveal", false, i18n.T("env.flag.reveal"))
			i18n.MarkFlagUsage(c, "reveal", "env.flag.reveal")
		}
		if c.Name() == "set" {
			c.Flags().Bool("stdin", false, i18n.T("env.flag.stdin"))
			i18n.MarkFlagUsage(c, "stdin", "env.flag.stdin")
		}
		original := c.RunE
		if original == nil {
			continue
		}
		c.RunE = func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			if cmd.Name() == "get" {
				reveal, _ := cmd.Flags().GetBool("reveal")
				if !reveal {
					return i18n.Errorf("env.reveal_required")
				}
			}
			if !global {
				if cmd.Flags().Changed("path") {
					return i18n.Errorf("env.path_global_required")
				}
				return original(cmd, args)
			}
			env, _ := cmd.Flags().GetString("env")
			folder, _ := cmd.Flags().GetString("path")
			if cmd.Flags().Changed("project") {
				return i18n.Errorf("env.global_project_conflict")
			}
			var result any
			var e error
			switch cmd.Name() {
			case "env":
				l, err := remote.LoadGlobalLocation()
				if err != nil {
					return err
				}
				if l == nil {
					output.Emit(map[string]any{"location": nil, "commands": []string{"one env bind --global"}})
					return nil
				}
				l, environments, err := remote.GlobalSummary(cmd.Context())
				if err != nil {
					return err
				}
				result = map[string]any{"location": l, "environments": environments, "commands": []string{"one env list --global --path /", "one exec --global --path /folder --env ENV -- command"}}
			case "list":
				result, e = remote.ListGlobal(cmd.Context(), env, folder)
			case "get", "unset":
				result, e = remote.GlobalSecret(cmd.Context(), cmd.Name(), env, folder, args[0], "")
			case "set":
				key, value := parseSetArgs(args)
				stdin, _ := cmd.Flags().GetBool("stdin")
				if stdin {
					if setValueProvided(args) {
						return i18n.Errorf("env.stdin_value_conflict")
					}
					b, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), (1<<20)+1))
					if err != nil {
						return err
					}
					if len(b) > 1<<20 {
						return i18n.Errorf("env.value_too_large")
					}
					value = strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r")
				} else if !setValueProvided(args) {
					if !output.CanPrompt() {
						return i18n.Errorf("env.stdin_required")
					}
					value, e = prompt.Password(i18n.T("env.prompt_secret"), nil)
					if e != nil {
						return e
					}
				}
				listing, err := remote.ListGlobal(cmd.Context(), env, folder)
				if err != nil {
					return err
				}
				action := "create"
				yes, _ := cmd.Flags().GetBool("yes")
				for _, v := range listing.Variables {
					if v.Key == key {
						if !yes {
							return i18n.Errorf("env.overwrite_required")
						}
						action = "update"
						break
					}
				}
				result, e = remote.GlobalSecret(cmd.Context(), action, env, folder, key, value)
			default:
				return i18n.Errorf("env.global_unsupported", cmd.Name())
			}
			if e != nil {
				return e
			}
			output.Emit(result)
			return nil
		}
	}
}
