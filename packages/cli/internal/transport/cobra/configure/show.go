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
	if r.Profile.Kustomize != nil {
		k := r.Profile.Kustomize
		fmt.Fprintln(w, "kustomize:")
		if k.KubeconfigPath != "" {
			fmt.Fprintf(w, "  kubeconfig: %s\n", k.KubeconfigPath)
		}
		if k.KubeconfigContext != "" {
			fmt.Fprintf(w, "  context:    %s\n", k.KubeconfigContext)
		}
	}
	if r.Profile.S3 != nil {
		o := r.Profile.S3
		fmt.Fprintln(w, "s3:")
		if o.Endpoint != "" {
			fmt.Fprintf(w, "  endpoint:       %s\n", o.Endpoint)
		} else {
			fmt.Fprintf(w, "  endpoint:       (AWS S3 default)\n")
		}
		if o.Region != "" {
			fmt.Fprintf(w, "  region:         %s\n", o.Region)
		}
		if o.ForcePathStyle {
			fmt.Fprintln(w, "  forcePathStyle: true (MinIO / RustFS)")
		}
		if o.Credentials != nil {
			fmt.Fprintf(w, "  accessKeyId:     %s\n", o.Credentials.AccessKeyID)
			fmt.Fprintf(w, "  accessKeySecret: %s\n", o.Credentials.AccessKeySecret)
		}
	}
	if r.Profile.Vercel != nil {
		v := r.Profile.Vercel
		fmt.Fprintln(w, "vercel:")
		if v.Team != "" {
			fmt.Fprintf(w, "  team:     %s\n", v.Team)
		} else {
			fmt.Fprintf(w, "  team:     (personal scope)\n")
		}
		if v.Credentials != nil {
			fmt.Fprintf(w, "  apiToken: %s\n", v.Credentials.APIToken)
		}
	}
	if r.Profile.Container != nil {
		c := r.Profile.Container
		fmt.Fprintln(w, "container:")
		if c.Registry != "" {
			fmt.Fprintf(w, "  registry:  %s\n", c.Registry)
		}
		if c.Region != "" {
			fmt.Fprintf(w, "  region:    %s\n", c.Region)
		}
		if c.Namespace != "" {
			fmt.Fprintf(w, "  namespace: %s\n", c.Namespace)
		}
		if c.Credentials != nil {
			fmt.Fprintf(w, "  username:  %s\n", c.Credentials.Username)
			fmt.Fprintf(w, "  password:  %s\n", c.Credentials.Password)
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
