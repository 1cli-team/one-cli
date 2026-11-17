// Package infisical installs the built-in Infisical CLI surface.
package infisical

import (
	"github.com/torchstellar-team/one-cli/packages/cli/internal/lifecycle"
	authcmd "github.com/torchstellar-team/one-cli/packages/cli/internal/plugins/infisical/auth"
	envcmd "github.com/torchstellar-team/one-cli/packages/cli/internal/plugins/infisical/env"
	core "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/infisical"
)

// The same integration supplies shared capabilities and CLI commands. Desktop
// composes the core plugin without depending on Cobra command implementations.
type Plugin struct{ *core.Plugin }

func New() *Plugin { return &Plugin{Plugin: core.New()} }

func (p *Plugin) Register(host lifecycle.Host) error {
	host.Root.AddCommand(authcmd.Commands()...)
	host.Root.AddCommand(envcmd.Commands(envcmd.Dependencies{Service: host.Dependencies.Environments})...)
	return nil
}

var _ lifecycle.Plugin = (*Plugin)(nil)
