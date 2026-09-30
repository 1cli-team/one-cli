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
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

func newSetCmd(deps Dependencies) *cobra.Command {
	var (
		project, environment string
		yes                  bool
	)
	cmd := &cobra.Command{
		Use:   "set <KEY[=VALUE]> [VALUE]",
		Short: i18n.T("env.set.short"),
		Long:  i18n.T("env.set.tip"),
		Args:  validateSetArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			value, provided, err := readSetValue(cmd, args)
			if err != nil {
				return err
			}
			plan, err := deps.Service.PlanSet(environmentmodule.PlanSetInput{
				Scope: commandScope(cmd), Environment: environment, Project: project,
			})
			if err != nil {
				return err
			}
			key, _ := parseSetArgs(args)
			if !provided {
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
				Plan: plan, Key: key, Value: value, Overwrite: yes,
			}
			var result *environmentmodule.SetResult
			progress := i18n.T("env.saving")
			if plan.NeedsBinding {
				progress = i18n.T("env.initializing")
			}
			err = prompt.Spin(progress, func() error {
				var setErr error
				result, setErr = deps.Service.Set(cmd.Context(), input)
				return setErr
			})
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
	i18n.MarkFlagUsage(cmd, "project", "env.flag.project")
	cmd.Flags().StringVar(&environment, "env", "", i18n.T("env.flag.environment"))
	i18n.MarkFlagUsage(cmd, "env", "env.flag.environment")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, i18n.T("env.flag.yes"))
	i18n.MarkFlagUsage(cmd, "yes", "env.flag.yes")
	cmd.Flags().Bool("stdin", false, i18n.T("env.flag.stdin"))
	i18n.MarkFlagUsage(cmd, "stdin", "env.flag.stdin")
	markEnvFlagUsage(cmd, "project", "env", "yes")
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

// Reject ambiguous assignment syntax before prompts or remote access. Echoing
// an invalid key here could expose a value accidentally supplied as KEY=VALUE.
func validateSetArgs(cmd *cobra.Command, args []string) error {
	if err := i18n.RangeArgs(1, 2)(cmd, args); err != nil {
		return err
	}
	if len(args) == 2 && strings.Contains(args[0], "=") {
		return i18n.Errorf("env.set_syntax_conflict")
	}
	key, _ := parseSetArgs(args)
	return secrets.AssertValidKey(key)
}

func parseSetArgs(args []string) (string, string) {
	if len(args) == 2 {
		return args[0], args[1]
	}
	first := args[0]
	if index := strings.IndexByte(first, '='); index >= 0 {
		return first[:index], first[index+1:]
	}
	return first, ""
}

func setValueProvided(args []string) bool {
	return len(args) >= 2 || (len(args) == 1 && strings.IndexByte(args[0], '=') >= 0)
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
	message := i18n.Tf("env.prompt_overwrite", key)
	if detail, ok := setErr.(*output.Error); ok {
		message = detail.Message + "\n" + message
	}
	overwrite, err := prompt.Confirm(message, false,
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
		i18n.Tf("env.create_environment_confirm", name),
		false, "", "")
	if err != nil {
		return err
	}
	if !ok {
		return cliErrors.New(cliErrors.PROMPT_CANCELLED, i18n.T("env.create_environment_cancelled")).WithExit0()
	}
	return nil
}
