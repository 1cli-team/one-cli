package provider

import (
	"context"
	oneclient "github.com/torchstellar-team/one-cli/packages/one-client"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"time"
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
type SharedCredentialsState struct {
	Status   string `json:"status"`
	Location any    `json:"location,omitempty"`
	Error    string `json:"error,omitempty"`
}
type State struct {
	Session           Info                    `json:"session"`
	Configuration     *Configuration          `json:"configuration,omitempty"`
	SharedCredentials *SharedCredentialsState `json:"sharedCredentials"`
	Login             *struct {
		Status string `json:"status"`
		URL    string `json:"url"`
	} `json:"login"`
	Error string `json:"error"`
}
type Configuration struct {
	Mode           string `json:"mode"`
	Source         string `json:"source"`
	Managed        bool   `json:"managed"`
	Method         string `json:"method"`
	SiteURL        string `json:"siteUrl,omitempty"`
	ClientID       string `json:"clientId,omitempty"`
	OrganizationID string `json:"organizationId,omitempty"`
}

func Status(ctx context.Context) (Info, error) {
	var s State
	err := oneclient.New().DefaultRequest(ctx, "GET", "/session", nil, &s)
	if err == nil && s.Error != "" {
		err = i18n.Errorf("server.provider_login_failed", s.Error)
	}
	return s.Session, err
}
func Logout(ctx context.Context) error {
	return oneclient.New().DefaultRequest(ctx, "DELETE", "/session", nil, nil)
}

type Attempt struct {
	URL    string
	ctx    context.Context
	cancel context.CancelFunc
	client *oneclient.Client
}

func Start(ctx context.Context, site string) (*Attempt, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	c := oneclient.New()
	var r struct {
		URL string `json:"url"`
	}
	if err := c.DefaultRequest(ctx, "POST", "/session/login", map[string]string{"siteUrl": site}, &r); err != nil {
		cancel()
		return nil, err
	}
	return &Attempt{URL: r.URL, ctx: ctx, cancel: cancel, client: c}, nil
}
func (a *Attempt) Cancel() {
	a.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = a.client.DefaultRequest(ctx, "DELETE", "/session/login", nil, nil)
}
func (a *Attempt) SubmitBrowserToken(token string) error {
	return a.client.DefaultRequest(a.ctx, "POST", "/session/login/token", map[string]string{"token": token}, nil)
}
func (a *Attempt) Result() (Info, error, bool) {
	var state State
	if err := a.client.DefaultRequest(a.ctx, "GET", "/session", nil, &state); err != nil {
		return Info{}, err, true
	}
	if state.Login != nil && state.Login.Status == "failed" {
		return Info{}, i18n.Errorf("server.provider_login_failed", state.Error), true
	}
	return state.Session, nil, state.Session.LoggedIn
}
func (a *Attempt) Wait() (Info, error) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return Info{}, a.ctx.Err()
		case <-ticker.C:
			if info, err, done := a.Result(); done {
				return info, err
			}
		}
	}
}
func PrepareAfterLogin(ctx context.Context, info Info) SharedCredentialsState {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		var state State
		if err := oneclient.New().DefaultRequest(ctx, "GET", "/session", nil, &state); err != nil {
			return SharedCredentialsState{Status: "failed", Error: err.Error()}
		}
		if state.SharedCredentials != nil && (state.SharedCredentials.Status == "ready" || state.SharedCredentials.Status == "failed") {
			return *state.SharedCredentials
		}
		select {
		case <-ctx.Done():
			return SharedCredentialsState{Status: "failed", Error: ctx.Err().Error()}
		case <-ticker.C:
		}
	}
}

func ConnectMachine(ctx context.Context, site, id, secret, organization string) (Info, error) {
	var response struct {
		Session Info `json:"session"`
	}
	err := oneclient.New().DefaultRequest(ctx, "POST", "/session/machine", map[string]string{"siteUrl": site, "clientId": id, "clientSecret": secret, "organizationId": organization}, &response)
	return response.Session, err
}
