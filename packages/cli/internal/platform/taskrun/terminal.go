package taskrun

import (
	"bytes"
	"io"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/parser"
	vt "github.com/charmbracelet/x/vt"
)

const historyLines = 3000
const replayBytes = 1024 * 1024

// terminal keeps the original byte stream so a resize can reflow output instead
// of irreversibly cropping the emulator's cells. The emulator still interprets
// colors, carriage returns, erase sequences and interactive terminal programs.
// Access is serialized by Session.mu; its input pump owns the pipe reader.
type terminal struct {
	*vt.Emulator
	output       []byte
	truncated    bool
	input        io.Writer
	inputDone    chan struct{}
	discarding   *ansi.Parser
	initialStyle uv.Style
}

func newTerminal(width, height int, input io.Writer) *terminal {
	t := &terminal{Emulator: vt.NewEmulator(width, height), input: input}
	t.SetScrollbackSize(historyLines)
	t.pumpInput()
	return t
}

func (t *terminal) pumpInput() {
	e := t.Emulator
	t.inputDone = make(chan struct{})
	done := t.inputDone
	go func() {
		defer close(done)
		_, _ = io.Copy(t.input, e)
	}()
}

func (t *terminal) Write(p []byte) (int, error) {
	n, err := t.Emulator.Write(p)
	t.retain(p[:n], replayBytes)
	return n, err
}

func (t *terminal) retain(p []byte, limit int) {
	// An oversized unfinished control sequence is discarded through its end,
	// rather than retaining unlimited bytes or displaying its tail as text.
	if t.discarding != nil {
		for i, b := range p {
			t.discarding.Advance(b)
			if t.discarding.State() == parser.GroundState {
				t.discarding = nil
				p = p[i+1:]
				break
			}
		}
		if t.discarding != nil {
			return
		}
	}
	t.output = append(t.output, p...)
	if len(t.output) <= limit {
		return
	}
	// Drop at least half the journal to amortize compaction. Prefer a logical
	// newline, and never split a UTF-8 character or an ANSI control sequence.
	cut, fallback := 0, 0
	decoder := ansi.NewParser()
	style, fallbackStyle := t.initialStyle, t.initialStyle
	decoder.SetHandler(ansi.Handler{HandleCsi: func(cmd ansi.Cmd, params ansi.Params) {
		if cmd == 'm' {
			uv.ReadStyle(params, &style)
		}
	}})
	for i, b := range t.output {
		decoder.Advance(b)
		if i+1 >= len(t.output)-limit/2 && decoder.State() == parser.GroundState {
			if fallback == 0 {
				fallback = i + 1
				fallbackStyle = style
			}
			if b == '\n' {
				cut = i + 1
				break
			}
		}
	}
	if cut == 0 {
		cut = fallback
		style = fallbackStyle
	}
	t.truncated = true
	if cut == 0 {
		t.output = nil
		t.discarding = decoder
		return
	}
	t.initialStyle = style
	t.output = bytes.Clone(t.output[cut:])
}

func (t *terminal) Resize(width, height int) {
	if width == t.Width() && height == t.Height() {
		return
	}
	// Full-screen children redraw their own alternate screen after SIGWINCH.
	// Replaying their old cursor-addressed output would overwrite that layout.
	if t.IsAltScreen() {
		t.Emulator.Resize(width, height)
		return
	}
	e := vt.NewEmulator(width, height)
	e.SetScrollbackSize(historyLines)
	// Replaying queries must never send duplicate terminal replies to a child.
	// io.Pipe preserves write boundaries: an empty write is a local fence after
	// all replay replies have been consumed. No fence bytes reach the child.
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		buf := make([]byte, 4096)
		for {
			n, err := e.Read(buf)
			if n == 0 || err != nil {
				return
			}
		}
	}()
	_, _ = e.Write([]byte(t.initialStyle.String()))
	_, _ = e.Write(t.output)
	_, _ = e.InputPipe().Write(nil)
	<-drained
	_ = t.Close()
	t.Emulator = e
	t.pumpInput()
}

func (t *terminal) Close() error {
	_ = t.Emulator.InputPipe().(io.Closer).Close()
	<-t.inputDone
	return t.Emulator.Close()
}
