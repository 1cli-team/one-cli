package infisicalsession

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/torchstellar-team/one-cli/packages/one-client/provider"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
)

// Deployment is immutable startup configuration. Neither it nor its access
// tokens are persisted or returned in management responses.
type Deployment struct {
	Source  string
	Machine Machine
}

type Configuration = provider.Configuration

func (m *Manager) Managed() bool { return m != nil && m.Deployment != nil }

func Writable(ctx context.Context) error {
	m, err := ManagerFrom(ctx)
	if err != nil {
		return err
	}
	if m.Managed() {
		return cliErrors.New(cliErrors.SERVE_CONFIG_MANAGED, i18n.ForContext(ctx).T("server.infisical_managed"))
	}
	return nil
}

func ConfigurationFor(ctx context.Context) (Configuration, error) {
	m, err := ManagerFrom(ctx)
	if err != nil {
		return Configuration{}, err
	}
	cfg := Configuration{Mode: "remote", Source: "none", Method: "none"}
	if m.Local {
		cfg.Mode = "local"
	}
	var machine Machine
	if m.Managed() {
		cfg.Source, cfg.Managed, machine = m.Deployment.Source, true, m.Deployment.Machine
	} else {
		data, err := m.Store.Secret(ctx, "infisical-machine")
		if err == nil {
			if json.Unmarshal(data, &machine) != nil {
				return cfg, i18n.ForContext(ctx).Errorf("auth.response_invalid")
			}
			cfg.Source = "database"
		} else if !errors.Is(err, store.ErrNotFound) {
			return cfg, err
		} else if s, err := Load(ctx); err == nil {
			cfg.Source, cfg.Method, cfg.SiteURL = "database", "browser", s.SiteURL
			return cfg, nil
		}
	}
	if cfg.Source != "none" {
		cfg.Method, cfg.SiteURL, cfg.ClientID, cfg.OrganizationID = "machine", machine.SiteURL, machine.ClientID, machine.OrganizationID
	}
	return cfg, nil
}
