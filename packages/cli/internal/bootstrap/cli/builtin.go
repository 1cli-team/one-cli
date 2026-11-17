package cli

import (
	"context"
	"sync"

	"github.com/spf13/cobra"
	gitplugin "github.com/torchstellar-team/one-cli/packages/cli/internal/plugins/git"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/plugins/infisical"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func defaultApplication() (*Application, error) {
	app, err := NewApplication(infisical.New(), gitplugin.New())
	if err == nil {
		app.rootHelp = func() string { return i18n.T("root.help") }
	}
	return app, err
}

func newRootCommand() *cobra.Command {
	app, err := defaultApplication()
	if err != nil {
		panic(err)
	}
	return app.root
}

// Construct stores only after storage initialization has validated ONE_HOME.
var rootCommand = sync.OnceValue(newRootCommand)

// RootCmd returns the command tree for read-only tooling without changing user data.
func RootCmd() *cobra.Command { return rootCommand() }

func Execute(version string, args []string) error {
	app, err := defaultApplication()
	if err != nil {
		return emitError(err)
	}
	return app.Execute(context.Background(), version, args)
}
