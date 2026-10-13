// Command one is the AI Native monorepo workspace orchestrator. This is the
// thin CLI entry point — the actual command handlers live in internal/bootstrap/cli.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/bootstrap/cli"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/desktopcontrol"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/installation"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	platformprocess "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/updatecheck"
)

// version is overridden at build time via -ldflags. Keep the source fallback
// deliberately non-release; release versions come from the release input/tag.
var version = "0.0.0-dev"

var commit = "unknown"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}
	if len(os.Args) > 1 {
		identity := desktopcontrol.Identity{Version: version, Commit: commit, Channel: updatecheck.Channel(), Protocol: desktopcontrol.Protocol, API: desktopcontrol.API}
		switch os.Args[1] {
		case "__install-terminal":
			if err := cli.InitializeStorage(os.Args[1:]); err != nil {
				os.Exit(1)
			}
			if len(os.Args) != 3 {
				os.Exit(2)
			}
			source, err := os.Executable()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			status, err := installation.Inspect(context.Background(), os.Args[2], version)
			if err == nil {
				status, err = installation.Install(context.Background(), source, os.Args[2], version, status.Digest)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			_ = json.NewEncoder(os.Stdout).Encode(status)
			return
		case "__desktop-identity":
			_ = json.NewEncoder(os.Stdout).Encode(identity)
			return
		}
	}
	// Identity/version probes don't open stores; all other public commands do.
	if err := cli.InitializeStorage(os.Args[1:]); err != nil {
		os.Exit(1)
	}
	if updatecheck.RunWorker(version, os.Args[1:]) {
		return
	}
	if err := cli.Execute(version, os.Args[1:]); err != nil {
		var exit *platformprocess.ExitStatus
		if errors.As(err, &exit) {
			os.Exit(exit.Code)
		}
		// cli.Execute already emits the structured error envelope; we only
		// need to surface the non-zero exit status here.
		var cliErr *output.Error
		if errors.As(err, &cliErr) && cliErr.Exit0 {
			// Cooperative cancel (e.g. Ctrl-C or "configure later") is
			// intentionally quiet and successful.
			return
		}
		os.Exit(1)
	}
}
