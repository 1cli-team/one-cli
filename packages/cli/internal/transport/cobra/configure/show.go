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

// ───────────────────── show ─────────────────────

type showResult struct {
	Schema           string          `json:"schema"`
	Domain           string          `json:"domain"`
	Backend          string          `json:"backend"`
	Name             string          `json:"name"`
	Profile          profile.Profile `json:"profile"`
	CredentialSource string          `json:"credentialSource"`
	Reveal           bool            `json:"reveal"`
}

func (r showResult) RenderTTY(w io.Writer) {
	fmt.Fprintf(w, i18n.T("configure.show_connection")+"\n", r.Name)
	fmt.Fprintf(w, i18n.T("configure.show_service")+"\n", serviceLabel(profile.Domain(r.Domain), r.Backend))
	src := r.CredentialSource
	if src == "" {
		src = profile.SourceFile
	}
	fmt.Fprintf(w, i18n.T("configure.show_credential_source")+"\n", src)
	if r.Profile.Infisical != nil {
		i := r.Profile.Infisical
		fmt.Fprintln(w, "infisical:")
		fmt.Fprintf(w, "  siteUrl:     %s\n", i.SiteURL)
		if i.Credentials != nil {
			fmt.Fprintf(w, "  clientId:     %s\n", i.Credentials.ClientID)
			fmt.Fprintf(w, "  clientSecret: %s\n", i.Credentials.ClientSecret)
		}
	}
	if !r.Reveal {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, i18n.T("configure.show_masked"))
	}
}

func buildShowCmd(profiles *configureapp.ProfileService) *cobra.Command {
	var (
		reveal      bool
		profileName string
	)
	cmd := &cobra.Command{
		Use:   "show [service-id] [--profile <name>]",
		Short: i18n.T("configure.show.short"),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			selection, err := resolveExistingConnection(profiles, args, profileName)
			if err != nil {
				return err
			}
			resolved, err := profiles.Resolve(profile.ResolveInput{
				Domain:       selection.Domain,
				Backend:      selection.Backend,
				FlagOverride: selection.Name,
			})
			if err != nil {
				return err
			}
			p := resolved.Profile
			if !reveal {
				p, err = profiles.MaskProfile(p)
				if err != nil {
					return err
				}
			}
			output.Emit(showResult{
				Schema:           "one-cli/configure-show/v1",
				Domain:           string(selection.Domain),
				Backend:          selection.Backend,
				Name:             selection.Name,
				Profile:          p,
				CredentialSource: resolved.CredSource,
				Reveal:           reveal,
			})
			return nil
		},
		ValidArgsFunction: pairCompletion(profiles),
	}
	cmd.Flags().StringVar(&profileName, "profile", "", i18n.T("configure.flag.profile_existing"))
	cmd.Flags().BoolVar(&reveal, "reveal", false, i18n.T("configure.flag.reveal"))
	i18n.MarkFlagUsage(cmd, "profile", "configure.flag.profile_existing")
	i18n.MarkFlagUsage(cmd, "reveal", "configure.flag.reveal")
	helpui.MarkAdvanced(cmd, "profile", "reveal")
	i18n.MarkShort(cmd, "configure.show.short")
	return cmd
}

func maskCredentials(profiles *configureapp.ProfileService, p profile.Profile) (profile.Profile, error) {
	return profiles.MaskProfile(p)
}
