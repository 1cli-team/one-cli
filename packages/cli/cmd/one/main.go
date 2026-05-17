// Command one is the AI Native monorepo workspace orchestrator. This is the
// thin CLI entry point — the actual command handlers live in internal/cli.
package main

import (
	"errors"
	"os"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cli"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// version is overridden at build time via -ldflags. The default matches
// VERSION so `go run` users see the repository's current version.
var version = "0.1.0"

func main() {
	if err := cli.Execute(version, os.Args[1:]); err != nil {
		// cli.Execute already emits the structured error envelope; we only
		// need to surface the non-zero exit status here.
		var cliErr *output.Error
		if errors.As(err, &cliErr) && cliErr.Exit0 {
			// Cooperative cancel (e.g. Ctrl-C in a prompt). Envelope was
			// emitted; treat as graceful exit so scripts and parent
			// processes don't see a fake failure.
			return
		}
		os.Exit(1)
	}
}
