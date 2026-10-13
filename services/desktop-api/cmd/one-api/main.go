// Command one-api is the private desktop sidecar, never a public HTTP daemon.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/bootstrap"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/desktopcontrol"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/updatecheck"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/app"
	serve "github.com/torchstellar-team/one-cli/services/desktop-api/internal/http"
)

var version = "0.0.0-dev"
var commit = "unknown"

func main() {
	identity := desktopcontrol.Identity{Version: version, Commit: commit, Channel: updatecheck.Channel(), Protocol: desktopcontrol.Protocol, API: desktopcontrol.API}
	if len(os.Args) == 2 && os.Args[1] == "--identity" {
		_ = json.NewEncoder(os.Stdout).Encode(identity)
		return
	}
	if err := app.Run(identity, func() serve.MuxOpts {
		d := bootstrap.Compose()
		return serve.MuxOpts{Catalog: d.Catalog, ManifestService: d.Manifest, CreationService: d.Creation, EnvironmentService: d.Environments, WorkspaceService: d.Workspaces, RegistryService: d.Registry}
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
