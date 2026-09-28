package taskrun

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestTerminalResizeReflowsOutputWithoutLosingTextOrColors(t *testing.T) {
	term := newTerminal(40, 5, io.Discard)
	defer term.Close()
	_, _ = term.Write([]byte("\x1b[31mabcdefghijklmnopqrstuvwxyz\x1b[0m\r\n中文日志\r\nlast"))
	for _, size := range [][2]int{{12, 4}, {50, 10}, {8, 3}, {40, 5}} {
		term.Resize(size[0], size[1])
		var lines []string
		for _, line := range term.Scrollback().Lines() {
			lines = append(lines, line.Render())
		}
		lines = append(lines, strings.Split(term.Render(), "\n")...)
		plain := strings.Builder{}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("width %d: %q", size[0], line)
			}
			plain.WriteString(strings.TrimRight(ansi.Strip(line), " "))
		}
		if got := plain.String(); got != "abcdefghijklmnopqrstuvwxyz中文日志last" {
			t.Fatalf("size %v lost or duplicated output: %q", size, got)
		}
		if !strings.Contains(strings.Join(lines, ""), "\x1b[") {
			t.Fatal("lost color")
		}
	}
}

func TestTerminalResizePreservesPartialSequencesAndCarriageReturns(t *testing.T) {
	term := newTerminal(30, 5, io.Discard)
	defer term.Close()
	_, _ = term.Write([]byte("old progress\rnew\x1b[K\r\n\x1b[3"))
	term.Resize(20, 6)
	_, _ = term.Write([]byte("1mRED\x1b[0m\r\n\xe4\xb8"))
	term.Resize(24, 6)
	_, _ = term.Write([]byte("\xad"))
	if got := term.String(); strings.Contains(got, "old") || !strings.Contains(got, "new\nRED\n中") {
		t.Fatalf("control or UTF-8 sequence lost: %q", got)
	}
}

type replyBuffer struct {
	sync.Mutex
	bytes.Buffer
}

func (b *replyBuffer) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.Write(p)
}

func TestTerminalResizeDoesNotReplayRepliesOrLoseChildInput(t *testing.T) {
	replies := &replyBuffer{}
	term := newTerminal(40, 6, replies)
	_, _ = term.Write([]byte("hello\x1b[6n\x1b[c\x1b]11;?\x1b\\"))
	term.Resize(20, 8)
	term.Resize(60, 12)
	term.SendKey(uv.KeyPressEvent{Code: 'x', Text: "x"})
	_ = term.Close()
	got := replies.String()
	if strings.Count(got, "R") != 1 || strings.Count(got, "c") != 1 || strings.Count(got, "rgb:") != 1 || !strings.HasSuffix(got, "x") {
		t.Fatalf("duplicate replies or lost input: %q", got)
	}
}

func TestTerminalReplayHistoryIsBoundedAtSequenceBoundaries(t *testing.T) {
	term := &terminal{}
	for range 50 {
		term.retain([]byte("\x1b[31m中文日志\x1b[0m\r\n"), 128)
	}
	if len(term.output) > 128 || !term.truncated || !utf8.Valid(term.output) || !bytes.HasPrefix(term.output, []byte("\x1b[31m")) {
		t.Fatalf("invalid history: %q", term.output)
	}
	term.retain([]byte("\x1b]0;"+strings.Repeat("x", 300)), 128)
	if len(term.output) > 128 || term.discarding == nil {
		t.Fatal("unbounded partial control sequence")
	}
	term.retain([]byte("ignored\x1b\\visible\r\n"), 128)
	if string(term.output) != "visible\r\n" || term.discarding != nil {
		t.Fatalf("partial sequence leaked into output: %q", term.output)
	}
}

func TestAlternateScreenResizeKeepsChildLayout(t *testing.T) {
	term := newTerminal(30, 5, io.Discard)
	defer term.Close()
	_, _ = term.Write([]byte("main\x1b[?1049h\x1b[2J\x1b[Hscreen"))
	term.Resize(50, 10)
	if !term.IsAltScreen() || term.Width() != 50 || !strings.Contains(term.String(), "screen") {
		t.Fatalf("alternate screen corrupted: %q", term.String())
	}
	_, _ = term.Write([]byte("\x1b[?1049l"))
	term.Resize(20, 5)
	if term.IsAltScreen() || !strings.Contains(term.String(), "main") {
		t.Fatalf("main screen not restored: %q", term.String())
	}
}

func BenchmarkTerminalResizeHistory(b *testing.B) {
	term := newTerminal(100, 30, io.Discard)
	defer term.Close()
	for range 10000 {
		_, _ = term.Write([]byte(strings.Repeat("long log output ", 12) + "\r\n"))
	}
	b.ResetTimer()
	for i := range b.N {
		term.Resize(70+(i%2)*60, 30)
	}
}

func TestTrimmedReplayKeepsStylesThatSpanLines(t *testing.T) {
	term := newTerminal(20, 4, io.Discard)
	defer term.Close()
	term.retain([]byte("\x1b[31m"+strings.Repeat("red line\r\n", 20)), 80)
	term.Resize(30, 6)
	if term.CellAt(0, 0).Style.Fg == nil || !strings.Contains(term.String(), "red line") {
		t.Fatal("trimming lost a color set before the retained suffix")
	}
}
