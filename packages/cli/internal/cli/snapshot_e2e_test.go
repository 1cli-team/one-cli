package cli_test

// True end-to-end snapshot tests: build the binary, exec it, diff stdout/
// stderr against testdata/reference/. These complement the in-process tests
// in internal/template/list_snapshot_test.go — those test the business
// logic; this file tests the actual CLI surface (cobra, error envelopes,
// stdout/stderr partitioning, exit codes).
//
// Tests are skipped if the binary hasn't been built yet (run `task build`).
// CI should run `task build && task test` so the binary is present.
// Shared helpers live in e2e_helpers_test.go.

import (
	"reflect"
	"testing"
)

func TestSnapshot_E2E_Version(t *testing.T) {
	stdout, _, code := runBinary(t, "--version")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	want := "0.1.0\n"
	if stdout != want {
		t.Errorf("stdout mismatch\n  want: %q\n  got:  %q", want, stdout)
	}
}

func TestSnapshot_E2E_TemplatesJSON(t *testing.T) {
	stdout, _, code := runBinary(t, "templates", "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	gotMap := mustParseJSON(t, stdout)
	want := loadFixture(t, "templates.json")
	if !reflect.DeepEqual(gotMap, want) {
		t.Errorf("drift vs testdata/reference/templates.json\n  got: %s", pretty(gotMap))
	}
}

func TestSnapshot_E2E_UnknownCommand(t *testing.T) {
	_, stderr, code := runBinary(t, "unknown-cmd", "-o", "json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	gotMap := mustParseJSON(t, firstJSONLine(stderr))
	want := loadFixture(t, "error-unknown-command.json")
	if !reflect.DeepEqual(gotMap, want) {
		t.Errorf("drift vs testdata/reference/error-unknown-command.json\n  want: %s\n  got:  %s",
			pretty(want), pretty(gotMap))
	}
}
