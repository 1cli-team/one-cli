package taskrun

import (
	"io"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestTerminalHistoryIncludesCurrentScreenAndKeepsColors(t *testing.T) {
	s := &Session{width: 20, height: 3}
	j := &job{}
	s.initTerminal(j)
	defer func() { _ = j.terminal.InputPipe().(io.Closer).Close(); <-j.inputDone; _ = j.terminal.Close() }()
	_, _ = j.terminal.Write([]byte("one\r\ntwo\r\n\x1b[31mthree\x1b[0m\r\nfour\r\nfive"))
	j.offset = 1
	got := terminalView(j, 3)
	plain := ansi.Strip(got)
	for _, want := range []string{"two", "three", "four"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %s in %q", want, plain)
		}
	}
	if strings.Contains(plain, "five") || !strings.Contains(got, "\x1b[") {
		t.Fatalf("bad history %q", got)
	}
}
func TestSearchAndNavigationDoNotEnterChildInput(t *testing.T) {
	s := &Session{jobs: []*job{{task: Task{Name: "web"}}, {task: Task{Name: "api"}}}}
	m := model{s: s, width: 100, height: 30}
	next, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = next.(model)
	for _, r := range "api" {
		next, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = next.(model)
	}
	if m.selected != 1 || m.input || m.query != "api" {
		t.Fatalf("%+v", m)
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(model)
	if m.searching || m.input {
		t.Fatalf("%+v", m)
	}
}
