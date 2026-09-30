package taskui

import (
	"slices"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestTaskPrefixColorsKeepIdentityAndBodyStyles(t *testing.T) {
	names := []string{"//:admin:dev", "//:_node:commercial-web:build", "//:server:dev"}
	s := testStore(t, names...)
	stdout, stderr := newLogWriter(s), newLogWriter(s)
	writeLog(t, stdout, "[admin:dev       ] \x1b[1;31m中文 error\n")
	writeLog(t, stderr, "[//:admin:dev\t] stderr\n")
	writeLog(t, stdout, "[//:_node:commercial-web:build] normal\n[server:dev] server\n[//:admin:dev] still red\x1b[0m\n[unknown] \x1b[32muntouched\x1b[0m\n")
	colors := map[ansi.IndexedColor]bool{}
	for _, id := range []int{0, 2, 3} {
		cells := uv.NewStyledString(s.read(id)).Lines(ansi.GraphemeWidth)[0]
		color, ok := cells[0].Style.Fg.(ansi.IndexedColor)
		if !ok || colors[color] {
			t.Fatal("task prefixes are not distinguishable", s.read(id))
		}
		colors[color] = true
	}
	first := uv.NewStyledString(s.read(0)).Lines(ansi.GraphemeWidth)[0]
	if got := ansi.Strip(s.read(1)); got != "[//:admin:dev] stderr" {
		t.Fatalf("scheduler tab padding retained: %q", got)
	}
	for _, id := range []int{1, 4} {
		cells := uv.NewStyledString(s.read(id)).Lines(ansi.GraphemeWidth)[0]
		if cells[0].Style.Fg != first[0].Style.Fg {
			t.Fatal("alias or output stream changed the task color")
		}
	}
	for _, id := range []int{0, 4} {
		cells := uv.NewStyledString(s.read(id)).Lines(ansi.GraphemeWidth)[0]
		if cells[len(cells)-1].Style != first[len(first)-1].Style || cells[len(cells)-1].Style.Fg != ansi.BasicColor(1) {
			t.Fatal("prefix color overwrote the child's red/bold style")
		}
	}
	for _, id := range []int{1, 2, 3} {
		cells := uv.NewStyledString(s.read(id)).Lines(ansi.GraphemeWidth)[0]
		if !cells[len(cells)-1].Style.IsZero() {
			t.Fatal("prefix or other writer's color leaked into the body")
		}
	}
	unknown := uv.NewStyledString(s.read(5)).Lines(ansi.GraphemeWidth)[0]
	if ansi.Strip(s.read(5)) != "[unknown] untouched" || !unknown[0].Style.IsZero() || unknown[len(unknown)-1].Style.Fg != ansi.BasicColor(2) {
		t.Fatal("unrecognized log prefix was recolored", s.read(5))
	}
	if got := textRange([]string{s.read(0), s.read(2)}, cursor{}, cursor{1, ansi.StringWidth(s.read(2))}); got != "[admin:dev] 中文 error\n[//:_node:commercial-web:build] normal" || strings.Contains(got, "\x1b") {
		t.Fatalf("prefix colors changed copied text: %q", got)
	}
	// The same graph gets the same colors even if discovery order changes.
	ordered := slices.Clone(names)
	slices.Reverse(ordered)
	other := testStore(t, ordered...)
	for _, name := range names {
		if s.prefixStyles[name] != other.prefixStyles[name] {
			t.Fatal("graph discovery order changed colors", name)
		}
	}
}
