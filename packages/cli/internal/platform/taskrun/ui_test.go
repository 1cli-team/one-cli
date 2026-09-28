package taskrun

import (
	"fmt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
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
	defer func() { _ = j.terminal.Close() }()
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
			_ = j.terminal.Close()
		}
	}
}

func logModel(t *testing.T) model {
	t.Helper()
	s := &Session{opts: Options{Title: "dev", Development: true}, width: 69, height: 8}
	for _, name := range []string{"web", "api"} {
		j := &job{task: Task{Name: name}, result: Result{Status: "running"}, started: time.Now()}
		s.initTerminal(j)
		s.jobs = append(s.jobs, j)
		t.Cleanup(func() { _ = j.terminal.Close() })
		for i := range 40 {
			_, _ = fmt.Fprintf(jobOutput{s, j}, "%s-%02d\r\n", name, i)
		}
	}
	return model{s: s, width: 100, height: 12}
}

func TestSelectedProjectScrollsWithMouseAndKeyboard(t *testing.T) {
	m := logModel(t)
	if m.View().MouseMode != tea.MouseModeCellMotion {
		t.Fatal("mouse reporting disabled")
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = next.(model)
	next, _ = m.Update(tea.MouseWheelMsg{X: 50, Y: 4, Button: tea.MouseWheelUp})
	m = next.(model)
	if m.selected != 1 || m.s.jobs[1].offset != 3 || m.s.jobs[0].offset != 0 {
		t.Fatal("wheel did not scroll only the selected project")
	}
	before := terminalView(m.s.jobs[1], 8)
	_, _ = (jobOutput{m.s, m.s.jobs[1]}).Write([]byte("new output\r\n"))
	if got := terminalView(m.s.jobs[1], 8); visibleLog(got) != visibleLog(before) {
		t.Fatalf("new logs moved a paused viewport (offset %d, history %d):\n%s\nwant:\n%s", m.s.jobs[1].offset, m.s.jobs[1].terminal.ScrollbackLen(), got, before)
	}
	for _, event := range []tea.MouseWheelMsg{{X: 2, Y: 4, Button: tea.MouseWheelUp}, {X: 50, Y: 0, Button: tea.MouseWheelUp}, {X: 50, Y: 11, Button: tea.MouseWheelUp}} {
		offset := m.s.jobs[1].offset
		next, _ = m.Update(event)
		m = next.(model)
		if m.s.jobs[1].offset != offset {
			t.Fatal("wheel outside log panel changed history")
		}
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = next.(model)
	if m.selected != 0 || m.s.jobs[0].offset != 0 {
		t.Fatal("project selection changed another project's scroll position")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	m = next.(model)
	if m.s.jobs[1].offset != 0 {
		t.Fatal("End did not resume live output")
	}
}

func TestInputModeCanBrowseHistoryAndThenResumeTyping(t *testing.T) {
	m := logModel(t)
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	m = next.(model)
	if !m.input || m.s.jobs[0].offset == 0 || m.View().Cursor != nil {
		t.Fatal("input mode cannot browse history safely")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnd, Mod: tea.ModShift})
	m = next.(model)
	if m.s.jobs[0].offset != 0 || m.View().Cursor == nil {
		t.Fatal("Shift+End did not restore the live input cursor")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp, Mod: tea.ModShift})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	m = next.(model)
	if m.s.jobs[0].offset != 0 || !m.input {
		t.Fatal("typing did not restore live output")
	}
}

func TestPausedHistoryStaysAnchoredWhenHistoryIsFull(t *testing.T) {
	m := logModel(t)
	j := m.s.jobs[0]
	for i := range historyLines + 20 {
		_, _ = fmt.Fprintf(jobOutput{m.s, j}, "line-%04d\r\n", i)
	}
	m.scroll(12)
	before := terminalView(j, 8)
	_, _ = (jobOutput{m.s, j}).Write([]byte("new-1\r\nnew-2\r\nnew-3\r\n"))
	if got := terminalView(j, 8); visibleLog(got) != visibleLog(before) {
		t.Fatalf("history moved after reaching the cap:\n%s\nwant:\n%s", got, before)
	}
	m.scroll(historyLines)
	_, _ = (jobOutput{m.s, j}).Write([]byte("evict oldest\r\n"))
	if j.offset != historyLines {
		t.Fatal("evicted history left an invalid scroll offset")
	}
}

func TestWindowAndSidebarResizeEveryProjectToTheActualLogArea(t *testing.T) {
	m := logModel(t)
	m.scroll(10)
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 18}, {Width: 50, Height: 10}, {Width: 130, Height: 25}} {
		next, _ := m.Update(size)
		m = next.(model)
		next, _ = m.Update(resizeMsg{m.width, m.height})
		m = next.(model)
		for _, j := range m.s.jobs {
			if j.terminal.Width() != m.width-m.logLeft() || j.terminal.Height() != m.height-4 {
				t.Fatalf("%s was not resized: %dx%d", j.task.Name, j.terminal.Width(), j.terminal.Height())
			}
			if j.offset < 0 || j.offset > j.terminal.ScrollbackLen() {
				t.Fatal("resize left an invalid scroll offset")
			}
		}
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) > m.width {
				t.Fatalf("resized output overflows: %q", line)
			}
		}
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = next.(model)
	if m.s.width != m.width || !m.hideList {
		t.Fatal("hidden sidebar still consumes log width")
	}
}

// History trims trailing empty cells; compare visible content across the
// boundary between scrollback and the current terminal screen.
func visibleLog(s string) string {
	lines := strings.Split(ansi.Strip(s), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.Join(lines, "\n")
}

func TestRapidResizeUsesOnlyTheLatestDimensions(t *testing.T) {
	m := logModel(t)
	for _, width := range []int{90, 50, 120} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 16})
		m = next.(model)
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("pending resize overflow: %q", line)
			}
		}
	}
	next, _ := m.Update(resizeMsg{50, 16})
	m = next.(model)
	if !m.resizePending || m.s.width != 69 {
		t.Fatal("stale resize changed terminal geometry")
	}
	next, _ = m.Update(resizeMsg{120, 16})
	m = next.(model)
	if m.resizePending || m.s.width != 89 {
		t.Fatal("final resize was not applied")
	}
}

func TestFullScreenChildKeepsPageKeysAndExplicitHistoryShortcut(t *testing.T) {
	replies := &replyBuffer{}
	term := newTerminal(60, 8, replies)
	_, _ = term.Write([]byte(strings.Repeat("line\r\n", 30) + "\x1b[?1049h"))
	j := &job{terminal: term, result: Result{Status: "running"}}
	m := model{s: &Session{jobs: []*job{j}}, width: 60, height: 12, input: true}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	m = next.(model)
	if j.offset != 0 {
		t.Fatal("unmodified PgUp was intercepted from a full-screen child")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp, Mod: tea.ModShift})
	m = next.(model)
	if j.offset == 0 || !m.input {
		t.Fatal("explicit history shortcut was not available")
	}
	_ = term.Close()
	if got := replies.String(); got != "\x1b[5~" {
		t.Fatalf("wrong keys forwarded to child: %q", got)
	}
}
