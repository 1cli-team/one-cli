package kernel

import (
	"context"
	"sync"

	"github.com/torchstellar-team/one-cli/packages/kernel/internal/effect"
)

var (
	// ErrNilCleanup reports an attempt to register an empty cleanup effect.
	ErrNilCleanup = effect.ErrNilCleanup
	// ErrLifecycleClosed reports registration after lifecycle shutdown started.
	ErrLifecycleClosed = effect.ErrClosed
)

// Cleanup releases one temporary effect.
type Cleanup func(context.Context) error

// Lifecycle owns temporary effects. Close runs cleanup once in reverse
// registration order and returns the joined cleanup failures.
type Lifecycle struct {
	mu    sync.Mutex
	stack *effect.Stack
}

// NewLifecycle creates an open lifecycle.
func NewLifecycle() *Lifecycle {
	return &Lifecycle{stack: effect.NewStack()}
}

func (l *Lifecycle) effectStack() *effect.Stack {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stack == nil {
		l.stack = effect.NewStack()
	}
	return l.stack
}

// Defer registers a named cleanup effect.
func (l *Lifecycle) Defer(name string, cleanup Cleanup) error {
	if cleanup == nil {
		return ErrNilCleanup
	}
	return l.effectStack().Add(name, effect.Cleanup(cleanup))
}

// Close releases the lifecycle. Concurrent callers wait for the owner that
// started cleanup, unless their context is cancelled first.
func (l *Lifecycle) Close(ctx context.Context) error {
	return l.effectStack().Close(ctx)
}
