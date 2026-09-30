package upgradecmd

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/updatecheck"
)

func Command() *cobra.Command { return command(updatecheck.Upgrade) }

func command(upgrade func(context.Context, string) (*updatecheck.Result, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "upgrade",
		Args:    i18n.NoArgs,
		Example: "  one upgrade\n  one upgrade -o json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if output.IsTTY() {
				fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("upgrade.checking"))
			}
			result, err := upgrade(cmd.Context(), cmd.Root().Version)
			if err != nil {
				return err
			}
			output.Emit(&upgradeResult{result})
			return nil
		},
	}
	i18n.MarkShort(cmd, "upgrade.short")
	i18n.MarkLong(cmd, "upgrade.long")
	return cmd
}

type upgradeResult struct{ *updatecheck.Result }

func (r *upgradeResult) RenderTTY(w io.Writer) {
	switch r.Status {
	case "updated":
		fmt.Fprintln(w, i18n.Tf("upgrade.updated", r.CurrentVersion, r.LatestVersion))
	case "pending":
		fmt.Fprintln(w, i18n.Tf("upgrade.pending", r.LatestVersion))
	default:
		fmt.Fprintln(w, i18n.Tf("upgrade.current", r.CurrentVersion, r.LatestVersion))
	}
}
