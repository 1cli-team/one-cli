// Package templatescmd is the `one templates` command. Lives under
// internal/transport/cobra/ alongside the other domain command packages; uses the
// explicit root-command composition.
package templatescmd

import (
	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func Commands() []*cobra.Command { return buildContributions() }

func buildContributions() []*cobra.Command {
	parent := &cobra.Command{
		Use:  "templates",
		RunE: runList,
	}
	list := &cobra.Command{
		Use:  "list",
		RunE: runList,
	}
	i18n.MarkShort(list, "templates.list.short")
	parent.AddCommand(list)
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
