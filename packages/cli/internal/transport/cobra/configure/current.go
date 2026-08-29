package configurecmd

import (
	"fmt"
	"io"

	"text/tabwriter"

	"github.com/spf13/cobra"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

// sectionCredentialSource returns the credentialSource string of one
// profile inside (domain, backend), or "" when the section / profile
// is missing. Used by `list` to render a per-profile source column.
// Dotenv / kustomize profiles have no credentialSource discriminator
// and always return "".
// ───────────────────── current ─────────────────────

type currentResult struct {
	Schema  string `json:"schema"`
	Domain  string `json:"domain"`
	Backend string `json:"backend"`
	Default string `json:"default,omitempty"`
}

func (r currentResult) RenderTTY(w io.Writer) {
	if r.Default == "" {
		fmt.Fprintf(w, i18n.T("configure.no_current")+"\n", r.Domain+"/"+r.Backend)
		return
	}
	fmt.Fprintf(w, "%s\n", r.Default)
}

// currentAllResult is emitted by `one configure current` (no pair),
// listing the default profile of every section.
type currentAllResult struct {
	Schema   string                   `json:"schema"`
	Defaults []currentAllSectionEntry `json:"defaults"`
}

type currentAllSectionEntry struct {
	Domain  string `json:"domain"`
	Backend string `json:"backend"`
	Default string `json:"default,omitempty"`
}

func (r currentAllResult) RenderTTY(w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, s := range r.Defaults {
		defaultName := s.Default
		if defaultName == "" {
			defaultName = i18n.T("configure.none")
		}
		fmt.Fprintf(tw, "%s\t%s\n", serviceLabel(profile.Domain(s.Domain), s.Backend), defaultName)
	}
	_ = tw.Flush()
}

func buildCurrentCmd(profiles *configureapp.ProfileService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "current [pair]",
		Short: i18n.T("configure.current.short"),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := profiles.Load()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				out := currentAllResult{Schema: "one-cli/configure-current-all/v1"}
				for _, p := range supportedPairs(profiles) {
					_, defaultName := listSection(profiles, cfg, p.Domain, p.Backend)
					out.Defaults = append(out.Defaults, currentAllSectionEntry{
						Domain:  string(p.Domain),
						Backend: p.Backend,
						Default: defaultName,
					})
				}
				output.Emit(out)
				return nil
			}
			domain, backend, err := parsePair(profiles, args[0])
			if err != nil {
				return err
			}
			_, defaultName := listSection(profiles, cfg, domain, backend)
			output.Emit(currentResult{
				Schema:  "one-cli/configure-current/v1",
				Domain:  string(domain),
				Backend: backend,
				Default: defaultName,
			})
			return nil
		},
		ValidArgsFunction: pairCompletion(profiles),
	}
	i18n.MarkShort(cmd, "configure.current.short")
	return cmd
}
