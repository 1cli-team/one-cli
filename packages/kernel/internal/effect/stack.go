// Package effect contains the private cleanup implementation used by the
// public Kernel lifecycle.
package effect

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrNilCleanup = errors.New("kernel: nil cleanup")
	ErrClosed     = errors.New("kernel: lifecycle already closed")
)

type Cleanup func(context.Context) error

type entry struct {
	name string
	run  Cleanup
}

// Stack owns cleanup effects for one execution tree.
type Stack struct {
	mu      sync.Mutex
	entries []entry
	closed  bool
	done    chan struct{}
	err     error
}

func NewStack() *Stack {
	return &Stack{done: make(chan struct{})}
}

func (s *Stack) Add(name string, cleanup Cleanup) error {
	if cleanup == nil {
		return ErrNilCleanup
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if s.done == nil {
		s.done = make(chan struct{})
	}
	s.entries = append(s.entries, entry{name: name, run: cleanup})
	return nil
}

func (s *Stack) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.done == nil {
		s.done = make(chan struct{})
	}
	if s.closed {
		done := s.done
		s.mu.Unlock()
		select {
		case <-done:
			return s.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.closed = true
	entries := append([]entry(nil), s.entries...)
	s.entries = nil
	done := s.done
	s.mu.Unlock()

	var cleanupErrors []error
	for index := len(entries) - 1; index >= 0; index-- {
		if err := entries[index].run(ctx); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("%s: %w", entries[index].name, err))
		}
	}
	result := errors.Join(cleanupErrors...)

	s.mu.Lock()
	s.err = result
	close(done)
	s.mu.Unlock()
	return result
}
