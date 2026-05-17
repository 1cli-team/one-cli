package cloudflare

import (
	"encoding/json"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// ProjectConfig is the typed view over
// `projects[i].domains.deploy.config` when the per-project deploy backend
// is Cloudflare. WorkerName mirrors wrangler.toml#name (the deploy slug
// visible in dash.cloudflare.com); empty defers to whatever wrangler.toml
// or CLOUDFLARE_WORKER_NAME env var says. Env names the deploy target
// environment (drawn from manifest.environments.names; empty / "prod" →
// production deploy, anything else → wrangler named environment).
type ProjectConfig struct {
	WorkerName string `json:"workerName,omitempty"`
	Env        string `json:"env,omitempty"`
}

// DecodeProjectConfig pulls the Cloudflare-specific config blob out of
// the manifest's per-project deploy section. Returns (nil, nil) when no
// project with that name has a Cloudflare deploy section configured.
func DecodeProjectConfig(m *workspace.Manifest, projectName string) (*ProjectConfig, error) {
	if m == nil {
		return nil, nil
	}
	for _, p := range m.Projects {
		if p.Name != projectName {
			continue
		}
		if p.Domains == nil || p.Domains.Deploy == nil || p.Domains.Deploy.Kind != workspace.DeployBackendCloudflare {
			return nil, nil
		}
		if len(p.Domains.Deploy.Config) == 0 {
			return &ProjectConfig{}, nil
		}
		var cfg ProjectConfig
		if err := json.Unmarshal(p.Domains.Deploy.Config, &cfg); err != nil {
			return nil, err
		}
		return &cfg, nil
	}
	return nil, nil
}

// EncodeProjectConfig writes cfg back into projects[i].domains.deploy.config
// (in memory), creating the deploy section if necessary. Caller persists
// via WriteManifest.
func EncodeProjectConfig(p *workspace.ManifestProject, cfg *ProjectConfig) error {
	if p == nil {
		return nil
	}
	if p.Domains == nil {
		p.Domains = &workspace.ProjectDomains{}
	}
	if p.Domains.Deploy == nil {
		p.Domains.Deploy = &workspace.ProjectDeployBackend{Kind: workspace.DeployBackendCloudflare}
	}
	if cfg == nil {
		p.Domains.Deploy.Config = nil
		return nil
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	p.Domains.Deploy.Config = raw
	return nil
}
