package workspace_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func TestGenerateProjectID_Shape(t *testing.T) {
	cases := []struct {
		in     string
		prefix string
	}{
		{"demo", "demo"},
		{"My App", "my-app"},
		{"  spaced  ", "spaced"},
		{"camelCase", "camel-case"},
		{"with-hyphens-already", "with-hyphens-already"},
	}
	idRE := regexp.MustCompile(`^[a-z0-9-]+$`)
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := workspace.GenerateProjectID(tc.in)
			if !strings.HasPrefix(got, tc.prefix+"-") {
				t.Errorf("GenerateProjectID(%q) = %q; want prefix %q-", tc.in, got, tc.prefix)
			}
			suffix := strings.TrimPrefix(got, tc.prefix+"-")
			if len(suffix) != 6 {
				t.Errorf("suffix len = %d; want 6 (got %q)", len(suffix), got)
			}
			if !idRE.MatchString(got) {
				t.Errorf("id %q has chars outside [a-z0-9-]", got)
			}
		})
	}
}

// TestGenerateProjectID_EmptyFallback covers the all-separators name edge
// case so the prefix never collapses to an empty string (which would emit
// a leading dash).
func TestGenerateProjectID_EmptyFallback(t *testing.T) {
	got := workspace.GenerateProjectID("___")
	if !strings.HasPrefix(got, "ws-") {
		t.Errorf("GenerateProjectID for blank-after-kebab fell back to %q; want ws- prefix", got)
	}
}

func TestGenerateProjectID_RandomnessAcrossCalls(t *testing.T) {
	// Two consecutive calls with the same input must produce different
	// suffixes; otherwise the env init retry loop can never escape.
	a := workspace.GenerateProjectID("demo")
	b := workspace.GenerateProjectID("demo")
	if a == b {
		t.Errorf("expected distinct ids across calls; got %q twice", a)
	}
}
