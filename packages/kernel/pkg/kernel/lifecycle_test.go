package kernel

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestLifecycleClosesLIFOOnceAndJoinsErrors(t *testing.T) {
	t.Parallel()

	lifecycle := NewLifecycle()
	order := []string{}
	cleanupErr := errors.New("failed")
	if err := lifecycle.Defer("first", func(context.Context) error {
		order = append(order, "first")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.Defer("second", func(context.Context) error {
		order = append(order, "second")
		return cleanupErr
	}); err != nil {
		t.Fatal(err)
	}

	err := lifecycle.Close(context.Background())
	if !errors.Is(err, cleanupErr) || err.Error() != "second: failed" {
		t.Fatalf("Close() error = %v", err)
	}
	if !reflect.DeepEqual(order, []string{"second", "first"}) {
		t.Fatalf("cleanup order = %#v", order)
	}
	if err := lifecycle.Close(context.Background()); !errors.Is(err, cleanupErr) {
		t.Fatalf("second Close() = %v", err)
	}
	if !reflect.DeepEqual(order, []string{"second", "first"}) {
		t.Fatalf("cleanup ran twice: %#v", order)
	}
	if err := lifecycle.Defer("late", func(context.Context) error { return nil }); !errors.Is(err, ErrLifecycleClosed) {
		t.Fatalf("late Defer() = %v", err)
	}
}

func TestLifecycleRejectsNilCleanup(t *testing.T) {
	t.Parallel()

	if err := NewLifecycle().Defer("nil", nil); !errors.Is(err, ErrNilCleanup) {
		t.Fatalf("Defer(nil) = %v", err)
	}
}
