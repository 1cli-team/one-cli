package taskui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func testStore(t *testing.T, names ...string) *logStore {
	t.Helper()
	s, err := newLogStore(names)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.close)
	return s
}
func writeLog(t *testing.T, w *logWriter, text string) {
	t.Helper()
	if _, err := w.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
}
func key(m model, code rune, text string) model {
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: code, Text: text}))
	return next.(model)
}
func plainPage(m model) string { return ansi.Strip(strings.Join(m.page(), "\n")) }

func TestFullHistoryJournalAndCleanup(t *testing.T) {
	s := testStore(t, "//:web:dev")
	w := newLogWriter(s)
	writeLog(t, w, "\x1b[32m[web:")
	writeLog(t, w, "dev]\x1b[0m 中文 ready\nuntagged\n")
	for i := 0; i < 10000; i++ {
		writeLog(t, w, fmt.Sprintf("[web:dev] %05d %s\n", i, strings.Repeat("x", 300)))
	}
	if s.count("") != 10002 || s.count("//:web:dev") != 10001 {
		t.Fatal("lost log records")
	}
	if got := s.read(0); ansi.Strip(got) != "[web:dev] 中文 ready" || got == ansi.Strip(got) {
		t.Fatalf("prefix/style lost: %q", got)
	}
	if !strings.Contains(s.read(10001), "09999") {
		t.Fatal("tail lost")
	}
	info, err := os.Stat(s.file.Name())
	if err != nil || info.Size() < 2<<20 {
		t.Fatal("journal not complete", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatal("journal accessible to other users")
	}
	path := s.file.Name()
	s.close()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("journal not removed")
	}
}
func TestStylesBlankLinesLongLinesAndProgress(t *testing.T) {
	s := testStore(t, "//:web:dev", "//:api:dev")
	w := newLogWriter(s)
	input := "\x1b[35m[web:dev\t]\x1b[0m \x1b[1;3;4;38;2;12;34;56m中文 👩‍💻\n[api:dev] normal\n[web:dev] still styled\x1b[0m\n\n[web:dev     ]   spaced\tcolumn\n[web:dev] progress 123\r\x1b[2Kdone\n[web:dev] abc\bZ\n"
	// Split every UTF-8 byte and every escape sequence across writes.
	for i := range len(input) {
		writeLog(t, w, input[i:i+1])
	}
	if s.count("") != 7 {
		t.Fatal(s.count(""))
	}
	if s.read(3) != "" {
		t.Fatal("blank line lost")
	}
	if ansi.Strip(s.read(4)) != "[web:dev]   spaced        column" {
		t.Fatalf("indent/tab %q", ansi.Strip(s.read(4)))
	}
	if ansi.Strip(s.read(5)) != "[web:dev] done" || ansi.Strip(s.read(6)) != "[web:dev] abZ" {
		t.Fatal("line editing failed", s.read(5), s.read(6))
	}
	styled := uv.NewStyledString(s.read(2)).Lines(ansi.GraphemeWidth)[0]
	last := styled[len(styled)-1]
	if last.Style.Attrs&uv.AttrBold == 0 || last.Style.Attrs&uv.AttrItalic == 0 || last.Style.Underline == uv.UnderlineStyleNone || last.Style.Fg == nil {
		t.Fatal("style did not survive a task switch", last.Style)
	}
	normal := uv.NewStyledString(s.read(1)).Lines(ansi.GraphemeWidth)[0]
	if !normal[len(normal)-1].Style.IsZero() {
		t.Fatal("style leaked into another task")
	}
	long := strings.Repeat("中文x", 20000)
	writeLog(t, w, long+"\n")
	if ansi.Strip(s.read(7)) != long {
		t.Fatal("long line truncated")
	}
	writeLog(t, w, "[web:dev] pending")
	if got := s.unfinished(""); len(got) != 1 || ansi.Strip(got[0]) != "[web:dev] pending" {
		t.Fatal("partial line invisible", got)
	}
	w.flush()
	if s.count("") != 9 || len(s.unfinished("")) != 0 {
		t.Fatal("flush lost or duplicated partial line")
	}
}
func TestControlsCannotEscapeLogPane(t *testing.T) {
	input := "\x1b]0;title\x07\x1b[2J\x1b[?1049l\x1b]52;c;abc\x07safe\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\\x1b[31"
	text, _ := renderText(input, pen{})
	for _, bad := range []string{"title", "52;", "2J", "1049", "[31"} {
		if strings.Contains(text, bad) {
			t.Fatalf("control leaked: %q", text)
		}
	}
	if ansi.Strip(text) != "safelink" || !strings.Contains(text, "https://example.com") {
		t.Fatal(text)
	}
}
func TestWrapRetainsPrefixStylesAndEveryGrapheme(t *testing.T) {
	text := "\x1b[34m[//:web:dev]\x1b[0m \x1b[1;31m  中文 👩‍💻 é " + strings.Repeat("long", 40) + "  \x1b[0m"
	for _, width := range []int{2, 17, 45, 80, 180} {
		rows := wrapText(text, width)
		var b strings.Builder
		for _, row := range rows {
			if ansi.StringWidth(row.text) > width {
				t.Fatalf("overflow at %d: %q", width, row.text)
			}
			b.WriteString(ansi.Strip(row.text))
			cells := uv.NewStyledString(row.text + "!").Lines(ansi.GraphemeWidth)[0]
			if !cells[len(cells)-1].Style.IsZero() {
				t.Fatal("style leaked past log row")
			}
		}
		if b.String() != ansi.Strip(text) {
			t.Fatalf("text lost at width %d: %q", width, b.String())
		}
	}
}
func TestScrollResizeAnchorAndTaskPositions(t *testing.T) {
	s := testStore(t, "//:web:dev")
	w := newLogWriter(s)
	for i := 0; i < 100; i++ {
		writeLog(t, w, fmt.Sprintf("[web:dev] line-%03d %s\n", i, strings.Repeat("中文", 40)))
	}
	m := testModel(s)
	m.height = 12
	m.width = 110
	m = key(m, tea.KeyHome, "")
	if !strings.Contains(plainPage(m), "line-000") {
		t.Fatal("home missed first log")
	}
	m = key(m, tea.KeyPgDown, "")
	before := m.top()
	original := s.read(before.line)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 45, Height: 10})
	m = m2.(model)
	after := m.top()
	if after.line != before.line || after.column > before.column {
		t.Fatal("resize lost anchor", before, after)
	}
	if s.read(after.line) != original {
		t.Fatal("anchor points to different log")
	}
	for _, width := range []int{27, 75, 38, 110} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 12})
		m = next.(model)
	}
	if m.top() != before {
		t.Fatal("repeated resize drifted", before, m.top())
	}
	held := plainPage(m)
	for i := 0; i < 30; i++ {
		writeLog(t, w, "[web:dev] NEW OUTPUT\n")
	}
	if got := plainPage(m); got != held {
		t.Fatal("history jumped when output arrived")
	}
	m = key(m, ']', "]")
	m = key(m, tea.KeyHome, "")
	m = key(m, 'a', "a")
	if plainPage(m) != held {
		t.Fatal("task switch lost position")
	}
	m = key(m, tea.KeyEnd, "")
	if !m.reading().follow || !strings.Contains(plainPage(m), "NEW OUTPUT") {
		t.Fatal("end did not follow")
	}
	mouse, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	m = mouse.(model)
	if m.reading().follow {
		t.Fatal("wheel did not pause")
	}
	m = key(m, 'f', "f")
	if !m.reading().follow {
		t.Fatal("f did not follow")
	}
}
func TestSearchReachesOldLogsAndPreservesColors(t *testing.T) {
	s := testStore(t, "//:web:dev")
	w := newLogWriter(s)
	writeLog(t, w, "[web:dev] \x1b[31mFIRST_NEEDLE\x1b[0m\n")
	for range 6000 {
		writeLog(t, w, "[web:dev] filler\n")
	}
	m := testModel(s)
	m.query = "needle"
	cmd := m.beginSearch(true)
	changed, _ := m.Update(cmd())
	m = changed.(model)
	if len(m.matches) != 1 || !strings.Contains(plainPage(m), "FIRST_NEEDLE") || !strings.Contains(strings.Join(m.page(), ""), "\x1b[31m") {
		t.Fatal("history search failed")
	}
	writeLog(t, w, "[web:dev] SECOND_NEEDLE\n")
	cmd = m.beginSearch(false)
	changed, _ = m.Update(cmd())
	m = changed.(model)
	if len(m.matches) != 2 {
		t.Fatal("new matches missing")
	}
}
func TestFocusAndCancellation(t *testing.T) {
	s := testStore(t, "//:web:dev")
	m := testModel(s)
	cancelled := false
	m.cancel = func() { cancelled = true }
	m = key(m, tea.KeyTab, "")
	m = key(m, tea.KeyDown, "")
	if m.selected != 1 {
		t.Fatal("task focus failed")
	}
	m = key(m, tea.KeyTab, "")
	m = key(m, tea.KeyUp, "")
	if m.selected != 1 {
		t.Fatal("scroll changed task")
	}
	_, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	if !cancelled {
		t.Fatal("ctrl+c did not cancel")
	}
}
func TestSmallTerminalsAndBothLocales(t *testing.T) {
	old := i18n.Active()
	t.Cleanup(func() { i18n.Init(old) })
	s := testStore(t, "//:web:dev")
	writeLog(t, newLogWriter(s), "[web:dev] 中文 long log text\n")
	for _, locale := range []string{"en-US", "zh-CN"} {
		i18n.Init(locale)
		for _, size := range [][2]int{{1, 1}, {2, 2}, {38, 8}, {80, 24}, {140, 30}} {
			m := testModel(s)
			m.width = size[0]
			m.height = size[1]
			lines := strings.Split(m.View().Content, "\n")
			if len(lines) > size[1] {
				t.Fatal("height overflow", size)
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > size[0] {
					t.Fatalf("width overflow %v %q", size, line)
				}
			}
		}
	}
}
func TestJournalFailureAndColorPreferences(t *testing.T) {
	s := testStore(t)
	_ = s.file.Close()
	if _, err := newLogWriter(s).Write([]byte("log\n")); err == nil || s.failure() == nil {
		t.Fatal("journal failure was hidden")
	}
	for _, entry := range []string{"NO_COLOR=1", "CLICOLOR=0", "FORCE_COLOR=0", "MISE_COLOR=false"} {
		env := ColorEnvironment([]string{entry})
		if len(env) != 1 {
			t.Fatal("overrode color preference", env)
		}
	}
	if !strings.Contains(strings.Join(ColorEnvironment([]string{"FORCE_COLOR=3"}), "\n"), "FORCE_COLOR=3") {
		t.Fatal("color depth overwritten")
	}
	if !strings.Contains(strings.Join(ColorEnvironment(nil), "\n"), "FORCE_COLOR=1") {
		t.Fatal("pipe color support missing")
	}
}
func TestNonTerminalFallsBackToStream(t *testing.T) {
	for _, mode := range []string{"auto", "tui", "stream"} {
		if Mode(mode, 2, strings.NewReader(""), &bytes.Buffer{}) != "stream" {
			t.Fatal(mode)
		}
	}
}
func BenchmarkHistoryViewport(b *testing.B) {
	s, err := newLogStore(nil)
	if err != nil {
		b.Fatal(err)
	}
	defer s.close()
	for i := 0; i < 100000; i++ {
		if err := s.add(logLine{text: fmt.Sprintf("log %d %s", i, strings.Repeat("x", 120))}); err != nil {
			b.Fatal(err)
		}
	}
	m := testModel(s)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.View()
	}
}

func TestConcurrentOutputAndHistoryReads(t *testing.T) {
	s := testStore(t, "//:web:dev", "//:api:dev")
	out, errOut := newLogWriter(s), newLogWriter(s)
	var wg sync.WaitGroup
	for i, w := range []*logWriter{out, errOut} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := []string{"web:dev", "api:dev"}[i]
			for j := 0; j < 1000; j++ {
				if _, err := w.Write([]byte(fmt.Sprintf("[%s] line %d\n", name, j))); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	m := testModel(s)
	for range 200 {
		m.View()
		s.search(context.Background(), "", "line", 0, s.count(""))
	}
	wg.Wait()
	if s.count("") != 2000 || s.count("//:web:dev") != 1000 || s.count("//:api:dev") != 1000 {
		t.Fatal("concurrent logs lost")
	}
}

func TestEraseDoesNotLeaveHalfWideCharacters(t *testing.T) {
	for _, input := range []string{"中文\b\x1b[K", "中文\x1b[3G\x1b[K"} {
		got, _ := renderText(input, pen{})
		if ansi.Strip(got) != "中" {
			t.Fatalf("half-wide glyph retained: %q", got)
		}
	}
}

func TestConcurrentPartialProgressRendering(t *testing.T) {
	s := testStore(t)
	w := newLogWriter(s)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			_, _ = w.Write([]byte(strings.Repeat("long", 100) + "\r\x1b[2Kx"))
			_, _ = w.Write([]byte("\n"))
		}
	}()
	m := testModel(s)
	m.width = 45
	for range 500 {
		m.View()
	}
	<-done
	if s.count("") != 500 {
		t.Fatal("partial progress was lost")
	}
}

func TestJournalCreationFailureIsExplainedBeforeStartingChild(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, missing)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer
	cmd := exec.Command("must-not-start")
	err := Run(ctx, cancel, cmd, Graph{}, strings.NewReader(""), &out, nil)
	if err == nil || cmd.Process != nil || !strings.Contains(out.String(), missing) {
		t.Fatal("storage error not explained", err, out.String())
	}
}

func TestTaskCompletionExitsWithoutInput(t *testing.T) {
	for _, result := range []struct {
		name string
		err  error
	}{{"success", nil}, {"failure", errors.New("child failed")}} {
		for _, view := range []string{"following", "history", "search"} {
			t.Run(result.name+"/"+view, func(t *testing.T) {
				s := testStore(t)
				m := testModel(s)
				cancelled := false
				m.cancel = func() { cancelled = true }
				if view == "history" {
					m.readings[""] = reading{follow: false}
				}
				m.search = view == "search"
				changed, cmd := m.Update(finishMsg{result.err})
				final := changed.(model)
				if cmd == nil {
					t.Fatal("completed task waits for keyboard input")
				}
				if _, ok := cmd().(tea.QuitMsg); !ok {
					t.Fatal("completion did not exit TUI")
				}
				if !final.finished || final.err != result.err || cancelled {
					t.Fatal("completion changed the child result")
				}
			})
		}
	}
}

func testModel(s *logStore) model {
	graph := Graph{}
	for _, name := range s.names {
		graph.Tasks = append(graph.Tasks, Task{Name: name})
	}
	return newModel(s, graph)
}
