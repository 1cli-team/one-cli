// Package env implements the Infisical-backed secrets workflow for One CLI.
// Workspace-level config (provider / projectId / environments / rootPath)
// lives in one.manifest.json#domains.env (kind="infisical") + the top-level
// environments section. Subproject-level overrides (path / inherits /
// disabled) live in the matching subproject's manifest entry under
// projects[].domains.env.
package infisical

import (
	"encoding/json"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// DefaultSiteURL is the public Infisical SaaS instance. Workspaces using a
// self-hosted instance must set siteUrl explicitly via `one configure add env/infisical --site-url`.
const DefaultSiteURL = "https://app.infisical.com"

// DefaultEnvironment is the canonical first environment every workspace
// gets.
const DefaultEnvironment = "dev"

// DefaultEnvironments is the canonical env list stamped into new Infisical
// workspaces. Order matters because defaultEnv resolution prefers the first
// matching entry.
var DefaultEnvironments = []string{"dev", "staging", "prod"}

// WorkspaceConfig is the runtime view of one.manifest.json#domains.env (when
// kind=infisical) merged with the workspace-level environments section.
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

	// ProfileName is the resolved env/infisical profile name powering
	// this config. Runtime-only (never persisted to manifest); set by
	// the resolver path so the SDK client can key its short-lived
	// access-token cache by (env, infisical, ProfileName).
	ProfileName string
}

// SubprojectConfig is the (optional) per-subproject override stored on the
// matching one.manifest.json subproject entry under projects[].domains.env.
type SubprojectConfig struct {
	// Path is the absolute Infisical folder path this subproject maps to.
	// Default: "/" + relativeDir (e.g. /services/user-api).
	Path string
	// Inherits controls whether the pull pipeline merges parent-folder
	// keys (root → ancestors → self) before applying the .env.example
	// filter. Default: true.
	Inherits *bool
	// Disabled is the explicit "this subproject doesn't consume secrets"
	// signal.
	Disabled bool
}

// manifestEnvConfig is the JSON shape persisted under
// `manifest.domains.env.config` when kind == "infisical". Backend-specific
// fields plus the shared workspace-tracked variable-name list.
type manifestEnvConfig struct {
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
// or the selected backend is not Infisical. Returns INFISICAL_NOT_CONFIGURED
// only when invoked via RequireWorkspaceConfig.
func LoadWorkspaceConfig(projectRoot string) (*WorkspaceConfig, error) {
	if !workspace.HasManifest(projectRoot) {
		return nil, nil
	}
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return nil, err
	}
	if m.Domains == nil || m.Domains.Env == nil {
		return nil, nil
	}
	if m.Domains.Env.Kind != workspace.EnvBackendInfisical {
		return nil, nil
	}
	cfg := &WorkspaceConfig{}
	if len(m.Domains.Env.Config) > 0 {
		var raw manifestEnvConfig
		if err := json.Unmarshal(m.Domains.Env.Config, &raw); err != nil {
			return nil, err
		}
		cfg.ProjectID = raw.ProjectID
		cfg.ProjectName = raw.ProjectName
		cfg.RootPath = raw.RootPath
		cfg.Keys = raw.Keys
	}
	if m.Environments != nil {
		cfg.Environments = append([]string{}, m.Environments.Names...)
		cfg.DefaultEnv = m.Environments.Default
	}
	return cfg, nil
}

// resolveCfgAndCreds is the v0.5+ adapter helper. Profile-level and
// manifest-level concerns are merged here:
//
//   - Manifest (one.manifest.json) is the source of truth for project-level
//     fields: ProjectID, ProjectName, Environments, DefaultEnv, RootPath.
//     Always read.
//   - Profile (~/.config/one/config.json + credentials.json) contributes
//     machine-level fields: SiteURL + credentials. When the cobra layer already
//     resolved a profile (cfgOverride / credsOverride non-nil),
//     they're applied directly. Otherwise we resolve here so the same
//     "profile is the only source" rule holds for callers (e.g. some
//     internal helpers) that don't go through envcmd.
//
// Splitting the scopes this way means a single profile drives many
// workspaces — each workspace pins its own projectId in its manifest;
// switching profile only switches "which Infisical instance + as
// whom", not "which project".
func resolveCfgAndCreds(projectRoot string, cfgOverride *WorkspaceConfig, credsOverride *Credentials) (*WorkspaceConfig, *Credentials, error) {
	cfg, err := RequireWorkspaceConfig(projectRoot)
	if err != nil {
		return nil, nil, err
	}
	if cfgOverride != nil {
		if strings.TrimSpace(cfgOverride.SiteURL) != "" {
			cfg.SiteURL = cfgOverride.SiteURL
		}
	}
	creds := credsOverride
	if creds == nil {
		// No upstream profile resolution — do it here. Errors with
		// INFISICAL_AUTH_MISSING when no profile is configured.
		profileName, c, siteURL, err := requireProfileCreds(projectRoot, "")
		if err != nil {
			return nil, nil, err
		}
		creds = c
		cfg.ProfileName = profileName
		if cfgOverride == nil && siteURL != "" {
			cfg.SiteURL = siteURL
		}
	} else if cfgOverride != nil && cfgOverride.ProfileName != "" {
		cfg.ProfileName = cfgOverride.ProfileName
	}
	return cfg, creds, nil
}

// RequireWorkspaceConfig is the strict variant: returns INFISICAL_NOT_CONFIGURED
// when no config is present. Use for command paths that depend on having
// Infisical wired up (set / get / list / pull / etc).
func RequireWorkspaceConfig(projectRoot string) (*WorkspaceConfig, error) {
	cfg, err := LoadWorkspaceConfig(projectRoot)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, cliErrors.New(cliErrors.INFISICAL_NOT_CONFIGURED,
			"未找到 Infisical 配置。请在 one.manifest.json#domains.env 中将 kind 设置为 \"infisical\"（机器级凭据通过 `one configure add env/infisical` 配置）。")
	}
	if strings.TrimSpace(cfg.ProjectID) == "" {
		return nil, cliErrors.New(cliErrors.INFISICAL_NOT_CONFIGURED,
			"当前工作区选择了 Infisical 但还没绑定项目（manifest.domains.env.config.projectId 为空）。"+
				"\n→ 确认已配置 `one configure add env/infisical --profile <name> --use`，"+
				"\n  然后重新运行 `one env get/set/list/pull` 触发 lazy auto-bind。"+
				"\n  （如果你只想用本地 .env，可以把 manifest.domains.env.kind 改成 \"dotenv\"。）")
	}
	return cfg, nil
}

// LoadSubprojectConfig reads the per-subproject env override from the
// matching one.manifest.json#projects[].domains.env entry. Returns
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
		if s.Domains == nil || s.Domains.Env == nil {
			return nil, nil
		}
		return &SubprojectConfig{
			Path:     s.Domains.Env.Path,
			Inherits: s.Domains.Env.Inherits,
			Disabled: s.Domains.Env.Disabled,
		}, nil
	}
	return nil, nil
}

// EncodeManifestConfig serialises the Infisical-specific config blob for
// storage under manifest.domains.env.config. Used by init.go when writing
// a freshly-resolved workspace setup back to disk.
func EncodeManifestConfig(cfg *WorkspaceConfig) (json.RawMessage, error) {
	raw := manifestEnvConfig{
		ProjectID:   cfg.ProjectID,
		ProjectName: cfg.ProjectName,
		RootPath:    cfg.RootPath,
		Keys:        cfg.Keys,
	}
	return json.Marshal(raw)
}
