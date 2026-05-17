package preset_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/preset"
)

// TestParseRejections covers structural errors the parser must catch.
// Every entry should fail Parse with a *ParseError; the substring is a
// non-strict sanity check on the message.
func TestParseRejections(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		msgSubstr string
	}{
		{"empty", "", "empty"},
		{"version only", "1", "at least one segment"},
		{"wrong version", "2.bgo", "unsupported version"},
		{"two-digit version", "12.bgo", "unsupported version"},
		{"empty segment", "1..bgo", "empty segment"},
		{"frontend payload too short", "1.fn", "payload must be 2 chars"},
		{"frontend payload too long", "1.fnabcd", "payload must be"},
		{"library with deploy code", "1.ltlv", "exactly 2 chars"},
		{"env payload too long", "1.bgo.eii", "exactly 1 char"},
		{"duplicate env segment", "1.bgo.ei.ed", "duplicate env segment"},
		{"uppercase in payload", "1.bGO", "[a-z0-9]"},
		{"hyphen in payload", "1.f-a", "[a-z0-9]"},
		{"missing project segment (env only)", "1.ei", "project segment"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			_, err := preset.Parse(c.input)
			if err == nil {
				t.Fatalf("expected error for %q", c.input)
			}
			var pe *preset.ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("expected *preset.ParseError, got %T (%v)", err, err)
			}
			if c.msgSubstr != "" && !strings.Contains(err.Error(), c.msgSubstr) {
				t.Errorf("error message %q does not contain %q", err.Error(), c.msgSubstr)
			}
		})
	}
}

// TestParseUnknownKindIsForwardCompat asserts that unrecognised kind
// characters (future-version segments) are surfaced via Spec.UnknownSegments
// rather than rejected outright. This lets older CLIs report the issue
// (via PRESET_INVALID with context) instead of crashing on a perfectly
// well-formed future id.
func TestParseUnknownKindIsForwardCompat(t *testing.T) {
	spec, err := preset.Parse("1.bgo.zzz")
	if err != nil {
		t.Fatalf("Parse should not fail on unknown kind: %v", err)
	}
	if len(spec.UnknownSegments) != 1 || spec.UnknownSegments[0] != "zzz" {
		t.Errorf("expected UnknownSegments = [\"zzz\"], got %v", spec.UnknownSegments)
	}
}
