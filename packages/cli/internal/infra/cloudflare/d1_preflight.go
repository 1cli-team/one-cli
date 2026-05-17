package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

var (
	cloudflareAPIBaseURL = "https://api.cloudflare.com/client/v4"
	cloudflareHTTPClient = &http.Client{Timeout: 30 * time.Second}
)

type wranglerD1Config struct {
	D1Databases []d1DatabaseBinding `toml:"d1_databases"`
}

type d1DatabaseBinding struct {
	Binding      string `toml:"binding"`
	DatabaseName string `toml:"database_name"`
	DatabaseID   string `toml:"database_id"`
}

type cloudflareEnvelope[T any] struct {
	Success bool                    `json:"success"`
	Errors  []cloudflareAPIResponse `json:"errors"`
	Result  T                       `json:"result"`
}

type cloudflareAPIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cloudflareD1Database struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

func preflightD1DatabaseBindings(ctx context.Context, projectDir, apiToken, accountID string) error {
	bindings, err := readD1DatabaseBindings(projectDir)
	if err != nil {
		return err
	}
	if len(bindings) == 0 {
		return nil
	}
	accountID, err = resolveD1PreflightAccountID(ctx, apiToken, accountID)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		if err := validateD1BindingShape(binding); err != nil {
			return err
		}
		db, err := fetchD1Database(ctx, accountID, binding.DatabaseID, apiToken)
		if err != nil {
			return d1BindingError(
				fmt.Sprintf("D1 binding %q 指向的 database_id 无法在 Cloudflare 中确认。", binding.Binding),
				binding,
				map[string]any{"account_id": accountID, "api_error": err.Error()},
			)
		}
		if db.Name != "" && binding.DatabaseName != "" && db.Name != binding.DatabaseName {
			return d1BindingError(
				fmt.Sprintf("D1 binding %q 的 database_name 与 Cloudflare 中的数据库不一致。", binding.Binding),
				binding,
				map[string]any{
					"account_id":           accountID,
					"actual_database_name": db.Name,
				},
			)
		}
	}
	return nil
}

func readD1DatabaseBindings(projectDir string) ([]d1DatabaseBinding, error) {
	raw, err := os.ReadFile(filepath.Join(projectDir, WranglerConfigFilename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cfg wranglerD1Config
	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return nil, cliErrors.New(cliErrors.CLOUDFLARE_DEPLOY_FAILED,
			"wrangler.toml 解析失败，无法校验 Cloudflare D1 binding。").
			WithContext(map[string]any{"project_dir": projectDir, "parse_error": err.Error()}).
			WithRemediation(output.Remediation{
				Action: "fix-wrangler-toml",
				Hint:   "检查 wrangler.toml 语法，特别是 [[d1_databases]] 块",
			})
	}
	return cfg.D1Databases, nil
}

func resolveD1PreflightAccountID(ctx context.Context, apiToken, accountID string) (string, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID != "" {
		return accountID, nil
	}
	accounts, err := listCloudflareAccounts(ctx, apiToken)
	if err != nil {
		return "", cliErrors.New(cliErrors.CLOUDFLARE_PROFILE_INVALID,
			"wrangler.toml 配置了 D1，但 Cloudflare profile 没有 accountId，且 One CLI 无法自动解析账号。").
			WithContext(map[string]any{"api_error": err.Error()}).
			WithRemediation(output.Remediation{
				Action:  "set-account-id",
				Hint:    "给 Cloudflare profile 补上 Account ID 后重试",
				Command: "one configure add deploy/cloudflare cf-prod --use --account-id <account-id>",
			})
	}
	if len(accounts) != 1 {
		return "", cliErrors.New(cliErrors.CLOUDFLARE_PROFILE_INVALID,
			"wrangler.toml 配置了 D1，但当前 token 没有唯一 Cloudflare account 可用于校验。").
			WithContext(map[string]any{"accounts_count": len(accounts)}).
			WithRemediation(output.Remediation{
				Action:  "set-account-id",
				Hint:    "多账号或无法自动判断账号时，需要在 Cloudflare profile 里写入 Account ID",
				Command: "one configure add deploy/cloudflare cf-prod --use --account-id <account-id>",
			})
	}
	return accounts[0].ID, nil
}

func validateD1BindingShape(binding d1DatabaseBinding) error {
	missing := make([]string, 0, 3)
	if strings.TrimSpace(binding.Binding) == "" {
		missing = append(missing, "binding")
	}
	if strings.TrimSpace(binding.DatabaseName) == "" {
		missing = append(missing, "database_name")
	}
	if strings.TrimSpace(binding.DatabaseID) == "" {
		missing = append(missing, "database_id")
	}
	if len(missing) == 0 {
		return nil
	}
	return d1BindingError("wrangler.toml 的 D1 binding 缺少必要字段。", binding, map[string]any{"missing_fields": missing})
}

func listCloudflareAccounts(ctx context.Context, apiToken string) ([]cloudflareAccount, error) {
	endpoint := cloudflareEndpoint("/accounts")
	q := endpoint.Query()
	q.Set("per_page", "50")
	endpoint.RawQuery = q.Encode()
	var env cloudflareEnvelope[[]cloudflareAccount]
	if err := cloudflareGet(ctx, endpoint.String(), apiToken, &env); err != nil {
		return nil, err
	}
	return env.Result, nil
}

func fetchD1Database(ctx context.Context, accountID, databaseID, apiToken string) (cloudflareD1Database, error) {
	path := fmt.Sprintf("/accounts/%s/d1/database/%s", url.PathEscape(accountID), url.PathEscape(databaseID))
	var env cloudflareEnvelope[cloudflareD1Database]
	if err := cloudflareGet(ctx, cloudflareEndpoint(path).String(), apiToken, &env); err != nil {
		return cloudflareD1Database{}, err
	}
	if env.Result.UUID == "" {
		env.Result.UUID = databaseID
	}
	return env.Result, nil
}

func cloudflareGet(ctx context.Context, endpoint, apiToken string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	resp, err := cloudflareHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var raw cloudflareEnvelope[json.RawMessage]
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !raw.Success {
		return fmt.Errorf("cloudflare api %s returned status %d: %s", endpoint, resp.StatusCode, cloudflareErrorMessages(raw.Errors))
	}
	if out == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func cloudflareEndpoint(path string) *url.URL {
	endpoint, _ := url.Parse(strings.TrimRight(cloudflareAPIBaseURL, "/") + "/" + strings.TrimLeft(path, "/"))
	return endpoint
}

func cloudflareErrorMessages(errors []cloudflareAPIResponse) string {
	if len(errors) == 0 {
		return "unknown error"
	}
	msgs := make([]string, 0, len(errors))
	for _, item := range errors {
		if item.Message != "" {
			msgs = append(msgs, item.Message)
		}
	}
	if len(msgs) == 0 {
		return "unknown error"
	}
	return strings.Join(msgs, "; ")
}

func d1BindingError(message string, binding d1DatabaseBinding, extra map[string]any) *output.Error {
	ctx := map[string]any{
		"binding":       binding.Binding,
		"database_name": binding.DatabaseName,
		"database_id":   binding.DatabaseID,
	}
	for k, v := range extra {
		ctx[k] = v
	}
	return cliErrors.New(cliErrors.CLOUDFLARE_DEPLOY_FAILED, message).
		WithContext(ctx).
		WithRemediation(
			output.Remediation{
				Action: "check-d1-database-id",
				Hint:   "确认 wrangler.toml 里的 database_id 来自当前 Cloudflare account 下的 D1 数据库",
			},
			output.Remediation{
				Action:  "list-d1-databases",
				Hint:    "查看当前账号下的 D1 数据库 ID",
				Command: "pnpm exec wrangler d1 list",
			},
		)
}
