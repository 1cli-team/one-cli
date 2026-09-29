// Package createcmd contributes `one create` to the explicit root command.
// It scaffolds a new workspace (one.manifest.toml + folder skeleton),
// scaffolds workspace tooling and development tasks without a remote binding.
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

// workspaceDefaultEnables records the supported environment source for creation output.
var workspaceDefaultEnables = []string{
	"env/infisical",
	"dev/process",
}

// canonicalDomainOrder is the canonical iteration order for emitting
// the list of enabled backends in the create envelope. Mirrors the
// legacy ordering.
var canonicalDomainOrder = []string{"dev", "ci", "env"}

type createFlags struct {
	name string
	yes  bool
}

func newCreateCmd(deps Dependencies) *cobra.Command {
	flags := &createFlags{}
	cmd := &cobra.Command{
		Use:     "create [dir]",
		Long:    i18n.T("create.tip"),
		Example: "  one create demo\n  one create .\n  one create demo --yes",
		Args:    i18n.MaximumNArgs(1),
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
	i18n.MarkFlagUsage(cmd, "name", "create.flag.name")
	i18n.MarkFlagUsage(cmd, "yes", "create.flag.yes")
	helpui.MarkAdvanced(cmd, "name")
	i18n.MarkShort(cmd, "create.short")
	i18n.MarkLong(cmd, "create.tip")
	return cmd
}
