package infisical

import (
	"context"
	"io"
	"net"
	"regexp"
	"strings"

	infisical "github.com/infisical/go-sdk"
	"github.com/infisical/go-sdk/packages/models"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
)

// Client wraps the SDK with the current browser session.
type Client struct {
	sdk         infisical.InfisicalClientInterface
	cfg         *WorkspaceConfig
	credentials *Credentials
	accessToken string
}

func NewClient(ctx context.Context, cfg *WorkspaceConfig, creds *Credentials) (*Client, error) {
	if creds == nil || creds.AccessToken == "" {
		return nil, session.Missing()
	}
	autoRefresh := false
	sdk := infisical.NewInfisicalClient(ctx, infisical.Config{SiteUrl: cfg.SiteURLOrDefault(), UserAgent: "one-cli/" + clientVersion, SilentMode: true, LogWriter: io.Discard, AutoTokenRefresh: &autoRefresh})
	sdk.Auth().SetAccessToken(creds.AccessToken)
	return &Client{sdk: sdk, cfg: cfg, credentials: creds, accessToken: creds.AccessToken}, nil
}

// clientVersion is overridden at link-time via -ldflags. We don't bother
// reading the binary version because the cobra layer is the only place
// that has it, and the secrets layer doesn't need it for any decision.
var clientVersion = "0.0.0-dev"

// ListSecrets reads every key at the given path/environment, optionally
// recursively when the caller requests a folder subtree.
func (c *Client) ListSecrets(env, secretPath string, recursive bool) ([]models.Secret, error) {
	out, err := c.sdk.Secrets().List(infisical.ListSecretsOptions{
		ProjectID:              c.cfg.ProjectID,
		Environment:            env,
		SecretPath:             secretPath,
		ExpandSecretReferences: true,
		Recursive:              recursive,
	})
	if err != nil {
		return nil, mapAPIError(err)
	}
	return out, nil
}

// RetrieveSecret reads a single key. Returns ENV_KEY_NOT_FOUND when
// the key does not exist (mapped from the SDK's 404).
func (c *Client) RetrieveSecret(env, secretPath, key string) (*models.Secret, error) {
	out, err := c.sdk.Secrets().Retrieve(infisical.RetrieveSecretOptions{
		ProjectID:              c.cfg.ProjectID,
		Environment:            env,
		SecretPath:             secretPath,
		SecretKey:              key,
		ExpandSecretReferences: true,
	})
	if err != nil {
		mapped := mapAPIError(err)
		if isNotFound(err) {
			return nil, cliErrors.New(cliErrors.ENV_KEY_NOT_FOUND,
				i18n.Tf("env.key_missing", key))
		}
		return nil, mapped
	}
	return &out, nil
}

// folderStep is one (name, parent) pair the EnsureFolder walker emits
// per path segment. Extracted as a named type so the segment-splitting
// logic is unit-testable without an SDK mock.
type folderStep struct {
	Name   string
	Parent string
}

// folderStepsFor returns the ordered (name, parent) pairs needed to
// idempotently materialize secretPath as nested Infisical folders. Root
// produces an empty slice — the root always exists.
func folderStepsFor(secretPath string) []folderStep {
	norm := NormalizePath(secretPath)
	if norm == "/" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(norm, "/"), "/")
	out := make([]folderStep, 0, len(parts))
	parent := "/"
	for _, name := range parts {
		if name == "" {
			continue
		}
		out = append(out, folderStep{Name: name, Parent: parent})
		if parent == "/" {
			parent = "/" + name
		} else {
			parent = parent + "/" + name
		}
	}
	return out
}

// EnsureFolder makes sure every path segment in secretPath exists as a
// folder under the project's environment. Infisical's secret-create API
// requires the parent folder to exist; without this helper, the first
// `one env set -p some/new/dir KEY V` against a brand-new path
// surfaces a confusing 404. Segments are created top-down and "already
// exists" responses are swallowed so the helper is idempotent.
func (c *Client) EnsureFolder(env, secretPath string) error {
	for _, step := range folderStepsFor(secretPath) {
		_, err := c.sdk.Folders().Create(infisical.CreateFolderOptions{
			ProjectID:   c.cfg.ProjectID,
			Environment: env,
			Name:        step.Name,
			Path:        step.Parent,
		})
		if err != nil && !isAlreadyExistsError(err) {
			return mapAPIError(err)
		}
	}
	return nil
}

// CreateSecret inserts a new key. Caller should already know the key
// doesn't exist; if it does, Infisical returns 409 and we surface that
// as INFISICAL_API_ERROR (callers can then retry with Update).
func (c *Client) CreateSecret(env, secretPath, key, value string) (*models.Secret, error) {
	out, err := c.sdk.Secrets().Create(infisical.CreateSecretOptions{
		ProjectID:   c.cfg.ProjectID,
		Environment: env,
		SecretPath:  secretPath,
		SecretKey:   key,
		SecretValue: value,
	})
	if err != nil {
		return nil, mapAPIError(err)
	}
	return &out, nil
}

// UpdateSecret writes a new value to an existing key.
func (c *Client) UpdateSecret(env, secretPath, key, value string) (*models.Secret, error) {
	out, err := c.sdk.Secrets().Update(infisical.UpdateSecretOptions{
		ProjectID:      c.cfg.ProjectID,
		Environment:    env,
		SecretPath:     secretPath,
		SecretKey:      key,
		NewSecretValue: value,
	})
	if err != nil {
		return nil, mapAPIError(err)
	}
	return &out, nil
}

// DeleteSecret removes one key from an exact Infisical folder. A missing key
// is mapped to ENV_KEY_NOT_FOUND so Dashboard and CLI transports can present
// the same stable error contract as Get.
func (c *Client) DeleteSecret(env, secretPath, key string) (*models.Secret, error) {
	out, err := c.sdk.Secrets().Delete(infisical.DeleteSecretOptions{
		ProjectID:   c.cfg.ProjectID,
		Environment: env,
		SecretPath:  secretPath,
		SecretKey:   key,
	})
	if err != nil {
		if isNotFound(err) {
			return nil, cliErrors.New(cliErrors.ENV_KEY_NOT_FOUND, i18n.Tf("env.key_missing", key))
		}
		return nil, mapAPIError(err)
	}
	return &out, nil
}

// VerifyProjectExists is what `env init` calls after a successful
// auth to confirm the projectId is reachable. We use a List on env=dev,
// path="/" — which always exists if the project does — and translate
// 404 into a typed error.
func (c *Client) VerifyProjectExists(env string) error {
	_, err := c.sdk.Secrets().List(infisical.ListSecretsOptions{
		ProjectID:   c.cfg.ProjectID,
		Environment: env,
		SecretPath:  "/",
	})
	if err == nil {
		return nil
	}
	if isNotFound(err) {
		return cliErrors.New(cliErrors.INFISICAL_PROJECT_NOT_FOUND,
			i18n.Tf("infisical.project_missing", c.cfg.ProjectID)).
			WithContext(map[string]any{"project_id": c.cfg.ProjectID, "site_url": c.cfg.SiteURLOrDefault()})
	}
	return mapAPIError(err)
}

// ----- error helpers below -----

func mapAPIError(err error) error {
	if err == nil {
		return nil
	}
	if isNetworkError(err) {
		return cliErrors.New(cliErrors.INFISICAL_NETWORK_ERROR, i18n.T("infisical.network"))
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized") {
		return cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED, i18n.T("infisical.relogin"))
	}
	if folder, env := parseFolderNotFound(err); folder != "" {
		return cliErrors.New(cliErrors.INFISICAL_FOLDER_NOT_FOUND, i18n.Tf("infisical.folder_missing", env, folder))
	}
	return cliErrors.New(cliErrors.INFISICAL_API_ERROR, i18n.T("infisical.request_failed"))
}

// folderNotFoundRE matches Infisical's folder-404 message shape:
//
//	Folder with path '/apps' in environment 'prod' was not found.
//
// Captures (path, environment). The SDK error wraps this verbatim inside
// its multi-line APIError dump, so we just regex-search the whole string.
var folderNotFoundRE = regexp.MustCompile(`Folder with path '([^']+)' in environment '([^']+)'`)

func parseFolderNotFound(err error) (folder, env string) {
	if err == nil {
		return "", ""
	}
	m := folderNotFoundRE.FindStringSubmatch(err.Error())
	if len(m) != 3 {
		return "", ""
	}
	return m[1], m[2]
}

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(net.Error); ok {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "dial tcp")
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "404")
}

// isAlreadyExistsError reports whether an Infisical SDK error indicates the
// resource we tried to create is already present. Infisical signals this with
// an HTTP 400 + a message containing "already exists" / "duplicate" (the SDK
// surfaces it as a plain error). EnsureFolder swallows these so the helper
// stays idempotent across re-invocations.
func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exist") ||
		strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "conflict") ||
		strings.Contains(msg, "409")
}
