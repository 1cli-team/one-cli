package taskrun

import (
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"io"
	"strings"
	"testing"
	"time"

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

func TestLocalizedUIKeepsChildOutputAndTerminalWidth(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	for _, locale := range []string{"en-US", "zh-CN"} {
		_ = i18n.Init(locale)
		for _, width := range []int{40, 80, 110} {
			s := &Session{opts: Options{Title: "dev", Development: true}, width: width, height: 24}
			j := &job{task: Task{Name: "web"}, result: Result{Status: "running"}, started: time.Now()}
			s.jobs = []*job{j}
			s.initTerminal(j)
			m := model{s: s, width: width, height: 24}
			m.resize()
			_, _ = j.terminal.Write([]byte("\x1b[31m原始日志 RAW_LOG\x1b[0m"))
			original := terminalView(j, 20)
			content := m.View().Content
			if !strings.Contains(content, strings.Split(original, "\n")[0]) || !strings.Contains(ansi.Strip(content), i18n.T("task.status.running")) {
				t.Fatalf("%s missing original output or translated status: %q", locale, content)
			}
			for _, help := range []bool{false, true} {
				m.help = help
				for _, line := range strings.Split(m.View().Content, "\n") {
					if ansi.StringWidth(line) > width {
						t.Errorf("%s width %d help %t overflow: %q", locale, width, help, line)
					}
				}
			}
			if j.result.Status != "running" || terminalView(j, 20) != original {
				t.Fatal("rendering mutated protocol or log")
			}
			_ = j.terminal.InputPipe().(io.Closer).Close()
			<-j.inputDone
			_ = j.terminal.Close()
		}
	}
}
