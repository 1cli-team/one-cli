// Package misecmd exposes explicit, reviewable mise configuration generation.
package misecmd

import (
	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/miseconfig"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func Commands() []*cobra.Command {
	var opts miseconfig.Options
	var dryRun bool
	cmd := &cobra.Command{
		Use: "mise", Short: "Generate or refresh optional mise tool and task configuration",
		Example: "  one configure mise --dry-run -o json\n  one configure mise\n  one dev web",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			w, err := execution.ResolveWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			plan, err := miseconfig.Build(w.Root(), opts)
			if err != nil {
				return err
			}
			if !dryRun {
				if err := plan.Apply(cmd.Context()); err != nil {
					return err
				}
			}
			output.Emit(plan)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print proposed file contents without writing files or executing mise")
	cmd.Flags().StringVar(&opts.NodeVersion, "node-version", "", "Exact Node version (default: previous generated version or 24.15.0)")
	cmd.Flags().StringVar(&opts.GoVersion, "go-version", "", "Exact Go version override (default: each project's go.mod)")
	return []*cobra.Command{cmd}
}
