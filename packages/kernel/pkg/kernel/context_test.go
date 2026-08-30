package kernel

import (
	"context"
	"testing"
)

func TestContextDeriveSharesLifecycle(t *testing.T) {
	t.Parallel()

	type key struct{}
	parent := NewContext(context.WithValue(context.Background(), key{}, "parent"))
	child := parent.Derive(context.WithValue(parent.Context(), key{}, "child"))

	if child.Lifecycle() != parent.Lifecycle() {
		t.Fatal("derived context did not share lifecycle")
	}
	if got := parent.Context().Value(key{}); got != "parent" {
		t.Fatalf("parent context value = %v", got)
	}
	if got := child.Context().Value(key{}); got != "child" {
		t.Fatalf("child context value = %v", got)
	}
}

func TestZeroContextIsSafe(t *testing.T) {
	t.Parallel()

	var runtime Context
	if runtime.Context() == nil {
		t.Fatal("zero Context returned nil standard context")
	}
	if err := runtime.Close(context.Background()); err != nil {
		t.Fatalf("zero Context Close() = %v", err)
	}
}
