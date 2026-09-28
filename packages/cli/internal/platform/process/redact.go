package process

import (
	"context"
	"errors"
	"os/exec"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/redact"
)

// RunRedacted forwards signals and masks injected secret values on both streams.
func RunRedacted(ctx context.Context, child *exec.Cmd, vars map[string]string) error {
	filter := redact.New(vars)
	out, flushOut := filter.Writer(child.Stdout)
	errOut, flushErr := filter.Writer(child.Stderr)
	child.Stdout, child.Stderr = out, errOut
	err := RunForwarded(ctx, child)
	return filter.Error(errors.Join(err, flushOut(), flushErr()))
}
