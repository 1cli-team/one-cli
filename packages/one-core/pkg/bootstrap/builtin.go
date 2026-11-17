package bootstrap

import (
	gitplugin "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/infisical"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/linear"
)

// Compose preserves the default product composition for CLI and Desktop.
func Compose() Dependencies {
	deps, err := ComposePlugins(infisical.New(), gitplugin.New(), linear.New())
	if err != nil {
		panic(err)
	}
	return deps
}
