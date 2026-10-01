// Package infisicalsession owns One's single browser-authenticated user session.
// Tokens live exclusively in the OS keyring, never in JSON output or manifests.
package infisicalsession

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/zalando/go-keyring"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/preferences"
)

const DefaultSiteURL = "https://app.infisical.com"
const service = "one-cli.infisical"
const account = "session"

type Info struct {
	LoggedIn       bool      `json:"loggedIn"`
	SiteURL        string    `json:"siteUrl,omitempty"`
	Email          string    `json:"email,omitempty"`
	UserID         string    `json:"userId,omitempty"`
	OrganizationID string    `json:"organizationId,omitempty"`
	ExpiresAt      time.Time `json:"expiresAt,omitempty"`
	Expired        bool      `json:"expired"`
}
type Session struct {
	Info
	Token string `json:"token"`
}

// Keyring functions are replaceable only inside package tests.
var getSecret = keyring.Get
var setSecret = keyring.Set
var deleteSecret = keyring.Delete

func Missing() error {
	return cliErrors.New(cliErrors.INFISICAL_AUTH_MISSING, i18n.T("auth.login_required"))
}
func Load() (*Session, error) {
	raw, err := getSecret(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, Missing()
	}
	if err != nil {
		return nil, i18n.Errorf("auth.store_read_failed", err)
	}
	var s Session
	if json.Unmarshal([]byte(raw), &s) != nil || s.Token == "" {
		return nil, Missing()
	}
	s.Expired = s.ExpiresAt.IsZero() || !time.Now().Before(s.ExpiresAt)
	s.LoggedIn = !s.Expired
	return &s, nil
}
func Require() (*Session, error) {
	s, err := Load()
	if err != nil {
		return nil, err
	}
	if s.Expired {
		return nil, cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED, i18n.T("auth.session_expired"))
	}
	return s, nil
}
func Status() (Info, error) {
	s, err := Load()
	var coded interface{ ErrorCode() string }
	if errors.As(err, &coded) && coded.ErrorCode() == string(cliErrors.INFISICAL_AUTH_MISSING) {
		return Info{}, nil
	}
	if err != nil {
		return Info{}, err
	}
	return s.Info, nil
}
func save(s *Session) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err = setSecret(service, account, string(raw)); err != nil {
		return i18n.Errorf("auth.store_write_failed", err)
	}
	return nil
}
func sessionLock(fn func() error) error {
	p, e := ConfigPath("session.lock")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	lock := flock.New(p)
	if e = lock.Lock(); e != nil {
		return e
	}
	defer lock.Unlock()
	return fn()
}

// WithUnchanged serializes a local commit with login and logout. Remote work
// must finish before calling it; fn must not acquire the session lock again.
func WithUnchanged(expected *Session, fn func() error) error {
	return sessionLock(func() error {
		current, err := Require()
		if err != nil {
			return err
		}
		if expected == nil || current.SiteURL != expected.SiteURL || current.UserID != expected.UserID ||
			current.OrganizationID != expected.OrganizationID || current.Token != expected.Token {
			return cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED, i18n.T("auth.session_changed"))
		}
		return fn()
	})
}

func Logout() error {
	return sessionLock(func() error {
		path, e := ConfigPath("login-generation")
		if e != nil {
			return e
		}
		if e = os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339Nano)), 0600); e != nil {
			return e
		}
		e = deleteSecret(service, account)
		if errors.Is(e, keyring.ErrNotFound) {
			return nil
		}
		return e
	})
}

func ConfigPath(name string) (string, error) {
	p, e := preferences.Path()
	return filepath.Join(filepath.Dir(p), name), e
}

func NormalizeSite(raw string) (string, error) {
	if raw == "" {
		raw = DefaultSiteURL
	}
	u, e := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", i18n.Errorf("auth.site_root_required")
	}
	local := u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return "", i18n.Errorf("auth.https_required")
	}
	u.Path = ""
	return u.String(), nil
}

// Request never follows redirects with bearer credentials, and never exposes
// upstream response bodies in errors: those can contain secret values.
func Request(ctx context.Context, s *Session, method, path string, body io.Reader, out any) error {
	req, e := http.NewRequestWithContext(ctx, method, s.SiteURL+path, body)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, e := client.Do(req)
	if e != nil {
		return cliErrors.New(cliErrors.INFISICAL_NETWORK_ERROR, i18n.T("auth.network_failed"))
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED, i18n.T("infisical.relogin"))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return cliErrors.New(cliErrors.INFISICAL_API_ERROR, i18n.Tf("auth.api_failed", resp.StatusCode))
	}
	if out == nil {
		return nil
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out) != nil {
		return cliErrors.New(cliErrors.INFISICAL_API_ERROR, i18n.T("auth.response_invalid"))
	}
	return nil
}

func verifiedSession(ctx context.Context, site, token, email string) (*Session, error) {
	s := &Session{Info: Info{SiteURL: site, Email: email, LoggedIn: true}, Token: token}
	if token == "" {
		return nil, Missing()
	}
	if e := Request(ctx, s, http.MethodPost, "/api/v1/auth/checkAuth", nil, nil); e != nil {
		return nil, e
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, i18n.Errorf("auth.token_format_invalid")
	}
	raw, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil {
		return nil, i18n.Errorf("auth.token_format_invalid")
	}
	var claims struct {
		UserID            string `json:"userId"`
		OrganizationID    string `json:"organizationId"`
		SubOrganizationID string `json:"subOrganizationId"`
		Exp               int64  `json:"exp"`
	}
	if json.Unmarshal(raw, &claims) != nil || claims.UserID == "" || claims.Exp <= time.Now().Unix() {
		return nil, i18n.Errorf("auth.token_expired")
	}
	s.UserID = claims.UserID
	s.OrganizationID = claims.OrganizationID
	if claims.SubOrganizationID != "" {
		s.OrganizationID = claims.SubOrganizationID
	}
	s.ExpiresAt = time.Unix(claims.Exp, 0).UTC()
	return s, nil
}
