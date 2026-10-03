package authcmd

import (
	"fmt"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	authentication "github.com/torchstellar-team/one-cli/packages/cli/internal/application/authentication"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func Commands() []*cobra.Command {
	var site string
	login := &cobra.Command{Use: "login", Short: i18n.T("auth.login.short"), Args: i18n.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		a, e := session.Start(c.Context(), site)
		if e != nil {
			return e
		}
		defer a.Cancel()
		fmt.Fprintf(c.ErrOrStderr(), i18n.T("auth.login.browser"), a.URL)
		_ = browser.OpenURL(a.URL)
		info, e := a.Wait()
		if e != nil {
			return e
		}
		fmt.Fprintln(c.ErrOrStderr(), i18n.T("auth.shared_preparing"))
		shared := authentication.PrepareAfterLogin(c.Context(), info)
		if shared.Status == "failed" {
			fmt.Fprintln(c.ErrOrStderr(), i18n.Tf("auth.shared_failed", shared.Error))
		}
		output.Emit(struct {
			session.Info
			SharedCredentials authentication.SharedCredentialsState `json:"sharedCredentials"`
		}{info, shared})
		return nil
	}}
	i18n.MarkShort(login, "auth.login.short")
	login.Flags().StringVar(&site, "site-url", session.DefaultSiteURL, i18n.T("auth.flag.site"))
	i18n.MarkFlagUsage(login, "site-url", "auth.flag.site")
	who := &cobra.Command{Use: "whoami", Short: i18n.T("auth.whoami.short"), Args: i18n.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		info, e := session.Status()
		if e != nil {
			return e
		}
		output.Emit(info)
		return nil
	}}
	i18n.MarkShort(who, "auth.whoami.short")
	logout := &cobra.Command{Use: "logout", Short: i18n.T("auth.logout.short"), Args: i18n.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if e := session.Logout(); e != nil {
			return e
		}
		output.Emit(map[string]any{"loggedIn": false})
		return nil
	}}
	i18n.MarkShort(logout, "auth.logout.short")
	return []*cobra.Command{login, who, logout}
}
