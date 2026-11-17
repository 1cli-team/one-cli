package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
)

type tokenResult struct {
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	ExpiresIn    int64           `json:"expires_in"`
	Scope        json.RawMessage `json:"scope,omitempty"`
}

func (r tokenResult) canWrite() bool {
	var scope string
	if json.Unmarshal(r.Scope, &scope) == nil {
		return slices.Contains(strings.Fields(scope), "write")
	}
	var scopes []string
	_ = json.Unmarshal(r.Scope, &scopes)
	return slices.Contains(scopes, "write")
}

func (p *Plugin) exchange(ctx context.Context, values url.Values) (tokenResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return tokenResult{}, tracking.Fail("config")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := p.client.Do(req)
	if err != nil {
		return tokenResult{}, tracking.Fail("network")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return tokenResult{}, tracking.Fail("reconnect")
	}
	var result tokenResult
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result) != nil || result.AccessToken == "" || result.RefreshToken == "" || result.ExpiresIn <= 0 {
		return tokenResult{}, tracking.Fail("remote")
	}
	return result, nil
}

func (p *Plugin) token(ctx context.Context, id string) (string, error) {
	var token string
	err := p.transaction(ctx, func(doc *document) (bool, error) {
		current, ok := doc.Connections[id]
		if !ok {
			return false, tracking.Fail("connection")
		}
		if time.Until(current.ExpiresAt) > time.Minute {
			token = current.AccessToken
			return false, nil
		}
		result, err := p.exchange(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {current.RefreshToken}, "client_id": {current.ClientID}})
		if err != nil {
			return false, err
		}
		current.AccessToken = result.AccessToken
		current.RefreshToken = result.RefreshToken
		current.ExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
		doc.Connections[id] = current
		token = current.AccessToken
		return true, nil
	})
	return token, err
}

func (p *Plugin) graph(ctx context.Context, connection, query string, variables any, out any) error {
	token, err := p.token(ctx, connection)
	if err != nil {
		return err
	}
	return p.graphWithToken(ctx, token, query, variables, out)
}

func (p *Plugin) graphWithToken(ctx context.Context, token, query string, variables any, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(body))
	if err != nil {
		return tracking.Fail("config")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	response, err := p.client.Do(req)
	if err != nil {
		return tracking.Fail("network")
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case 401:
		return tracking.Fail("reconnect")
	case 403:
		return tracking.Fail("forbidden")
	case 429:
		return tracking.Fail("rate_limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return tracking.Fail("remote")
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Extensions struct {
				Code string `json:"code"`
			} `json:"extensions"`
		} `json:"errors"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&envelope) != nil {
		return tracking.Fail("remote")
	}
	if len(envelope.Errors) > 0 {
		switch envelope.Errors[0].Extensions.Code {
		case "RATELIMITED":
			return tracking.Fail("rate_limit")
		case "FORBIDDEN":
			return tracking.Fail("forbidden")
		case "AUTHENTICATION_ERROR", "UNAUTHENTICATED":
			return tracking.Fail("reconnect")
		case "ENTITY_NOT_FOUND", "NOT_FOUND":
			return tracking.Fail("not_found")
		}
		return tracking.Fail("remote")
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" || json.Unmarshal(envelope.Data, out) != nil {
		return tracking.Fail("remote")
	}
	return nil
}
