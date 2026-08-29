package configurecmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

// ───────────────────── remove ─────────────────────

type removeResult struct {
	Schema  string `json:"schema"`
	Domain  string `json:"domain"`
	Backend string `json:"backend"`
	Name    string `json:"name"`
}

func (r removeResult) RenderTTY(w io.Writer) {
	fmt.Fprintf(w, i18n.T("configure.remove_success")+"\n", r.Name, serviceLabel(profile.Domain(r.Domain), r.Backend))
}

func buildRemoveCmd(profiles *configureapp.ProfileService) *cobra.Command {
	var profileName string
	cmd := &cobra.Command{
		Use:   "remove [service-id] [--profile <name>]",
		Short: i18n.T("configure.remove.short"),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			selection, err := resolveExistingConnection(profiles, args, profileName)
			if err != nil {
				return err
			}
			if err := profiles.Remove(selection.Domain, selection.Backend, selection.Name); err != nil {
				return err
			}
			output.Emit(removeResult{
				Schema:  "one-cli/configure-remove/v1",
				Domain:  string(selection.Domain),
				Backend: selection.Backend,
				Name:    selection.Name,
			})
			return nil
		},
		ValidArgsFunction: pairCompletion(profiles),
	}
	cmd.Flags().StringVar(&profileName, "profile", "", i18n.T("configure.flag.profile_existing"))
	i18n.MarkFlagUsage(cmd, "profile", "configure.flag.profile_existing")
	helpui.MarkAdvanced(cmd, "profile")
	i18n.MarkShort(cmd, "configure.remove.short")
	return cmd
}

// ───────────────────── shared helpers ─────────────────────

// listSection returns (names, default) for one (domain, backend)
// section. Each (domain, backend) maps to a discrete struct field on
// profile.Config; the profile package keeps the struct flat for v3
// schema readability so we mirror that here rather than going through
// a generic accessor.
func listSection(profiles *configureapp.ProfileService, cfg *profile.Config, domain profile.Domain, backend string) ([]string, string) {
	section, err := profiles.Section(cfg, domain, backend)
	if err != nil {
		return nil, ""
	}
	return section.Names, section.Default
}
