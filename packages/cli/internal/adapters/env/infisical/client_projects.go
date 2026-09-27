package infisical

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// CreateProject calls Infisical's POST /api/v2/workspace endpoint to create
// a new secret-manager project named `projectName`. It does not use the
// Infisical Go SDK because the SDK's public surface is secrets-only — we
// reuse the active browser session access token
// and issue the HTTP request directly.
//
// Returned (id, resolvedName) reflect what Infisical actually accepted; the
// caller may have to retry with a suffix when the API surfaces a name
// collision (INFISICAL_PROJECT_NAME_TAKEN).
func (c *Client) CreateProject(projectName string) (string, string, error) {
	return c.CreateProjectContext(context.Background(), projectName)
}

func (c *Client) CreateProjectContext(ctx context.Context, projectName string) (string, string, error) {
	token := c.accessToken
	if token == "" && c.sdk != nil {
		token = c.sdk.Auth().GetAccessToken()
	}
	if token == "" {
		return "", "", cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED,
			i18n.T("infisical.token_missing"))
	}

	body, err := json.Marshal(map[string]any{
		"projectName": projectName,
		"type":        "secret-manager",
	})
	if err != nil {
		return "", "", err
	}

	url := strings.TrimRight(c.cfg.SiteURLOrDefault(), "/") + "/api/v2/workspace"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "one-cli/"+clientVersion)

	httpClient := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := httpClient.Do(req)
	if err != nil {
		if isNetworkError(err) {
			return "", "", cliErrors.New(cliErrors.INFISICAL_NETWORK_ERROR,
				i18n.T("infisical.unreachable"))
		}
		return "", "", cliErrors.New(cliErrors.INFISICAL_API_ERROR, i18n.T("infisical.request_error"))
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		var parsed struct {
			Project struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"project"`
		}
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return "", "", cliErrors.New(cliErrors.INFISICAL_API_ERROR,
				i18n.Tf("infisical.create_response_invalid", err.Error()))
		}
		if parsed.Project.ID == "" {
			return "", "", cliErrors.New(cliErrors.INFISICAL_API_ERROR,
				i18n.T("infisical.create_missing_id"))
		}
		name := parsed.Project.Name
		if name == "" {
			name = projectName
		}
		return parsed.Project.ID, name, nil

	case resp.StatusCode == http.StatusForbidden:
		return "", "", cliErrors.New(cliErrors.INFISICAL_PROJECT_CREATE_FORBIDDEN,
			i18n.T("infisical.create_forbidden"))

	case resp.StatusCode == http.StatusUnauthorized:
		return "", "", cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED,
			i18n.T("infisical.token_rejected"))

	default:
		// Look for known name-collision signals in the body before falling
		// back to the generic API error code so the init flow can branch on
		// INFISICAL_PROJECT_NAME_TAKEN and retry with a suffix.
		lower := strings.ToLower(string(respBody))
		if resp.StatusCode == http.StatusConflict ||
			strings.Contains(lower, "already exists") ||
			strings.Contains(lower, "name is already taken") ||
			strings.Contains(lower, "duplicate") {
			return "", "", cliErrors.New(cliErrors.INFISICAL_PROJECT_NAME_TAKEN,
				i18n.Tf("infisical.project_name_taken", projectName)).
				WithContext(map[string]any{"status": resp.StatusCode})
		}
		return "", "", cliErrors.New(cliErrors.INFISICAL_API_ERROR,
			i18n.Tf("infisical.create_failed", resp.StatusCode)).
			WithContext(map[string]any{"status": resp.StatusCode})
	}
}
