package envcmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

func newSetCmd(deps Dependencies) *cobra.Command {
	var (
		project, environment, profile string
		yes                           bool
	)
	cmd := &cobra.Command{
		Use:   "set <KEY[=VALUE]> [VALUE]",
		Short: "写一个环境变量值（dotenv 写到 .env / .env.<env>，infisical 写到对应环境）",
		Long:  i18n.T("env.set.tip"),
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := deps.Service.PlanSet(environmentmodule.PlanSetInput{
				Scope: commandScope(cmd), Environment: environment, Project: project,
			})
			if err != nil {
				return err
			}
			key, value := parseSetArgs(args)
			if !setValueProvided(args) {
				if !output.CanPrompt() {
					return cliErrors.New(cliErrors.ENV_SET_VALUE_REQUIRED, i18n.T("env.value_required"))
				}
				value, err = prompt.Password(i18n.Tf("env.prompt_value", key), func(value string) error {
					if value == "" {
						return fmt.Errorf("%s", i18n.T("env.value_empty"))
					}
					return nil
				})
				if err != nil {
					return err
				}
			}
			if plan.NeedsEnvironmentCreation {
				if err := confirmCreateEnv(plan.Environment, yes); err != nil {
					return err
				}
			}
			plan, err = chooseSetProject(plan, output.CanPrompt())
			if err != nil {
				return err
			}
			input := environmentmodule.SetInput{
				Plan: plan, Key: key, Value: value, Profile: profile, Overwrite: yes,
			}
			result, err := deps.Service.Set(cmd.Context(), input)
			if retry, confirmErr := confirmOverwrite(err, key, yes); confirmErr != nil {
				return confirmErr
			} else if retry {
				input.Overwrite = true
				result, err = deps.Service.Set(cmd.Context(), input)
			}
			if err != nil {
				return err
			}
			output.Emit(setOutput{result})
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", i18n.T("env.flag.project"))
	cmd.Flags().StringVar(&environment, "env", "", i18n.T("env.flag.environment"))
	cmd.Flags().StringVar(&profile, "profile", "", i18n.T("env.flag.profile"))
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, i18n.T("env.flag.yes"))
	markEnvFlagUsage(cmd, "project", "env", "profile", "yes")
	i18n.MarkShort(cmd, "env.set.short")
	i18n.MarkLong(cmd, "env.set.tip")
	return cmd
}

func chooseSetProject(
	plan environmentmodule.SetPlan,
	interactive bool,
) (environmentmodule.SetPlan, error) {
	if len(plan.ProjectChoices) == 0 {
		return plan, nil
	}
	if !interactive {
		return plan.WithProject(""), nil
	}
	options := []prompt.Option[string]{{Label: i18n.T("env.scope_workspace_shared"), Value: ""}}
	for _, project := range plan.ProjectChoices {
		options = append(options, prompt.Option[string]{
			Label: i18n.Tf("env.scope_project_option", project), Value: project,
		})
	}
	project, err := prompt.Select(i18n.T("env.prompt_scope"), options)
	if err != nil {
		return environmentmodule.SetPlan{}, err
	}
	return plan.WithProject(project), nil
}

func parseSetArgs(args []string) (string, string) {
	if len(args) == 2 {
		return args[0], args[1]
	}
	first := args[0]
	if index := strings.IndexByte(first, '='); index > 0 {
		return first[:index], first[index+1:]
	}
	return first, ""
}

func setValueProvided(args []string) bool {
	return len(args) >= 2 || (len(args) == 1 && strings.IndexByte(args[0], '=') > 0)
}

func confirmOverwrite(setErr error, key string, yes bool) (bool, error) {
	if setErr == nil {
		return false, nil
	}
	coded, ok := setErr.(interface{ ErrorCode() string })
	if !ok || coded.ErrorCode() != string(cliErrors.ENV_SET_OVERWRITE_REQUIRED) {
		return false, setErr
	}
	if yes || !output.CanPrompt() {
		return false, setErr
	}
	overwrite, err := prompt.Confirm(i18n.Tf("env.prompt_overwrite", key), false,
		i18n.T("common.overwrite"), i18n.T("common.cancel"))
	if err != nil {
		return false, err
	}
	if !overwrite {
		return false, cliErrors.New(cliErrors.PROMPT_CANCELLED, i18n.T("common.cancelled")).WithExit0()
	}
	return true, nil
}

func confirmCreateEnv(name string, yes bool) error {
	if yes || !output.CanPrompt() {
		return nil
	}
	ok, err := prompt.Confirm(
		fmt.Sprintf("环境 %q 不在 manifest.environments.names 中。要创建并继续吗？", name),
		false, "", "")
	if err != nil {
		return err
	}
	if !ok {
		return cliErrors.New(cliErrors.PROMPT_CANCELLED, "已取消创建新环境。").WithExit0()
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
