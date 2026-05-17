package infisical

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
)

// CreateProject calls Infisical's POST /api/v2/workspace endpoint to create
// a new secret-manager project named `projectName`. It does not use the
// Infisical Go SDK because the SDK's public surface is secrets-only — we
// reach into the access token the SDK already obtained via UniversalAuthLogin
// and issue the HTTP request directly.
//
// Returned (id, resolvedName) reflect what Infisical actually accepted; the
// caller may have to retry with a suffix when the API surfaces a name
// collision (INFISICAL_PROJECT_NAME_TAKEN).
func (c *Client) CreateProject(projectName string) (string, string, error) {
	token := c.accessToken
	if token == "" && c.sdk != nil {
		token = c.sdk.Auth().GetAccessToken()
	}
	if token == "" {
		return "", "", cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED,
			"Infisical access token 不可用，无法调用 create-project。")
	}

	body, err := json.Marshal(map[string]any{
		"projectName": projectName,
		"type":        "secret-manager",
	})
	if err != nil {
		return "", "", err
	}

	url := strings.TrimRight(c.cfg.SiteURLOrDefault(), "/") + "/api/v2/workspace"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "one-cli/"+clientVersion)

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		if isNetworkError(err) {
			return "", "", cliErrors.New(cliErrors.INFISICAL_NETWORK_ERROR,
				"无法连接到 Infisical："+err.Error())
		}
		return "", "", cliErrors.New(cliErrors.INFISICAL_API_ERROR, err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

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
				"Infisical create-project 响应解析失败："+err.Error()).
				WithContext(map[string]any{"body": string(respBody)})
		}
		if parsed.Project.ID == "" {
			return "", "", cliErrors.New(cliErrors.INFISICAL_API_ERROR,
				"Infisical create-project 响应缺少 project.id").
				WithContext(map[string]any{"body": string(respBody)})
		}
		name := parsed.Project.Name
		if name == "" {
			name = projectName
		}
		return parsed.Project.ID, name, nil

	case resp.StatusCode == http.StatusForbidden:
		return "", "", cliErrors.New(cliErrors.INFISICAL_PROJECT_CREATE_FORBIDDEN,
			"Infisical 拒绝创建项目（403）。机器身份缺少 create-project 权限。").
			WithContext(map[string]any{"body": string(respBody)})

	case resp.StatusCode == http.StatusUnauthorized:
		return "", "", cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED,
			"Infisical 拒绝了访问令牌（401）。").
			WithContext(map[string]any{"body": string(respBody)})

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
				"Infisical 项目名 "+projectName+" 已被占用").
				WithContext(map[string]any{"status": resp.StatusCode, "body": string(respBody)})
		}
		return "", "", cliErrors.New(cliErrors.INFISICAL_API_ERROR,
			fmt.Sprintf("Infisical create-project 失败（HTTP %d）", resp.StatusCode)).
			WithContext(map[string]any{"status": resp.StatusCode, "body": string(respBody)})
	}
}
