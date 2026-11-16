// Package lifecycle owns plugin registration and one command invocation.
// It has no vendor-specific behavior and never uses process-global hooks.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/bootstrap"
)

type Host struct {
	Root         *cobra.Command
	Dependencies bootstrap.Dependencies
}

// Plugin contributes commands once per application. Registration must be
// local and read-only: help and inspection construct the same command tree.
type Plugin interface {
	ID() string
	Register(Host) error
}

type Invocation struct {
	Version string
	Args    []string
}

// Starter optionally acquires invocation resources. A returned cleanup runs
// even if Start fails; successful starts unwind in reverse registration order.
type Starter interface {
	Start(context.Context, Invocation) (func(context.Context) error, error)
}

type BeforeCommand interface {
	BeforeCommand(context.Context, Invocation) error
}

// AfterCommand observes the result, including failures in BeforeCommand or
// the command itself. Only plugins entered for this invocation are notified.
type AfterCommand interface {
	AfterCommand(context.Context, Invocation, error) error
}

type Lifecycle struct{ plugins []Plugin }

func New(installed ...Plugin) (*Lifecycle, error) {
	seen := make(map[string]bool, len(installed))
	for _, plugin := range installed {
		if plugin == nil || plugin.ID() == "" {
			return nil, fmt.Errorf("lifecycle: missing plugin identity")
		}
		if seen[plugin.ID()] {
			return nil, fmt.Errorf("lifecycle: duplicate plugin %q", plugin.ID())
		}
		seen[plugin.ID()] = true
	}
	return &Lifecycle{plugins: append([]Plugin(nil), installed...)}, nil
}

func (l *Lifecycle) Register(host Host) error {
	for _, plugin := range l.plugins {
		if err := plugin.Register(host); err != nil {
			return fmt.Errorf("register plugin %s: %w", plugin.ID(), err)
		}
	}
	// Cobra otherwise silently resolves duplicate commands to the first match.
	return validateCommands(host.Root)
}

func validateCommands(root *cobra.Command) error {
	seen := make(map[string]bool)
	for _, command := range root.Commands() {
		for _, name := range append([]string{command.Name()}, command.Aliases...) {
			if seen[name] {
				return fmt.Errorf("lifecycle: duplicate command %q under %s", name, root.CommandPath())
			}
			seen[name] = true
		}
		if err := validateCommands(command); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lifecycle) Run(ctx context.Context, invocation Invocation, command func(context.Context) error) (result error) {
	var entered []Plugin
	var cleanups []func(context.Context) error
	defer func() {
		// Cancellation stops work, but must not prevent resources being released.
		closing, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		commandResult := result
		for i := len(entered) - 1; i >= 0; i-- {
			if hook, ok := entered[i].(AfterCommand); ok {
				result = join(result, hook.AfterCommand(closing, copyInvocation(invocation), commandResult))
			}
		}
		for i := len(cleanups) - 1; i >= 0; i-- {
			result = join(result, cleanups[i](closing))
		}
	}()
	for _, plugin := range l.plugins {
		if err := ctx.Err(); err != nil {
			return err
		}
		if hook, ok := plugin.(Starter); ok {
			cleanup, err := hook.Start(ctx, copyInvocation(invocation))
			if cleanup != nil {
				cleanups = append(cleanups, cleanup)
			}
			if err != nil {
				return err
			}
		}
	}
	for _, plugin := range l.plugins {
		if err := ctx.Err(); err != nil {
			return err
		}
		entered = append(entered, plugin)
		if hook, ok := plugin.(BeforeCommand); ok {
			if err := hook.BeforeCommand(ctx, copyInvocation(invocation)); err != nil {
				return err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return command(ctx)
}

func copyInvocation(invocation Invocation) Invocation {
	invocation.Args = append([]string(nil), invocation.Args...)
	return invocation
}

func join(result, err error) error {
	if err == nil {
		return result
	}
	if result == nil {
		return err
	}
	return errors.Join(result, err)
}
