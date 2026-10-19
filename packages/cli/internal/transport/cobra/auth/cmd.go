package authcmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	authentication "github.com/torchstellar-team/one-cli/packages/one-client/provider"
	session "github.com/torchstellar-team/one-cli/packages/one-client/provider"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/prompt"
)

func Commands() []*cobra.Command {
	var site, clientID, organization string
	var secretStdin bool
	var noBrowser bool
	login := &cobra.Command{Use: "login", Short: i18n.T("auth.login.short"), Long: i18n.T("auth.login.tip"), Args: i18n.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if clientID != "" || secretStdin {
			if clientID == "" || !secretStdin {
				return i18n.Errorf("server.machine_cli_required")
			}
			raw, err := io.ReadAll(io.LimitReader(c.InOrStdin(), 16<<10))
			if err != nil {
				return err
			}
			info, err := session.ConnectMachine(c.Context(), site, clientID, strings.TrimSpace(string(raw)), organization)
			if err != nil {
				return err
			}
			output.Emit(info)
			return nil
		}
		manual := noBrowser || (!c.Flags().Changed("no-browser") && isSSH())
		if manual && !output.CanPrompt() {
			return i18n.Errorf("auth.login.terminal_required")
		}
		ctx, cancel := context.WithTimeout(c.Context(), 10*time.Minute)
		defer cancel()
		a, e := session.Start(ctx, site)
		if e != nil {
			return e
		}
		defer func() {
			a.Cancel()
			_, _ = a.Wait()
		}()
		if !manual {
			fmt.Fprintf(c.ErrOrStderr(), i18n.T("auth.login.browser"), a.URL)
			if err := browser.OpenURL(a.URL); err != nil {
				if !output.CanPrompt() {
					return i18n.Errorf("auth.login.browser_failed", err)
				}
				manual = true
			}
		}
		if manual {
			fmt.Fprintf(c.ErrOrStderr(), i18n.T("auth.login.remote"), a.URL)
			promptCtx, stopPrompt := context.WithCancel(ctx)
			defer stopPrompt()
			go func() {
				_, _ = a.Wait()
				stopPrompt()
			}()
			token, err := prompt.PasswordContext(promptCtx, i18n.T("auth.login.browser_token"), nil)
			if _, resultErr, done := a.Result(); done {
				// A local browser or SSH tunnel may still deliver the callback.
				if resultErr != nil {
					return resultErr
				}
			} else if err != nil {
				if ctx.Err() != nil {
					return i18n.Errorf("auth.login_timeout")
				}
				return err
			} else if err := a.SubmitBrowserToken(token); err != nil {
				return err
			}
		}
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
	i18n.MarkLong(login, "auth.login.tip")
	login.Flags().StringVar(&site, "site-url", session.DefaultSiteURL, i18n.T("auth.flag.site"))
	login.Flags().BoolVar(&noBrowser, "no-browser", false, i18n.T("auth.flag.no_browser"))
	i18n.MarkFlagUsage(login, "site-url", "auth.flag.site")
	i18n.MarkFlagUsage(login, "no-browser", "auth.flag.no_browser")
	login.Flags().StringVar(&clientID, "client-id", "", "")
	login.Flags().StringVar(&organization, "organization-id", "", "")
	login.Flags().BoolVar(&secretStdin, "client-secret-stdin", false, "")
	i18n.MarkFlagUsage(login, "client-id", "server.flag.client_id")
	i18n.MarkFlagUsage(login, "organization-id", "server.flag.organization_id")
	i18n.MarkFlagUsage(login, "client-secret-stdin", "server.flag.client_secret_stdin")
	who := &cobra.Command{Use: "whoami", Short: i18n.T("auth.whoami.short"), Args: i18n.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		info, e := session.Status(c.Context())
		if e != nil {
			return e
		}
		output.Emit(info)
		return nil
	}}
	i18n.MarkShort(who, "auth.whoami.short")
	logout := &cobra.Command{Use: "logout", Short: i18n.T("auth.logout.short"), Args: i18n.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if e := session.Logout(c.Context()); e != nil {
			return e
		}
		output.Emit(map[string]any{"loggedIn": false})
		return nil
	}}
	i18n.MarkShort(logout, "auth.logout.short")
	return []*cobra.Command{login, who, logout}
}

func isSSH() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != ""
}
