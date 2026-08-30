// Package cicmd contributes the optional `one ci` command family.
//
// CI is deliberately not enabled by workspace creation, project addition, or
// deployment. This command is the sole user-facing entry point that writes or
// removes CI workflow files. The generated workflow itself is the state: no CI
// selection is added to one.manifest.json.
package cicmd

import (
	"github.com/spf13/cobra"

	ciapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/ci"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func Commands(service *ciapp.Service) []*cobra.Command {
	return []*cobra.Command{newCICmd(service)}
}

type selectionFlags struct {
	project string
}

type enableFlags struct {
	selectionFlags
	provider string
}

type disableFlags struct {
	selectionFlags
	yes bool
}

func newCICmd(service *ciapp.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ci",
		Long:    i18n.T("ci.tip"),
		Example: "  one ci\n  one ci enable web\n  one ci sync\n  one ci disable web",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStatus(cmd.Context(), service)
		},
	}
	cmd.AddCommand(newEnableCmd(service), newSyncCmd(service), newDisableCmd(service))
	i18n.MarkShort(cmd, "ci.short")
	i18n.MarkLong(cmd, "ci.tip")
	return cmd
}

func newEnableCmd(service *ciapp.Service) *cobra.Command {
	flags := &enableFlags{}
	cmd := &cobra.Command{
		Use:     "enable [project]",
		Long:    i18n.T("ci.enable.tip"),
		Example: "  one ci enable\n  one ci enable web\n  one ci enable web --provider ci/github-actions",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selector, err := resolveSelector(args, flags.project)
			if err != nil {
				return err
			}
			return runEnable(cmd.Context(), service, selector, flags.provider)
		},
	}
	cmd.Flags().StringVarP(&flags.project, "project", "p", "", i18n.T("ci.flag.project"))
	cmd.Flags().StringVar(&flags.provider, "provider", "", i18n.T("ci.flag.provider"))
	i18n.MarkFlagUsage(cmd, "project", "ci.flag.project")
	i18n.MarkFlagUsage(cmd, "provider", "ci.flag.provider")
	helpui.MarkAdvanced(cmd, "project", "provider")
	i18n.MarkShort(cmd, "ci.enable.short")
	i18n.MarkLong(cmd, "ci.enable.tip")
	return cmd
}

func newSyncCmd(service *ciapp.Service) *cobra.Command {
	flags := &selectionFlags{}
	cmd := &cobra.Command{
		Use:     "sync [project]",
		Long:    i18n.T("ci.sync.tip"),
		Example: "  one ci sync\n  one ci sync web",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selector, err := resolveSelector(args, flags.project)
			if err != nil {
				return err
			}
			return runSync(cmd.Context(), service, selector)
		},
	}
	cmd.Flags().StringVarP(&flags.project, "project", "p", "", i18n.T("ci.flag.project"))
	i18n.MarkFlagUsage(cmd, "project", "ci.flag.project")
	helpui.MarkAdvanced(cmd, "project")
	i18n.MarkShort(cmd, "ci.sync.short")
	i18n.MarkLong(cmd, "ci.sync.tip")
	return cmd
}

func newDisableCmd(service *ciapp.Service) *cobra.Command {
	flags := &disableFlags{}
	cmd := &cobra.Command{
		Use:     "disable [project]",
		Long:    i18n.T("ci.disable.tip"),
		Example: "  one ci disable web\n  one ci disable --yes",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selector, err := resolveSelector(args, flags.project)
			if err != nil {
				return err
			}
			return runDisable(cmd.Context(), service, selector, flags.yes)
		},
	}
	cmd.Flags().StringVarP(&flags.project, "project", "p", "", i18n.T("ci.flag.project"))
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, i18n.T("ci.flag.yes"))
	i18n.MarkFlagUsage(cmd, "project", "ci.flag.project")
	i18n.MarkFlagUsage(cmd, "yes", "ci.flag.yes")
	helpui.MarkAdvanced(cmd, "project")
	i18n.MarkShort(cmd, "ci.disable.short")
	i18n.MarkLong(cmd, "ci.disable.tip")
	return cmd
}
