package process

import (
	"context"
	"os/exec"
)

// Command creates a child command and transparently routes Windows batch
// launchers (.cmd/.bat) through the native command processor.
func Command(name string, args ...string) *exec.Cmd {
	return commandContext(context.Background(), name, args...)
}

// CommandContext is the context-aware form of Command.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return commandContext(ctx, name, args...)
}
