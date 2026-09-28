package miseconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

// TrustGenerated registers only the exact generated configurations published by
// an explicit create/add operation. Planning and ordinary task execution never
// call it. Explicit file paths avoid trusting parent or unrelated configurations.
func (p *Plan) TrustGenerated(ctx context.Context, provider runtimeport.Provider) error {
	if provider == nil {
		return nil
	}
	var failures []error
	for _, generated := range p.generated {
		path := filepath.Join(p.Root, generated.Path)
		err := p.trustFile(ctx, provider, generated)
		if err != nil {
			failures = append(failures, i18n.Errorf("miseconfig.trust_failed", path, err, path))
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(failures...)
}

func (p *Plan) trustFile(ctx context.Context, provider runtimeport.Provider, generated Change) error {
	path := filepath.Join(p.Root, generated.Path)
	command, err := provider.PrepareCLI(ctx, runtimeport.Command{
		Directory: p.Root,
		Argv:      []string{"trust", "--yes", "--quiet", path},
		Env:       os.Environ(),
	})
	if err != nil {
		return err
	}
	// Recheck after runtime preparation, which may take time on a new machine.
	if err := p.safePath(generated.Path); err != nil {
		return err
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(actual) != generated.After {
		return conflict(generated.Path, i18n.T("config.plan_changed"))
	}
	cmd := exec.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
	cmd.Dir, cmd.Env = command.Directory, command.Env
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return err
}
