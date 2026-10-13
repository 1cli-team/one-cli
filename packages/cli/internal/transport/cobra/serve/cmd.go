// Package servecmd keeps the reserved migration entrypoint for the old browser UI.
package servecmd

import (
	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func Commands() []*cobra.Command {
	cmd := &cobra.Command{Use: "serve", Args: i18n.NoArgs, RunE: func(*cobra.Command, []string) error { return i18n.Errorf("desktop.serve_moved") }}
	i18n.MarkShort(cmd, "serve.short")
	i18n.MarkLong(cmd, "serve.tip")
	return []*cobra.Command{cmd}
}
