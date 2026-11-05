// Package misecmd exposes explicit, reviewable mise configuration generation.
package misecmd

import (
	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/miseconfig"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
)

func Commands() []*cobra.Command {
	var opts miseconfig.Options
	var dryRun bool
	cmd := &cobra.Command{
		Use: "mise", Short: i18n.T("mise.configure.short"),
		Example: "  one init mise --dry-run -o json\n  one init mise\n  one run dev -p web",
		Args:    i18n.NoArgs,
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
	i18n.MarkShort(cmd, "mise.configure.short")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.T("mise.flag.dry_run"))
	i18n.MarkFlagUsage(cmd, "dry-run", "mise.flag.dry_run")
	cmd.Flags().StringVar(&opts.NodeVersion, "node-version", "", i18n.T("mise.flag.node_version"))
	i18n.MarkFlagUsage(cmd, "node-version", "mise.flag.node_version")
	cmd.Flags().StringVar(&opts.GoVersion, "go-version", "", i18n.T("mise.flag.go_version"))
	i18n.MarkFlagUsage(cmd, "go-version", "mise.flag.go_version")
	for _, flag := range []struct {
		name  string
		value *string
		key   string
	}{
		{"bun-version", &opts.BunVersion, "mise.flag.bun_version"},
		{"deno-version", &opts.DenoVersion, "mise.flag.deno_version"},
		{"python-version", &opts.PythonVersion, "mise.flag.python_version"},
		{"uv-version", &opts.UVVersion, "mise.flag.uv_version"},
	} {
		cmd.Flags().StringVar(flag.value, flag.name, "", i18n.T(flag.key))
		i18n.MarkFlagUsage(cmd, flag.name, flag.key)
	}
	return []*cobra.Command{cmd}
}
