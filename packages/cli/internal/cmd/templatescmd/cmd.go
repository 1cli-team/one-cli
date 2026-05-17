// Package templatescmd is the `one templates` command. Lives under
// internal/cmd/ alongside the other domain command packages; uses the
// cliexts registration mechanism.
package templatescmd

import (
	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/template"
)

func init() {
	cliexts.Register("templates", buildContributions)
}

func buildContributions() []*cobra.Command {
	parent := &cobra.Command{
		Use:  "templates",
		RunE: runList,
	}
	parent.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出可用模板",
		RunE:  runList,
	})
	i18n.MarkShort(parent, "templates.short")
	return []*cobra.Command{parent}
}

func runList(cmd *cobra.Command, _ []string) error {
	result, err := template.List(cmd.Context())
	if err != nil {
		return err
	}
	output.Emit(result)
	return nil
}
