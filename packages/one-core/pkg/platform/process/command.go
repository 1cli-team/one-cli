package process

import (
	"context"
	"os/exec"
)

// Command creates a child command, routes Windows batch launchers (.cmd/.bat)
// through the native command processor, and preserves cmd.exe shell source.
func Command(name string, args ...string) *exec.Cmd {
	return commandContext(context.Background(), name, args...)
}

// CommandContext is the context-aware form of Command.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return commandContext(ctx, name, args...)
}
