package gatewaycmd

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/linear"
)

const cancelAuthorizationTimeout = 4 * time.Second

func linearSetupCommand(worker *string) *cobra.Command {
	var config tracking.OAuthConfig
	var noBrowser bool
	cmd := &cobra.Command{Use: "linear", Args: i18n.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		client, address, err := openClient()
		if err != nil {
			return err
		}
		oauth := config
		if oauth.ClientID == "" || oauth.RedirectURI == "" {
			state, err := linear.New().Status(cmd.Context())
			if err != nil {
				return err
			}
			if oauth.ClientID == "" {
				oauth.ClientID = state.Config.ClientID
			}
			if oauth.RedirectURI == "" {
				oauth.RedirectURI = state.Config.RedirectURI
			}
		}
		if oauth.RedirectURI == "" {
			oauth.RedirectURI = "http://127.0.0.1:39127/oauth/linear/callback"
		}
		redirect, err := url.Parse(oauth.RedirectURI)
		if err != nil || oauth.ClientID == "" || !validLinearRedirect(redirect) {
			return i18n.Errorf("gateway.cli.setup_oauth_config")
		}
		listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", redirect.Port()))
		if err != nil {
			return tracking.Fail("callback_busy")
		}
		defer listener.Close()

		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
		defer cancel()
		var attempt tracking.Authorization
		if err := client.Setup(ctx, *worker, gateway.SetupInput{Operation: gateway.SetupLinearStart, OAuth: &oauth}, &attempt); err != nil {
			return setupError(err)
		}
		data, err := waitLinearCallback(ctx, cmd, client, *worker, attempt, redirect, listener, noBrowser)
		if err != nil {
			cancelSetupAuthorization(ctx, client, *worker, attempt.ID)
			return err
		}
		output.Emit(result{Schema: "one-cli/gateway/v1", URL: address, Data: data})
		return nil
	}}
	cmd.Flags().StringVar(&config.ClientID, "client-id", "", "")
	cmd.Flags().StringVar(&config.RedirectURI, "redirect-uri", "", "")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "")
	i18n.MarkFlagUsage(cmd, "client-id", "linear.flag.client-id")
	i18n.MarkFlagUsage(cmd, "redirect-uri", "linear.flag.redirect-uri")
	i18n.MarkFlagUsage(cmd, "no-browser", "linear.flag.no-browser")
	i18n.MarkShort(cmd, "gateway.cli.setup_linear")
	return cmd
}

func validLinearRedirect(u *url.URL) bool {
	port, err := strconv.Atoi(u.Port())
	return err == nil && port >= 1024 && port <= 65535 &&
		u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost") &&
		u.Path == "/oauth/linear/callback" && u.RawPath == "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Fragment == ""
}

func waitLinearCallback(ctx context.Context, cmd *cobra.Command, client *gateway.Client, worker string, attempt tracking.Authorization, redirect *url.URL, listener net.Listener, noBrowser bool) (gateway.SetupReadiness, error) {
	authorize, err := url.Parse(attempt.URL)
	if err != nil || authorize.Scheme != "https" || authorize.Host != "linear.app" || authorize.User != nil ||
		authorize.Path != "/oauth/authorize" || attempt.ID == "" || authorize.Query().Get("state") != attempt.ID ||
		authorize.Query().Get("redirect_uri") != redirect.String() {
		return gateway.SetupReadiness{}, tracking.Fail("oauth_state")
	}
	type outcome struct {
		data gateway.SetupReadiness
		err  error
	}
	finished := make(chan outcome, 1)
	var consumed atomic.Bool
	server := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		if r.Method != http.MethodGet || r.URL.Path != redirect.Path || !strings.EqualFold(r.Host, redirect.Host) {
			http.NotFound(w, r)
			return
		}
		query := r.URL.Query()
		if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(attempt.ID)) != 1 || !consumed.CompareAndSwap(false, true) {
			http.Error(w, i18n.T("linear.error.oauth_state"), http.StatusBadRequest)
			return
		}
		var result outcome
		code := query.Get("code")
		if query.Get("error") != "" || code == "" || len(code) > 2048 {
			result.err = tracking.Fail("oauth_denied")
		} else {
			input := gateway.SetupInput{Operation: gateway.SetupLinearFinish, State: attempt.ID, Code: code}
			if err := client.Setup(ctx, worker, input, &result.data); err != nil {
				result.err = setupError(err)
			}
		}
		if result.err != nil {
			http.Error(w, i18n.T("linear.error.oauth_failed"), http.StatusBadRequest)
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, i18n.T("linear.oauth.success"))
		}
		finished <- result
	})
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	defer func() {
		closing, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(closing)
	}()
	fmt.Fprintln(cmd.ErrOrStderr(), attempt.URL)
	if !noBrowser {
		if err := browser.OpenURL(attempt.URL); err != nil {
			return gateway.SetupReadiness{}, tracking.Fail("oauth_browser")
		}
	}
	select {
	case result := <-finished:
		return result.data, result.err
	case <-ctx.Done():
		return gateway.SetupReadiness{}, tracking.Fail("oauth_expired")
	case err := <-served:
		return gateway.SetupReadiness{}, err
	}
}
