// Command one is the AI Native monorepo workspace orchestrator. This is the
// thin CLI entry point — the actual command handlers live in internal/bootstrap/cli.
package main

import (
	"errors"
	"os"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/bootstrap/cli"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

// version is overridden at build time via -ldflags. Keep the source fallback
// deliberately non-release; release versions come from the release input/tag.
var version = "0.0.0-dev"

func main() {
	if err := cli.Execute(version, os.Args[1:]); err != nil {
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
