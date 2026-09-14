// Package runtime defines the boundary for preparing a command's tool environment.
package runtime

import (
	"context"
	"fmt"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

const (
	Builtin            = "builtin"
	Mise               = "mise"
	MinimumMiseVersion = "2026.9.7"
)

type Command struct {
	Directory string
	Argv      []string
	Env       []string
}

// Provider wraps a command without executing that command or loading One secrets.
type Provider interface {
	Prepare(context.Context, Command) (Command, error)
	// PrepareCLI addresses the runtime's own commands (for example trust or
	// doctor) without loading the project's runtime environment first.
	PrepareCLI(context.Context, Command) (Command, error)
}

func Validate(kind string) error {
	if kind == "" || kind == Builtin || kind == Mise {
		return nil
	}
	return cliErrors.New(cliErrors.RUNTIME_INVALID, fmt.Sprintf("Unknown runtime %q; use builtin or mise.", kind))
}
