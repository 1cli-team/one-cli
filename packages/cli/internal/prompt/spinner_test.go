package prompt

import (
	"errors"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

func TestSpin_NonTTY_RunsActionDirectly(t *testing.T) {
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })
	output.SetMode(output.ModeJSON) // pipe-equivalent: no spinner UI

	called := false
	err := Spin("does not render", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Errorf("Spin returned: %v", err)
	}
	if !called {
		t.Errorf("action was not invoked in non-TTY mode")
	}
}

func TestSpin_PropagatesActionError(t *testing.T) {
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })
	output.SetMode(output.ModeJSON)

	want := errors.New("boom")
	got := Spin("ignored", func() error { return want })
	if !errors.Is(got, want) {
		t.Errorf("Spin error = %v, want %v", got, want)
	}
}

func TestSpin_NonTTY_DoesNotBlockOnFastAction(t *testing.T) {
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })
	output.SetMode(output.ModeJSON)

	done := make(chan struct{})
	go func() {
		_ = Spin("x", func() error { return nil })
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatalf("Spin did not return within 2s in non-TTY mode")
	}
}

func TestStep_NonTTY_NoOp(t *testing.T) {
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })
	output.SetMode(output.ModeJSON)
	// In non-TTY mode Step writes nothing. We can't easily intercept
	// stderr without redirection plumbing, so this is mostly a smoke
	// test that the function returns without panicking.
	Step("hello")
}
