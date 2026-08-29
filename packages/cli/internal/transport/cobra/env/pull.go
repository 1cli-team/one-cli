package envcmd

import (
	"github.com/spf13/cobra"

	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

func newPullCmd(deps Dependencies) *cobra.Command {
	var (
		environment, project, profile string
		force, dryRun                 bool
	)
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "从远端拉取环境变量写入本地 .env（仅 infisical）",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var result *environmentmodule.PullResult
			if err := prompt.Spin(i18n.T("env.pull.running"), func() error {
				value, err := deps.Service.Pull(cmd.Context(), environmentmodule.PullInput{
					Scope: commandScope(cmd), Environment: environment,
					Project: project, Profile: profile, Force: force, DryRun: dryRun,
				})
				result = value
				return err
			}); err != nil {
				return err
			}
			if result != nil {
				output.Emit(pullOutput{result})
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&environment, "env", "", i18n.T("env.flag.environment"))
	cmd.Flags().StringVarP(&project, "project", "p", "", i18n.T("env.flag.pull_project"))
	cmd.Flags().BoolVar(&force, "force", false, i18n.T("env.flag.force"))
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.T("env.flag.dry_run"))
	cmd.Flags().StringVar(&profile, "profile", "", i18n.T("env.flag.profile"))
	markEnvFlagUsage(cmd, "env", "project", "force", "dry-run", "profile")
	i18n.MarkShort(cmd, "env.pull.short")
	return cmd
}

func markEnvFlagUsage(cmd *cobra.Command, names ...string) {
	keys := map[string]string{
		"project": "env.flag.project", "env": "env.flag.environment",
		"profile": "env.flag.profile", "yes": "env.flag.yes",
		"force": "env.flag.force", "dry-run": "env.flag.dry_run",
	}
	if cmd.Name() == "pull" {
		keys["project"] = "env.flag.pull_project"
	}
	for _, name := range names {
		i18n.MarkFlagUsage(cmd, name, keys[name])
	}
}
