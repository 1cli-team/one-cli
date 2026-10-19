// Command one-api runs the shared local or self-hosted management server.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/desktopcontrol"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/updatecheck"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/app"
)

var version = "0.0.0-dev"
var commit = "unknown"

func main() {
	identity := desktopcontrol.Identity{Version: version, Commit: commit, Channel: updatecheck.Channel(), Protocol: desktopcontrol.Protocol, API: desktopcontrol.API}
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--identity" {
		_ = json.NewEncoder(os.Stdout).Encode(identity)
		return
	}
	if err := app.Run(identity, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
