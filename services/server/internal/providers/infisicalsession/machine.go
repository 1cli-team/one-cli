package infisicalsession

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type Machine struct {
	SiteURL        string `json:"siteUrl"`
	ClientID       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	OrganizationID string `json:"organizationId,omitempty"`
}

func ConnectMachine(ctx context.Context, machine Machine) (Info, error) {
	manager, err := ManagerFrom(ctx)
	if err != nil {
		return Info{}, err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	s, err := authenticateMachine(ctx, machine)
	if err != nil {
		return Info{}, err
	}
	data, err := json.Marshal(machine)
	if err != nil {
		return Info{}, err
	}
	if err = manager.Store.PutSecret(ctx, "infisical-machine", data); err != nil {
		return Info{}, err
	}
	if err = save(ctx, s); err != nil {
		return Info{}, err
	}
	return s.Info, nil
}

func machineSession(ctx context.Context) (*Session, error) {
	manager, err := ManagerFrom(ctx)
	if err != nil {
		return nil, err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if s, err := Load(ctx); err == nil && !s.Expired {
		return s, nil
	}
	data, err := manager.Store.Secret(ctx, "infisical-machine")
	if err != nil {
		return nil, err
	}
	var machine Machine
	if err = json.Unmarshal(data, &machine); err != nil {
		return nil, err
	}
	s, err := authenticateMachine(ctx, machine)
	if err != nil {
		return nil, err
	}
	if err = save(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

func authenticateMachine(ctx context.Context, machine Machine) (*Session, error) {
	site, err := NormalizeSite(machine.SiteURL)
	if err != nil {
		return nil, err
	}
	if machine.ClientID == "" || machine.ClientSecret == "" {
		return nil, i18n.ForContext(ctx).Errorf("server.machine_auth_required")
	}
	body, err := json.Marshal(map[string]string{"clientId": machine.ClientID, "clientSecret": machine.ClientSecret})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", site+"/api/v1/auth/universal-auth/login", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return nil, networkFailure(req, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, i18n.ForContext(ctx).Errorf("auth.api_failed", response.StatusCode)
	}
	var token struct {
		AccessToken string `json:"accessToken"`
		ExpiresIn   int64  `json:"expiresIn"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&token) != nil || token.AccessToken == "" || token.ExpiresIn <= 0 {
		return nil, i18n.ForContext(ctx).Errorf("auth.response_invalid")
	}
	organization := machine.OrganizationID
	parts := strings.Split(token.AccessToken, ".")
	if len(parts) == 3 {
		if data, e := base64.RawURLEncoding.DecodeString(parts[1]); e == nil {
			var claims struct {
				OrganizationID string `json:"organizationId"`
			}
			if json.Unmarshal(data, &claims) == nil && organization == "" {
				organization = claims.OrganizationID
			}
		}
	}
	return &Session{Info: Info{LoggedIn: true, SiteURL: site, UserID: machine.ClientID, OrganizationID: organization, ExpiresAt: time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)}, Token: token.AccessToken}, nil
}
