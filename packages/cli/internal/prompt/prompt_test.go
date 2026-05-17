package prompt

import (
	stderrors "errors"
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// mapErr is the only piece of prompt logic that doesn't require a TTY,
// so it's the only piece worth unit-testing here. The huh fields
// themselves drive a bubbletea program; exercising them needs a PTY,
// which is out of scope (covered by manual smoke tests in dev).

func TestMapErr_UserAborted(t *testing.T) {
	got := mapErr(huh.ErrUserAborted)
	cliErr, ok := got.(*output.Error)
	if !ok {
		t.Fatalf("expected *output.Error, got %T", got)
	}
	if cliErr.Code != "PROMPT_CANCELLED" {
		t.Errorf("code = %q, want PROMPT_CANCELLED", cliErr.Code)
	}
	if !cliErr.Exit0 {
		t.Errorf("Exit0 should be true so Ctrl-C exits 0")
	}
}

func TestMapErr_UnknownError(t *testing.T) {
	got := mapErr(stderrors.New("oops"))
	cliErr, ok := got.(*output.Error)
	if !ok {
		t.Fatalf("expected *output.Error, got %T", got)
	}
	if cliErr.Code != "ONE_CLI_ERROR" {
		t.Errorf("code = %q, want ONE_CLI_ERROR", cliErr.Code)
	}
	if cliErr.Exit0 {
		t.Errorf("unknown errors should not be Exit0")
	}
}

func TestMapErr_Nil(t *testing.T) {
	if mapErr(nil) != nil {
		t.Error("mapErr(nil) should pass through")
	}
}
