package envcmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	remote "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

func configureGlobal(parent *cobra.Command, deps Dependencies) {
	parent.PersistentFlags().Bool("global", false, "管理 Infisical 共享凭据，可在工作区之外使用")
	parent.PersistentFlags().String("path", "/", "共享凭据目录（仅当前层，不递归）")
	parent.Flags().String("env", "", "环境名")
	bind := &cobra.Command{Use: "bind", Short: "选择共享凭据的存放项目和默认环境", Args: cobra.NoArgs}
	bind.Flags().String("project-id", "", "已有 Infisical 项目 ID")
	bind.Flags().String("env", "", "默认环境")
	bind.RunE = func(c *cobra.Command, _ []string) error {
		global, _ := c.Flags().GetBool("global")
		if !global {
			return fmt.Errorf("请使用 one env bind --global")
		}
		id, _ := c.Flags().GetString("project-id")
		env, _ := c.Flags().GetString("env")
		if id == "" && output.CanPrompt() {
			ps, e := remote.Projects(c.Context())
			if e != nil {
				return e
			}
			if len(ps) == 0 {
				return fmt.Errorf("没有可用项目，请先在 Infisical 中创建项目")
			}
			options := []prompt.Option[string]{}
			for _, p := range ps {
				options = append(options, prompt.Option[string]{Label: p.Name, Value: p.ID})
			}
			id, e = prompt.Select("选择存放共享凭据的项目", options)
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
				return fmt.Errorf("项目没有可用环境")
			}
			env, e = prompt.Select("默认浏览环境", options)
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
	unset := &cobra.Command{Use: "unset <KEY>", Short: "删除一个 Infisical 环境变量", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		env, _ := c.Flags().GetString("env")
		project, _ := c.Flags().GetString("project")
		r, e := deps.Service.Delete(c.Context(), environmentmodule.DeleteInput{Scope: commandScope(c), Environment: env, Project: project, Key: args[0]})
		if e != nil {
			return e
		}
		output.Emit(r)
		return nil
	}}
	unset.Flags().String("env", "", "环境名")
	unset.Flags().StringP("project", "p", "", "项目名或路径")
	parent.AddCommand(unset, bind)
	for _, c := range append([]*cobra.Command{parent}, parent.Commands()...) {
		if c == bind {
			continue
		}
		if c.Name() == "get" {
			c.Flags().Bool("reveal", false, "显式输出明文；通常请通过 one run 使用变量")
		}
		if c.Name() == "set" {
			c.Flags().Bool("stdin", false, "从标准输入读取值，避免写入命令历史（共享凭据）")
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
					return fmt.Errorf("读取明文必须指定 --reveal；执行任务请优先使用 one run")
				}
			}
			if !global {
				if cmd.Flags().Changed("path") {
					return fmt.Errorf("--path 仅用于 --global")
				}
				return original(cmd, args)
			}
			env, _ := cmd.Flags().GetString("env")
			folder, _ := cmd.Flags().GetString("path")
			if cmd.Flags().Changed("project") {
				return fmt.Errorf("--global 不能与 --project 同时使用")
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
				result = map[string]any{"location": l, "environments": environments, "commands": []string{"one env list --global --path /", "one run --global --path /folder --env ENV -- command"}}
			case "list":
				result, e = remote.ListGlobal(cmd.Context(), env, folder)
			case "get", "unset":
				result, e = remote.GlobalSecret(cmd.Context(), cmd.Name(), env, folder, args[0], "")
			case "set":
				key, value := parseSetArgs(args)
				stdin, _ := cmd.Flags().GetBool("stdin")
				if stdin {
					if setValueProvided(args) {
						return fmt.Errorf("--stdin 不能同时提供参数值")
					}
					b, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), (1<<20)+1))
					if err != nil {
						return err
					}
					if len(b) > 1<<20 {
						return fmt.Errorf("变量值过大")
					}
					value = strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r")
				} else if !setValueProvided(args) {
					if !output.CanPrompt() {
						return fmt.Errorf("请通过 --stdin 提供值")
					}
					value, e = prompt.Password("变量值", nil)
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
							return fmt.Errorf("变量已存在；覆盖请指定 --yes")
						}
						action = "update"
						break
					}
				}
				result, e = remote.GlobalSecret(cmd.Context(), action, env, folder, key, value)
			default:
				return fmt.Errorf("one env %s 不支持 --global", cmd.Name())
			}
			if e != nil {
				return e
			}
			output.Emit(result)
			return nil
		}
	}
}
