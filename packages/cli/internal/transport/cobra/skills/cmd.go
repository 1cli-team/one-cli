package skillscmd

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/skills"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/prompt"
)

type selectionPrompt func(context.Context, string, string, []prompt.Option[string], []string) ([]string, error)

func ConfigureCommand(installer skillsmodule.Installer) *cobra.Command {
	return configureCommand(installer, output.CanPrompt, defaultRecommendationPrompts())
}

func configureCommand(installer skillsmodule.Installer, canPrompt func() bool, prompts recommendationPrompts) *cobra.Command {
	var dryRun, recommend, yes, global, copySkills bool
	var agents []string
	var names []string
	cmd := &cobra.Command{
		Use: "skills", Args: i18n.NoArgs,
		Example: "  one init skills --dry-run -o json\n  one init skills\n  one init skills --skill one-go\n  one init skills --install-recommend\n  one init skills --install-recommend --dry-run -o json\n  one init skills --install-recommend --yes\n  one skills update --project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if recommend && cmd.Flags().Changed("skill") {
				return i18n.Errorf("skills.recommend.conflict")
			}
			if !recommend {
				for _, name := range []string{"yes", "agent", "global", "copy"} {
					if cmd.Flags().Changed(name) {
						return i18n.Errorf("skills.recommend.flag_requires_recommend", name)
					}
				}
			}
			if recommend && !dryRun && !yes && !canPrompt() {
				return i18n.Errorf("skills.recommend.noninteractive")
			}
			workspace, err := execution.ResolveWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			if recommend {
				options := skillsmodule.InstallOptions{Scope: "project", Mode: "symlink", Agents: agents}
				if global {
					options.Scope = "global"
				}
				if copySkills {
					options.Mode = "copy"
				}
				return runRecommended(cmd, installer, workspace.Root(), options, yes, dryRun, prompts)
			}
			result, err := (skillsmodule.Initializer{Hash: installer.Hash, Timeout: installer.Timeout}).Init(cmd.Context(), workspace.Root(), names, dryRun)
			if err != nil {
				return err
			}
			output.Emit(result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.T("skills.init_dry_run"))
	cmd.Flags().StringSliceVar(&names, "skill", nil, i18n.T("skills.init_names"))
	cmd.Flags().BoolVar(&recommend, "install-recommend", false, i18n.T("skills.recommend.flag"))
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, i18n.T("skills.recommend.yes"))
	cmd.Flags().StringSliceVar(&agents, "agent", nil, i18n.T("skills.batch.agent_flag"))
	cmd.Flags().BoolVar(&global, "global", false, i18n.T("skills.batch.global_flag"))
	cmd.Flags().BoolVar(&copySkills, "copy", false, i18n.T("skills.batch.copy_flag"))
	i18n.MarkFlagUsage(cmd, "dry-run", "skills.init_dry_run")
	i18n.MarkFlagUsage(cmd, "skill", "skills.init_names")
	i18n.MarkFlagUsage(cmd, "install-recommend", "skills.recommend.flag")
	i18n.MarkFlagUsage(cmd, "yes", "skills.recommend.yes")
	i18n.MarkFlagUsage(cmd, "agent", "skills.batch.agent_flag")
	i18n.MarkFlagUsage(cmd, "global", "skills.batch.global_flag")
	i18n.MarkFlagUsage(cmd, "copy", "skills.batch.copy_flag")
	i18n.MarkShort(cmd, "skills.init_short")
	i18n.MarkLong(cmd, "skills.init_tip")
	return cmd
}

func recommendationOptions(result *skillsmodule.RecommendationResult) ([]prompt.Option[string], []string, string) {
	var options []prompt.Option[string]
	var selected, installed, blocked []string
	for _, skill := range result.Skills {
		switch skill.Status {
		case "installed":
			installed = append(installed, skill.Name)
		case "blocked":
			blocked = append(blocked, skill.Name+": "+skill.Message)
		default:
			options = append(options, prompt.Option[string]{Label: skill.Name, Value: skill.Name})
			if skill.Selected {
				selected = append(selected, skill.Name)
			}
		}
	}
	description := i18n.T("skills.recommend.selection_hint")
	if len(installed) > 0 {
		description += "\n" + i18n.Tf("skills.recommend.installed", strings.Join(installed, ", "))
	}
	if len(blocked) > 0 {
		description += "\n" + strings.Join(blocked, "\n")
	}
	if len(result.Warnings) > 0 {
		description += "\n" + strings.Join(result.Warnings, "\n")
	}
	return options, selected, description
}

func Commands(run skillsmodule.RunFunc) []*cobra.Command {
	cmd := &cobra.Command{
		Use: "skills [args...]", DisableFlagParsing: true,
		Example: "  one skills find react\n  one skills add owner/repo --skill skill-name\n  one skills list\n  one skills update",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			return run(cmd.Context(), dir, args, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	i18n.MarkShort(cmd, "skills.short")
	i18n.MarkLong(cmd, "skills.tip")
	return []*cobra.Command{cmd}
}
