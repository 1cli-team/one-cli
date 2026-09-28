// Package infisical implements the Infisical-backed secrets workflow for One CLI.
// Workspace-level config (provider / projectId / environments / rootPath)
// lives in one.manifest.json#env + the top-level
// environments section. Subproject-level overrides (path / inherits /
// disabled) live in the matching subproject's manifest entry under
// projects[].env.
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

// DefaultEnvironments is the canonical env list stamped into new Infisical
// workspaces. Order matters because defaultEnv resolution prefers the first
// matching entry.
var DefaultEnvironments = []string{"dev", "preview", "prod"}

// WorkspaceConfig is the runtime view of one.manifest.json#env merged with the workspace-level environments section.
//
// ProjectName mirrors the Infisical-side display name that auto-bind resolved
// to (matters when the auto-create flow appended a collision suffix).
// Persisting it lets us surface the actual name back to the user without an
// extra round trip to Infisical.
type WorkspaceConfig struct {
	ProjectID    string
	ProjectName  string
	SiteURL      string
	Environments []string
	DefaultEnv   string
	RootPath     string
	Keys         []string
}

// SubprojectConfig is the (optional) per-subproject override stored on the
// matching one.manifest.json subproject entry under projects[].env.
type SubprojectConfig struct {
	// Path is the absolute Infisical folder path this subproject maps to.
	// Default: "/" + relativeDir (e.g. /services/user-api).
	Path string
	// Inherits controls whether injection merges parent-folder keys. Default: true.
	Inherits *bool
	// Disabled is the explicit "this subproject doesn't consume secrets"
	// signal.
	Disabled bool
}

// manifestEnvConfig is the JSON shape persisted under
// `manifest.env`, including the shared variable-name list.
type manifestEnvConfig struct {
	SiteURL     string   `json:"siteUrl,omitempty"`
	ProjectID   string   `json:"projectId,omitempty"`
	ProjectName string   `json:"projectName,omitempty"`
	RootPath    string   `json:"rootPath,omitempty"`
	Keys        []string `json:"keys,omitempty"`
}

// SiteURLOrDefault returns the configured Infisical instance, falling back
// to the public SaaS endpoint when the workspace did not specify one.
func (c *WorkspaceConfig) SiteURLOrDefault() string {
	if strings.TrimSpace(c.SiteURL) == "" {
		return DefaultSiteURL
	}
	return c.SiteURL
}

// DefaultEnvOrFallback returns the configured default environment, falling
// back to "dev" when neither defaultEnv nor environments[0] are set.
func (c *WorkspaceConfig) DefaultEnvOrFallback() string {
	if strings.TrimSpace(c.DefaultEnv) != "" {
		return c.DefaultEnv
	}
	if len(c.Environments) > 0 {
		return c.Environments[0]
	}
	return DefaultEnvironment
}

// RootPathOrDefault returns the workspace-level root folder inside the
// Infisical project. Defaults to "/".
func (c *WorkspaceConfig) RootPathOrDefault() string {
	if strings.TrimSpace(c.RootPath) == "" {
		return "/"
	}
	return c.RootPath
}

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
	cfg := &WorkspaceConfig{SiteURL: m.Env.SiteURL, ProjectID: m.Env.ProjectID, ProjectName: m.Env.ProjectName, RootPath: m.Env.RootPath, Keys: append([]string(nil), m.Env.Keys...)}

	if m.Environments != nil {
		cfg.Environments = append([]string{}, m.Environments.Names...)
		cfg.DefaultEnv = m.Environments.Default
	}
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
// Infisical wired up (set / get / list).
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

// LoadSubprojectConfig reads the per-subproject env override from the
// matching one.manifest.json#projects[].env entry. Returns
// (nil, nil) when the manifest is missing, the entry doesn't exist, or no
// override is set.
func LoadSubprojectConfig(projectRoot, relativeDir string) (*SubprojectConfig, error) {
	if !workspace.HasManifest(projectRoot) {
		return nil, nil
	}
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return nil, err
	}
	rel := workspace.ToPosixPath(relativeDir)
	for _, s := range m.Projects {
		if s.RelativeDir != rel {
			continue
		}
		if s.Env == nil {
			return nil, nil
		}
		return &SubprojectConfig{
			Path:     s.Env.Path,
			Inherits: s.Env.Inherits,
			Disabled: s.Env.Disabled,
		}, nil
	}
	return nil, nil
}

// EncodeManifestConfig serialises the Infisical-specific config blob for
// storage under manifest.env. Used by init.go when writing
// a freshly-resolved workspace setup back to disk.
func EncodeManifestConfig(cfg *WorkspaceConfig) (json.RawMessage, error) {
	raw := manifestEnvConfig{
		SiteURL:     cfg.SiteURL,
		ProjectID:   cfg.ProjectID,
		ProjectName: cfg.ProjectName,
		RootPath:    cfg.RootPath,
		Keys:        cfg.Keys,
	}
	return json.Marshal(raw)
}
