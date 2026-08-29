package processorch

// supervisor_test.go — platform-agnostic helpers only. Procfile.dev
// parsing has been removed; ProcEntry slices come directly from the
// manifest now (see ops_test.go for buildEntriesFromManifest coverage).
// Lifecycle tests that exec real shell commands live in
// supervisor_unix_test.go behind a build tag.

import (
	"reflect"
	"strings"
	"testing"
)

func TestPadName(t *testing.T) {
	if got := padName("api", 5); got != "api  " {
		t.Errorf("padName(api,5) = %q, want %q", got, "api  ")
	}
	if got := padName("nestjs-api", 5); got != "nestjs-api" {
		t.Errorf("padName already-longer should not truncate, got %q", got)
	}
}

func TestMaxNameLen(t *testing.T) {
	entries := []ProcEntry{{Name: "api"}, {Name: "nestjs-api"}, {Name: "go"}}
	if n := maxNameLen(entries); n != len("nestjs-api") {
		t.Errorf("maxNameLen = %d, want %d", n, len("nestjs-api"))
	}
	if n := maxNameLen(nil); n != 0 {
		t.Errorf("maxNameLen(nil) = %d, want 0", n)
	}
}

func TestDecoratePrefix_DisabledReturnsPaddedBare(t *testing.T) {
	got := decoratePrefix("api  ", 0, false)
	if got != "api  " {
		t.Errorf("uncolored prefix = %q, want %q", got, "api  ")
	}
}

func TestDecoratePrefix_AppliesPaletteByIndex(t *testing.T) {
	first := decoratePrefix("api  ", 0, true)
	second := decoratePrefix("web  ", 1, true)
	if !strings.HasPrefix(first, "\x1b[") || !strings.HasSuffix(first, "\x1b[0m") {
		t.Errorf("missing ANSI wrap on first prefix: %q", first)
	}
	if !strings.Contains(first, "api  ") {
		t.Errorf("prefix payload missing: %q", first)
	}
	// Distinct index must yield distinct color escape — palette has > 1 entry.
	if first == second {
		t.Errorf("indices 0 and 1 produced identical coloring")
	}
}

func TestDecoratePrefix_WrapsRoundPalette(t *testing.T) {
	// Index past the palette length must wrap modulo without panicking.
	a := decoratePrefix("api", 0, true)
	b := decoratePrefix("api", len(prefixPalette), true)
	if a != b {
		t.Errorf("indices 0 and len(palette) should produce same color (modulo wrap), got %q vs %q", a, b)
	}
}

// Ensure exported types stay shaped as documented (constructor pattern
// callers in ops.go rely on the field names).
func TestProcEntry_FieldShape(t *testing.T) {
	got := ProcEntry{Name: "api", Cmd: "pnpm run dev"}
	want := ProcEntry{Name: "api", Cmd: "pnpm run dev"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ProcEntry shape drifted: %+v vs %+v", got, want)
	}
}
