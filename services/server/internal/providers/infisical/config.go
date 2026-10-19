package infisical

import (
	"context"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
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

// Management supplies the complete binding; provider operations never inspect a checkout.
func resolveCfgAndCreds(ctx context.Context, projectRoot string, cfg *WorkspaceConfig, creds *Credentials) (*WorkspaceConfig, *Credentials, error) {
	if cfg == nil || strings.TrimSpace(cfg.ProjectID) == "" {
		return nil, nil, cliErrors.New(cliErrors.INFISICAL_NOT_CONFIGURED, i18n.ForContext(ctx).T("infisical.binding_missing"))
	}
	if creds == nil {
		c, site, err := sessionCredentials(ctx)
		if err != nil {
			return nil, nil, err
		}
		if cfg.SiteURLOrDefault() != site {
			return nil, nil, i18n.ForContext(ctx).Errorf("infisical.binding_instance_mismatch")
		}
		creds = c
	}
	return cfg, creds, nil
}
