// Package createcmd contributes `one create` to the explicit root command.
// It scaffolds a new workspace (one.manifest.json + folder skeleton),
// applies the default workspace capabilities (local .env | one dev).
// Projects, CI, and deployment targets are intentionally deferred.
package createcmd

import (
	"github.com/spf13/cobra"

	creationmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/creation"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

type Dependencies struct {
	Creation *creationmodule.Service
}

func Commands(deps Dependencies) []*cobra.Command { return buildContributions(deps) }

func buildContributions(deps Dependencies) []*cobra.Command {
	return []*cobra.Command{newCreateCmd(deps)}
}

// workspaceDefaultEnables are the default backend ids stamped into
// the manifest when scaffolding a new workspace. env defaults to dotenv
// (lowest-friction), while dev uses the built-in process runner. CI is not
// enabled implicitly. Advanced automation may override the env source with
// --env-provider.
var workspaceDefaultEnables = []string{
	"env/dotenv",
	"dev/process",
}

// canonicalDomainOrder is the canonical iteration order for emitting
// the list of enabled backends in the create envelope. Mirrors the
// legacy ordering.
var canonicalDomainOrder = []string{"container", "dev", "deploy", "ci", "env"}

type createFlags struct {
	name         string
	yes          bool
	envProvider  string
	preset       string
	projectNames string
}

func newCreateCmd(deps Dependencies) *cobra.Command {
	flags := &createFlags{}
	cmd := &cobra.Command{
		Use:     "create [dir]",
		Long:    i18n.T("create.tip"),
		Example: "  one create demo\n  one create . --name demo\n  one create demo --yes",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := ""
			if len(args) > 0 {
				dir = args[0]
			}
			return runCreate(deps, cmd, dir, flags)
		},
	}
	cmd.Flags().StringVarP(&flags.name, "name", "n", "", i18n.T("create.flag.name"))
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, i18n.T("create.flag.yes"))
	cmd.Flags().StringVar(&flags.envProvider, "env-provider", "",
		i18n.T("create.flag.env_provider"))
	cmd.Flags().StringVar(&flags.preset, "preset", "",
		i18n.T("create.flag.preset"))
	cmd.Flags().StringVar(&flags.projectNames, "project-names", "",
		i18n.T("create.flag.project_names"))
	i18n.MarkFlagUsage(cmd, "name", "create.flag.name")
	i18n.MarkFlagUsage(cmd, "yes", "create.flag.yes")
	i18n.MarkFlagUsage(cmd, "env-provider", "create.flag.env_provider")
	i18n.MarkFlagUsage(cmd, "preset", "create.flag.preset")
	i18n.MarkFlagUsage(cmd, "project-names", "create.flag.project_names")
	helpui.MarkAdvanced(cmd, "env-provider", "preset", "project-names")
	i18n.MarkShort(cmd, "create.short")
	i18n.MarkLong(cmd, "create.tip")
	return cmd
}
