package envcmd

import (
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
	bind := &cobra.Command{
		Use:     "bind",
		Short:   i18n.T("env.bind.short"),
		Long:    i18n.T("env.bind.tip"),
		Example: "  one env bind\n  one env bind --create\n  one env bind --project-id PROJECT_ID\n  one env bind --global",
		Args:    i18n.NoArgs,
	}
	i18n.MarkShort(bind, "env.bind.short")
	i18n.MarkLong(bind, "env.bind.tip")
	bind.Flags().String("project-id", "", i18n.T("env.flag.project_id"))
	i18n.MarkFlagUsage(bind, "project-id", "env.flag.project_id")
	bind.Flags().Bool("create", false, i18n.T("env.bind.flag.create"))
	i18n.MarkFlagUsage(bind, "create", "env.bind.flag.create")
	bind.Flags().String("env", "", i18n.T("env.flag.default_environment"))
	i18n.MarkFlagUsage(bind, "env", "env.flag.default_environment")
	bind.RunE = func(c *cobra.Command, _ []string) error {
		global, _ := c.Flags().GetBool("global")
		id, _ := c.Flags().GetString("project-id")
		env, _ := c.Flags().GetString("env")
		create, _ := c.Flags().GetBool("create")
		if create && id != "" {
			return i18n.Errorf("env.bind.create_conflict")
		}
		if global && create {
			return i18n.Errorf("env.bind.create_workspace_only")
		}
		if !global {
			if c.Flags().Changed("path") {
				return i18n.Errorf("env.path_global_required")
			}
			if c.Flags().Changed("env") {
				return i18n.Errorf("env.bind.workspace_env_fixed")
			}
			return runWorkspaceBind(c, deps, id, create)
		}
		if id == "" {
			location, err := remote.BindDefaultGlobal(c.Context(), env)
			if err != nil {
				return err
			}
			output.Emit(bindOutput{location})
			return nil
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
		output.Emit(bindOutput{l})
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
		original := c.RunE
		if original == nil {
			continue
		}
		c.RunE = func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
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
			case "unset":
				result, e = remote.GlobalSecret(cmd.Context(), cmd.Name(), env, folder, args[0], "")
			case "set":
				key, _ := parseSetArgs(args)
				value, provided, err := readSetValue(cmd, args)
				if err != nil {
					return err
				}
				if !provided {
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
