package bootstrap

import (
	gitplugin "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/infisical"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/linear"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/openviking"
)

// Compose preserves the default product composition for CLI and Desktop.
func Compose() Dependencies {
	deps, err := ComposePlugins(infisical.New(), gitplugin.New(), linear.New(), openviking.New())
	if err != nil {
		panic(err)
	}
	return deps
}
