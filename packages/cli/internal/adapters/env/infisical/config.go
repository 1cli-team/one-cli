package infisical

import (
	"encoding/json"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// DefaultSiteURL is the public Infisical SaaS instance. Workspaces using a
// self-hosted instance must set siteUrl explicitly via `one login --site-url`.
const DefaultSiteURL = "https://app.infisical.com"

// DefaultEnvironment is the canonical first environment every workspace
// gets.
const DefaultEnvironment = "dev"

// DefaultEnvironments matches the default Infisical project slugs.
var DefaultEnvironments = []string{"dev", "staging", "prod"}

// WorkspaceConfig holds the remote binding and transient remote display metadata.
type WorkspaceConfig struct {
	ProjectID    string
	ProjectName  string
	SiteURL      string
	Environments []string
}

// SiteURLOrDefault returns the configured Infisical instance, falling back
// to the public SaaS endpoint when the workspace did not specify one.
func (c *WorkspaceConfig) SiteURLOrDefault() string {
	if strings.TrimSpace(c.SiteURL) == "" {
		return DefaultSiteURL
	}
	return c.SiteURL
}

// DefaultEnvOrFallback is fixed; --env selects a different environment per operation.
func (c *WorkspaceConfig) DefaultEnvOrFallback() string { return "dev" }

func (c *WorkspaceConfig) RootPathOrDefault() string { return "/" }

// LoadWorkspaceConfig reads the workspace's Infisical config from the
// manifest. Returns nil config (no error) when no env backend is selected
// Returns INFISICAL_NOT_CONFIGURED
// only when invoked via RequireWorkspaceConfig.
func LoadWorkspaceConfig(projectRoot string) (*WorkspaceConfig, error) {
	if !workspace.HasManifest(projectRoot) {
		return nil, nil
	}
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return nil, err
	}
	if m.Env == nil {
		return nil, nil
	}
	cfg := &WorkspaceConfig{SiteURL: m.Env.SiteURL, ProjectID: m.Env.ProjectID, Environments: workspace.EnvironmentNames(m)}
	cfg.SiteURL = cfg.SiteURLOrDefault()

	return cfg, nil
}

// resolveCfgAndCreds combines manifest project metadata with the active session.
func resolveCfgAndCreds(projectRoot string, cfgOverride *WorkspaceConfig, credsOverride *Credentials) (*WorkspaceConfig, *Credentials, error) {
	cfg, err := RequireWorkspaceConfig(projectRoot)
	if err != nil {
		return nil, nil, err
	}
	if cfgOverride != nil {
		if strings.TrimSpace(cfgOverride.SiteURL) != "" {
			if cfg.SiteURL != "" && cfg.SiteURL != cfgOverride.SiteURL {
				return nil, nil, cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED, i18n.T("infisical.binding_account_mismatch"))
			}
			cfg.SiteURL = cfgOverride.SiteURL
		}
	}
	creds := credsOverride
	if creds == nil {
		// Resolve the browser session when the caller did not provide credentials.
		c, siteURL, err := sessionCredentials()
		if err != nil {
			return nil, nil, err
		}
		creds = c
		if cfgOverride == nil && siteURL != "" {
			if cfg.SiteURL != "" && cfg.SiteURL != siteURL {
				return nil, nil, cliErrors.New(cliErrors.INFISICAL_AUTH_FAILED, i18n.T("infisical.binding_instance_mismatch"))
			}
			cfg.SiteURL = siteURL
		}
	}
	return cfg, creds, nil
}

// RequireWorkspaceConfig is the strict variant: returns INFISICAL_NOT_CONFIGURED
// when no config is present. Use for command paths that depend on having
// Infisical wired up (set / list).
func RequireWorkspaceConfig(projectRoot string) (*WorkspaceConfig, error) {
	cfg, err := LoadWorkspaceConfig(projectRoot)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, cliErrors.New(cliErrors.INFISICAL_NOT_CONFIGURED,
			i18n.T("infisical.config_missing"))
	}
	if strings.TrimSpace(cfg.ProjectID) == "" {
		return nil, cliErrors.New(cliErrors.INFISICAL_NOT_CONFIGURED,
			i18n.T("infisical.binding_missing"))
	}
	return cfg, nil
}

// EncodeManifestConfig contains only persisted binding fields.
func EncodeManifestConfig(cfg *WorkspaceConfig) (json.RawMessage, error) {
	return json.Marshal(workspace.EnvironmentConfig{SiteURL: cfg.SiteURL, ProjectID: cfg.ProjectID, Environments: cfg.Environments})
}
