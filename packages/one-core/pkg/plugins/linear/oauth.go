package linear

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type authAttempt struct {
	public   tracking.Authorization
	cancel   context.CancelFunc
	state    string
	verifier string
	consumed bool
}

func validateConfig(cfg tracking.OAuthConfig) error {
	u, err := url.Parse(cfg.RedirectURI)
	if err != nil || strings.TrimSpace(cfg.ClientID) == "" || len(cfg.ClientID) > 256 || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/oauth/linear/callback" {
		return tracking.Fail("config")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1024 || port > 65535 {
		return tracking.Fail("config")
	}
	return nil
}

func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (p *Plugin) Authorize(parent context.Context) (tracking.Authorization, error) {
	var cfg tracking.OAuthConfig
	if err := p.transaction(parent, func(doc *document) (bool, error) { cfg = doc.Config; return false, nil }); err != nil {
		return tracking.Authorization{}, err
	}
	if err := validateConfig(cfg); err != nil {
		return tracking.Authorization{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.auth != nil && p.auth.public.Status == "pending" {
		return p.auth.public, nil
	}
	u, _ := url.Parse(cfg.RedirectURI)
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", u.Port()))
	if err != nil {
		return tracking.Authorization{}, tracking.Fail("callback_busy")
	}
	state, err := randomSecret()
	if err != nil {
		listener.Close()
		return tracking.Authorization{}, err
	}
	verifier, err := randomSecret()
	if err != nil {
		listener.Close()
		return tracking.Authorization{}, err
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	challenge := sha256.Sum256([]byte(verifier))
	authorize, _ := url.Parse(p.authorizeURL)
	query := url.Values{"client_id": {cfg.ClientID}, "redirect_uri": {cfg.RedirectURI}, "response_type": {"code"}, "scope": {"read,issues:create,comments:create"}, "actor": {"user"}, "state": {state}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"}, "prompt": {"consent"}}
	authorize.RawQuery = query.Encode()
	attempt := &authAttempt{public: tracking.Authorization{ID: state, URL: authorize.String(), Status: "pending"}, cancel: cancel, state: state, verifier: verifier}
	p.auth = attempt
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method != http.MethodGet || r.URL.Path != u.Path || !strings.EqualFold(r.Host, u.Host) {
			http.NotFound(w, r)
			return
		}
		p.mu.Lock()
		valid := p.auth == attempt && !attempt.consumed && ctx.Err() == nil && subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(attempt.state)) == 1
		if valid {
			attempt.consumed = true
		}
		p.mu.Unlock()
		if !valid {
			http.Error(w, i18n.T("linear.error.oauth_state"), http.StatusBadRequest)
			return
		}
		var authErr error
		if r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
			authErr = tracking.Fail("oauth_denied")
		} else {
			authErr = p.finishAuthorization(ctx, attempt, cfg, r.URL.Query().Get("code"))
		}
		p.mu.Lock()
		if p.auth == attempt && attempt.public.Status == "pending" {
			if authErr == nil {
				attempt.public.Status = "connected"
			} else {
				attempt.public.Status = "failed"
				attempt.public.ErrorCode = "oauth_failed"
			}
		}
		p.mu.Unlock()
		message := i18n.T("linear.oauth.success")
		if authErr != nil {
			message = i18n.T("linear.error.oauth_failed")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<!doctype html><html><meta charset=\"utf-8\"><title>One · Linear</title><body style=\"font:16px system-ui;padding:48px\"><h1>One · Linear</h1><p>%s</p></body></html>", html.EscapeString(message))
		go cancel()
	})
	go func() { _ = server.Serve(listener) }()
	go func() {
		<-ctx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		_ = server.Shutdown(shutdown)
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.auth == attempt && attempt.public.Status == "pending" {
			attempt.public.Status = "expired"
			attempt.public.ErrorCode = "oauth_expired"
		}
	}()
	return attempt.public, nil
}

func (p *Plugin) finishAuthorization(ctx context.Context, attempt *authAttempt, cfg tracking.OAuthConfig, code string) error {
	tokens, err := p.exchange(ctx, url.Values{"grant_type": {"authorization_code"}, "client_id": {cfg.ClientID}, "redirect_uri": {cfg.RedirectURI}, "code": {code}, "code_verifier": {attempt.verifier}})
	if err != nil {
		return err
	}
	var data struct {
		Viewer struct {
			ID           string         `json:"id"`
			Name         string         `json:"name"`
			Organization tracking.Named `json:"organization"`
		} `json:"viewer"`
	}
	if err := p.graphWithToken(ctx, tokens.AccessToken, `query OneLinearViewer { viewer { id name organization { id name } } }`, nil, &data); err != nil {
		return err
	}
	if data.Viewer.ID == "" || data.Viewer.Organization.ID == "" {
		return tracking.Fail("remote")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.auth != attempt || ctx.Err() != nil || attempt.public.Status != "pending" {
		return tracking.Fail("oauth_expired")
	}
	id := data.Viewer.Organization.ID + ":" + data.Viewer.ID
	return p.transaction(ctx, func(doc *document) (bool, error) {
		if doc.Config != cfg {
			return false, tracking.Fail("config")
		}
		doc.Connections[id] = credential{Connection: tracking.Connection{ID: id, UserID: data.Viewer.ID, Name: data.Viewer.Name, OrganizationID: data.Viewer.Organization.ID, OrganizationName: data.Viewer.Organization.Name}, AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken, ExpiresAt: time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second), ClientID: cfg.ClientID}
		return true, nil
	})
}

func (p *Plugin) CancelAuthorization() {
	p.mu.Lock()
	var cancel context.CancelFunc
	if p.auth != nil && p.auth.public.Status == "pending" {
		p.auth.public.Status = "cancelled"
		cancel = p.auth.cancel
	}
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
