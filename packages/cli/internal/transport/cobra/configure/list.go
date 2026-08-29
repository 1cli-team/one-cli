package configurecmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

// ───────────────────── list ─────────────────────

type listResult struct {
	Schema   string         `json:"schema"`
	Domain   string         `json:"domain"`
	Backend  string         `json:"backend"`
	Default  string         `json:"default,omitempty"`
	Profiles []profileEntry `json:"profiles"`
}

type profileEntry struct {
	Name             string `json:"name"`
	Default          bool   `json:"default"`
	CredentialSource string `json:"credentialSource,omitempty"`
}

func (r listResult) RenderTTY(w io.Writer) {
	if len(r.Profiles) == 0 {
		fmt.Fprintf(w, i18n.T("configure.no_service_connections")+"\n", r.Domain+"/"+r.Backend)
		return
	}
	for _, p := range r.Profiles {
		marker := ""
		if p.Default {
			marker = i18n.T("configure.default_marker")
		}
		fmt.Fprintf(w, "  %s  %s%s\n", p.Name, serviceLabel(profile.Domain(r.Domain), r.Backend), marker)
	}
}

// listAllResult is emitted by `one configure list` (no pair). It
// rolls up every (domain, backend) section into a single envelope so
// scripts can scan one's profile state without 5 separate calls.
type listAllResult struct {
	Schema   string                `json:"schema"`
	Sections []listAllSectionEntry `json:"sections"`
}

type listAllSectionEntry struct {
	Domain   string         `json:"domain"`
	Backend  string         `json:"backend"`
	Default  string         `json:"default,omitempty"`
	Profiles []profileEntry `json:"profiles"`
}

func (r listAllResult) RenderTTY(w io.Writer) {
	any := false
	for _, s := range r.Sections {
		if len(s.Profiles) == 0 {
			continue
		}
		any = true
		for _, p := range s.Profiles {
			marker := ""
			if p.Default {
				marker = i18n.T("configure.default_marker")
			}
			fmt.Fprintf(w, "  %s  %s%s\n", p.Name, serviceLabel(profile.Domain(s.Domain), s.Backend), marker)
		}
	}
	if !any {
		fmt.Fprintln(w, i18n.T("configure.no_connections"))
	}
}

func buildListCmd(profiles *configureapp.ProfileService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [pair]",
		Short: i18n.T("configure.list.short"),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := profiles.Load()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				output.Emit(collectAllSections(profiles, cfg))
				return nil
			}
			domain, backend, err := parsePair(profiles, args[0])
			if err != nil {
				return err
			}
			output.Emit(collectSection(profiles, cfg, domain, backend))
			return nil
		},
		ValidArgsFunction: pairCompletion(profiles),
	}
	i18n.MarkShort(cmd, "configure.list.short")
	return cmd
}

func collectSection(profiles *configureapp.ProfileService, cfg *profile.Config, domain profile.Domain, backend string) listResult {
	names, defaultName := listSection(profiles, cfg, domain, backend)
	sort.Strings(names)
	entries := make([]profileEntry, 0, len(names))
	for _, n := range names {
		entries = append(entries, profileEntry{
			Name:             n,
			Default:          n == defaultName,
			CredentialSource: profiles.CredentialSource(cfg, domain, backend, n),
		})
	}
	return listResult{
		Schema:   "one-cli/configure-list/v1",
		Domain:   string(domain),
		Backend:  backend,
		Default:  defaultName,
		Profiles: entries,
	}
}

func collectAllSections(profiles *configureapp.ProfileService, cfg *profile.Config) listAllResult {
	pairs := supportedPairs(profiles)
	sections := make([]listAllSectionEntry, 0, len(pairs))
	for _, p := range pairs {
		section := collectSection(profiles, cfg, p.Domain, p.Backend)
		sections = append(sections, listAllSectionEntry{
			Domain:   section.Domain,
			Backend:  section.Backend,
			Default:  section.Default,
			Profiles: section.Profiles,
		})
	}
	return listAllResult{
		Schema:   "one-cli/configure-list-all/v1",
		Sections: sections,
	}
}
