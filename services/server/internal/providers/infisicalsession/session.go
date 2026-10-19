// Package infisicalsession stores provider sessions encrypted in management storage.
package infisicalsession

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
)

const DefaultSiteURL = "https://app.infisical.com"

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
type Manager struct {
	Store    *store.Store
	Root     string
	Local    bool
	mu       sync.Mutex
	location sync.Mutex
}
type managerKey struct{}

func WithManager(ctx context.Context, m *Manager) context.Context {
	return context.WithValue(ctx, managerKey{}, m)
}
func ManagerFrom(ctx context.Context) (*Manager, error) {
	m, ok := ctx.Value(managerKey{}).(*Manager)
	if !ok || m == nil {
		return nil, i18n.ForContext(ctx).Errorf("server.context_missing")
	}
	return m, nil
}
func Missing(ctx context.Context) error {
	return cliErrors.New(cliErrors.INFISICAL_AUTH_MISSING, i18n.ForContext(ctx).T("auth.login_required"))
}
func Load(ctx context.Context) (*Session, error) {
	m, err := ManagerFrom(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := m.Store.Secret(ctx, "infisical-session")
	if errors.Is(err, store.ErrNotFound) {
		return nil, Missing(ctx)
	}
	if err != nil {
		return nil, err
	}
	var s Session
	if json.Unmarshal(raw, &s) != nil || s.Token == "" {
		return nil, Missing(ctx)
	}
	s.Expired = s.ExpiresAt.IsZero() || !time.Now().Before(s.ExpiresAt)
	s.LoggedIn = !s.Expired
	return &s, nil
}
func Require(ctx context.Context) (*Session, error) {
	s, err := Load(ctx)
	if err != nil || s.Expired {
		if machine, e := machineSession(ctx); e == nil {
			return machine, nil
		}
	}
	if err != nil {
		return nil, err
	}
	if s.Expired {
		return nil, cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED, i18n.ForContext(ctx).T("auth.session_expired"))
	}
	return s, nil
}
func Status(ctx context.Context) (Info, error) {
	s, err := Load(ctx)
	var coded interface{ ErrorCode() string }
	if errors.As(err, &coded) && coded.ErrorCode() == string(cliErrors.INFISICAL_AUTH_MISSING) {
		return Info{}, nil
	}
	if err != nil {
		return Info{}, err
	}
	return s.Info, nil
}
func save(ctx context.Context, s *Session) error {
	m, err := ManagerFrom(ctx)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return m.Store.PutSecret(ctx, "infisical-session", raw)
}
func sessionLock(ctx context.Context, fn func() error) error {
	m, err := ManagerFrom(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return err
	}
	return fn()
}
func WithUnchanged(ctx context.Context, expected *Session, fn func() error) error {
	return sessionLock(ctx, func() error {
		current, err := Load(ctx)
		if err != nil {
			return err
		}
		if current.Expired {
			return Missing(ctx)
		}
		if expected == nil || current.SiteURL != expected.SiteURL || current.UserID != expected.UserID || current.OrganizationID != expected.OrganizationID || current.Token != expected.Token {
			return cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED, i18n.ForContext(ctx).T("auth.session_changed"))
		}
		return fn()
	})
}
func Logout(ctx context.Context) error {
	return sessionLock(ctx, func() error {
		m, err := ManagerFrom(ctx)
		if err != nil {
			return err
		}
		if err = m.Store.PutSetting(ctx, "login-generation", []byte(time.Now().String())); err != nil {
			return err
		}
		if err = m.Store.DeleteSecret(ctx, "infisical-machine"); err != nil {
			return err
		}
		return m.Store.DeleteSecret(ctx, "infisical-session")
	})
}
func ConfigPath(ctx context.Context, name string) (string, error) {
	m, err := ManagerFrom(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(m.Root, "state", name), nil
}
func ReadSetting(ctx context.Context, key string) ([]byte, error) {
	m, err := ManagerFrom(ctx)
	if err != nil {
		return nil, err
	}
	return m.Store.Setting(ctx, key)
}
func WriteSetting(ctx context.Context, key string, value []byte) error {
	m, err := ManagerFrom(ctx)
	if err != nil {
		return err
	}
	return m.Store.PutSetting(ctx, key, value)
}
func WithLocationLock(ctx context.Context, fn func() error) error {
	m, err := ManagerFrom(ctx)
	if err != nil {
		return err
	}
	m.location.Lock()
	defer m.location.Unlock()
	if err = ctx.Err(); err != nil {
		return err
	}
	return fn()
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

type requestFailure struct {
	cause error
	key   string
	args  []any
}

func (e *requestFailure) Error() string { return i18n.Tf(e.key, e.args...) }
func (e *requestFailure) Unwrap() error { return e.cause }

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
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return networkFailure(req, e)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return &requestFailure{cause: cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED, i18n.ForContext(ctx).T("infisical.relogin")), key: "infisical.relogin"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &requestFailure{cause: cliErrors.New(cliErrors.INFISICAL_API_ERROR, i18n.ForContext(ctx).Tf("auth.api_failed", resp.StatusCode)), key: "auth.api_failed", args: []any{resp.StatusCode}}
	}
	if out == nil {
		return nil
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out) != nil {
		return &requestFailure{cause: cliErrors.New(cliErrors.INFISICAL_API_ERROR, i18n.ForContext(ctx).T("auth.response_invalid")), key: "auth.response_invalid"}
	}
	return nil
}

func networkFailure(req *http.Request, err error) error {
	// net/http wraps transport failures in url.Error. Its URL may contain query
	// credentials, so render only the transport reason and a sanitized endpoint.
	detail := err
	for {
		var requestErr *url.Error
		if !errors.As(detail, &requestErr) {
			break
		}
		detail = requestErr.Err
	}
	endpoint := *req.URL
	endpoint.User, endpoint.RawQuery, endpoint.Fragment, endpoint.RawFragment = nil, "", "", ""
	endpoint.ForceQuery = false
	args := []any{req.Method, endpoint.String(), detail.Error()}
	return &requestFailure{
		cause: cliErrors.New(cliErrors.INFISICAL_NETWORK_ERROR, i18n.ForContext(req.Context()).Tf("auth.network_failed", args...)).WithCause(err),
		key:   "auth.network_failed",
		args:  args,
	}
}

func verifiedSession(ctx context.Context, site, token, email string) (*Session, error) {
	s := &Session{Info: Info{SiteURL: site, Email: email, LoggedIn: true}, Token: token}
	if token == "" {
		return nil, Missing(ctx)
	}
	if e := Request(ctx, s, http.MethodPost, "/api/v1/auth/checkAuth", nil, nil); e != nil {
		return nil, e
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, i18n.ForContext(ctx).Errorf("auth.token_format_invalid")
	}
	raw, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil {
		return nil, i18n.ForContext(ctx).Errorf("auth.token_format_invalid")
	}
	var claims struct {
		UserID            string `json:"userId"`
		OrganizationID    string `json:"organizationId"`
		SubOrganizationID string `json:"subOrganizationId"`
		Exp               int64  `json:"exp"`
	}
	if json.Unmarshal(raw, &claims) != nil || claims.UserID == "" || claims.Exp <= time.Now().Unix() {
		return nil, i18n.ForContext(ctx).Errorf("auth.token_expired")
	}
	s.UserID = claims.UserID
	s.OrganizationID = claims.OrganizationID
	if claims.SubOrganizationID != "" {
		s.OrganizationID = claims.SubOrganizationID
	}
	s.ExpiresAt = time.Unix(claims.Exp, 0).UTC()
	return s, nil
}
