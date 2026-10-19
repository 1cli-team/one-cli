package localecmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	oneclient "github.com/torchstellar-team/one-cli/packages/one-client"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/preferences"
)

// ───────────────────── locale ─────────────────────
//
// `one locale` reads or updates the user's global display-language preference.
// Workspace configuration is stored separately in one.manifest.toml.

type localeResult struct {
	Schema       string `json:"schema"`
	StoredLocale string `json:"stored_locale"`
	Resolved     string `json:"resolved"`
	Detected     string `json:"detected,omitempty"`
	ConfigPath   string `json:"config_path"`
	Updated      bool   `json:"updated"`
}

func (r *localeResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	if r.Updated {
		fmt.Fprintf(w, i18n.T("locale_success")+"\n", r.StoredLocale)
	}
	fmt.Fprintf(w, i18n.T("locale_stored")+"\n", r.StoredLocale)
	fmt.Fprintf(w, i18n.T("locale_resolved"), r.Resolved)
	if r.StoredLocale == preferences.LocaleAuto && r.Detected != "" {
		fmt.Fprint(w, i18n.T("locale_from_env"))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, i18n.T("locale_path")+"\n", r.ConfigPath)
}

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "locale [auto|zh-CN|en-US]",
		Long: i18n.T("locale.tip"),
		Args: i18n.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := oneclient.New()
			prefs, err := client.Preferences(cmd.Context())
			if err != nil {
				return cliErrors.New(cliErrors.PREFERENCES_FILE_INVALID,
					i18n.Tf("locale.read_failed", err.Error()))
			}
			path := prefs.ConfigPath

			if len(args) == 1 {
				newLocale := strings.TrimSpace(args[0])
				if !preferences.IsValidLocale(newLocale) {
					return cliErrors.New(cliErrors.PREFERENCES_INVALID,
						i18n.Tf("locale.unknown", newLocale))
				}
				if _, err := client.SetLocale(cmd.Context(), newLocale); err != nil {
					return err
				}
				_ = i18n.Init(i18n.Resolve(newLocale))
				i18n.RefreshTree(cmd.Root())
				output.Emit(&localeResult{
					Schema:       "one-cli/locale/v1",
					StoredLocale: newLocale,
					Resolved:     i18n.Resolve(newLocale),
					Detected:     i18n.DetectFromEnv(),
					ConfigPath:   path,
					Updated:      true,
				})
				return nil
			}

			output.Emit(&localeResult{
				Schema:       "one-cli/locale/v1",
				StoredLocale: prefs.StoredLocale,
				Resolved:     i18n.Resolve(prefs.StoredLocale),
				Detected:     i18n.DetectFromEnv(),
				ConfigPath:   path,
				Updated:      false,
			})
			return nil
		},
	}
	i18n.MarkLong(cmd, "locale.tip")
	i18n.MarkShort(cmd, "locale.short")
	return cmd
}
