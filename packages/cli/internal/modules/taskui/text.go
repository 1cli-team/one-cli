package taskui

import (
	"bytes"
	"strings"
	"sync"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

type pen struct {
	style uv.Style
	link  uv.Link
}
type logWriter struct {
	store   *logStore
	pending []byte
	styles  map[string]pen
}

func newLogWriter(s *logStore) *logWriter {
	w := &logWriter{store: s, styles: map[string]pen{}}
	s.writers = append(s.writers, w)
	return w
}
func (w *logWriter) Write(b []byte) (int, error) {
	n := len(b)
	for len(b) > 0 {
		end := bytes.IndexByte(b, '\n')
		if end < 0 {
			w.pending = append(w.pending, b...)
			break
		}
		w.pending = append(w.pending, b[:end]...)
		line, next := w.render()
		w.styles[line.task] = next
		if err := w.store.add(line); err != nil {
			return 0, err
		}
		w.pending = nil
		b = b[end+1:]
	}
	if len(w.pending) > 0 {
		line, _ := w.render()
		w.store.setPending(w, line)
	} else {
		w.store.setPending(w, logLine{})
	}
	return n, nil
}
func (w *logWriter) flush() {
	if len(w.pending) > 0 {
		line, _ := w.render()
		_ = w.store.add(line)
		w.pending = nil
	}
	w.store.setPending(w, logLine{})
}
func (w *logWriter) render() (logLine, pen) {
	raw := string(w.pending)
	task, prefix, body := splitPrefix(raw, w.store.names)
	// mise resets its label's styling on every line. Keep the application's
	// own pen separately, including when two tasks interleave their output.
	label, _ := renderText(prefix, pen{})
	content, next := renderText(body, w.styles[task])
	return logLine{task, label + content}, next
}
func splitPrefix(raw string, names []string) (task, prefix, body string) {
	plain := ansi.Strip(raw)
	if !strings.HasPrefix(plain, "[") {
		return "", "", raw
	}
	end := strings.IndexByte(plain, ']')
	if end < 0 {
		return "", "", raw
	}
	for _, name := range names {
		if plain[1:end] == name || plain[1:end] == strings.TrimPrefix(name, "//:") {
			task = name
			break
		}
	}
	if task == "" {
		return "", "", raw
	}
	// Preserve exactly one mise separator, including any label reset before it.
	target := end + 1
	if len(plain) > target && plain[target] == ' ' {
		target++
	}
	visible, off := 0, 0
	var state byte
	for off < len(raw) && visible < target {
		seq, width, n, next := ansi.DecodeSequence(raw[off:], state, nil)
		if n == 0 {
			break
		}
		if width > 0 {
			visible += len(seq)
		}
		off += n
		state = next
	}
	return task, raw[:off], raw[off:]
}

var parserPool = sync.Pool{New: func() any { p := new(ansi.Parser); p.SetParamsSize(32); p.SetDataSize(4096); return p }}

// Interpret line-local editing, but never let a child move the outer TUI's
// cursor, clear its screen, set titles, or change terminal modes.
func renderText(text string, style pen) (string, pen) {
	p := parserPool.Get().(*ansi.Parser)
	p.Reset()
	defer parserPool.Put(p)
	var cells []uv.Cell
	x := 0
	put := func(c uv.Cell) {
		for len(cells) < x+c.Width {
			cells = append(cells, uv.EmptyCell)
		}
		// Clear both halves of any wide grapheme overwritten by cursor editing.
		for i := x; i < x+c.Width; i++ {
			start := i
			for start > 0 && cells[start].Width == 0 {
				start--
			}
			stop := min(len(cells), start+max(1, cells[start].Width))
			for j := start; j < stop; j++ {
				cells[j] = uv.EmptyCell
			}
		}
		cells[x] = c
		for i := 1; i < c.Width; i++ {
			cells[x+i] = uv.Cell{}
		}
		x += c.Width
	}
	var state byte
	for len(text) > 0 {
		seq, width, n, next := ansi.DecodeSequence(text, state, p)
		if n == 0 {
			break
		}
		state = next
		text = text[n:]
		switch {
		case width > 0:
			put(uv.Cell{Content: seq, Width: width, Style: style.style, Link: style.link})
		case seq == "\r":
			x = 0
		case seq == "\b":
			x = max(0, x-1)
		case seq == "\t":
			stop := (x/8 + 1) * 8
			for x < stop {
				put(uv.Cell{Content: " ", Width: 1, Style: style.style, Link: style.link})
			}
		case ansi.HasCsiPrefix(seq) && state == ansi.NormalState:
			arg, _ := p.Param(0, 1)
			switch p.Command() {
			case 'm':
				uv.ReadStyle(p.Params(), &style.style)
			case 'G', '`':
				x = min(max(0, arg-1), len(cells)+4096)
			case 'C':
				x = min(x+max(1, arg), len(cells)+4096)
			case 'D':
				x = max(0, x-max(1, arg))
			case 'K':
				mode, _ := p.Param(0, 0)
				lo, hi := x, len(cells)
				switch mode {
				case 1:
					lo = 0
					hi = min(x+1, len(cells))
				case 2:
					lo = 0
				}
				if mode > 2 {
					continue
				}
				lo = min(lo, len(cells))
				// Erasing either half of a wide grapheme clears the whole cell.
				for lo > 0 && lo < len(cells) && cells[lo].Width == 0 {
					lo--
				}
				if hi > 0 {
					last := hi - 1
					for last > 0 && cells[last].Width == 0 {
						last--
					}
					hi = min(len(cells), max(hi, last+cells[last].Width))
				}
				for i := lo; i < hi; i++ {
					cells[i] = uv.Cell{Content: " ", Width: 1, Style: style.style}
				}
				// Erasing the suffix should not retain a screen-width run of empty cells.
				if mode == 0 {
					cells = cells[:lo]
				}
				if mode == 2 {
					cells = nil
				}
			}
		case ansi.HasOscPrefix(seq) && p.Command() == 8 && state == ansi.NormalState:
			uv.ReadLink(p.Data(), &style.link)
			if strings.ContainsAny(style.link.URL+style.link.Params, "\x1b\x07\r\n") {
				style.link = uv.Link{}
			}
		case len(seq) > 0 && seq[0] >= 0xc0 && x > 0:
			i := min(x-1, len(cells)-1)
			for i > 0 && cells[i].Width == 0 {
				i--
			}
			if i >= 0 {
				cells[i].Content += seq
			}
		}
	}
	return renderCells(cells), style
}
func renderCells(cells []uv.Cell) string {
	var b strings.Builder
	var current pen
	for _, c := range cells {
		if c.Width == 0 {
			continue
		}
		b.WriteString(c.Style.Diff(&current.style))
		current.style = c.Style
		if c.Link != current.link {
			b.WriteString(ansi.SetHyperlink(c.Link.URL, c.Link.Params))
			current.link = c.Link
		}
		b.WriteString(c.Content)
	}
	if !current.style.IsZero() {
		b.WriteString(ansi.ResetStyle)
	}
	if !current.link.IsZero() {
		b.WriteString(ansi.ResetHyperlink())
	}
	return b.String()
}

type wrappedRow struct {
	text   string
	column int
}

func wrapText(text string, width int) []wrappedRow {
	width = max(1, width)
	lines := uv.NewStyledString(text).Lines(ansi.GraphemeWidth)
	var cells []uv.Cell
	if len(lines) > 0 {
		cells = lines[0]
	}
	rows := []wrappedRow{}
	start, col, used := 0, 0, 0
	for i, c := range cells {
		if used > 0 && used+c.Width > width {
			rows = append(rows, wrappedRow{renderCells(cells[start:i]), col - used})
			start = i
			used = 0
		}
		if c.Width > width {
			// A double-width glyph cannot fit a one-column terminal. Its original
			// bytes stay in the journal and reappear when the terminal grows.
			rows = append(rows, wrappedRow{"�", col})
			col += c.Width
			start = i + 1
			continue
		}
		col += c.Width
		used += c.Width
	}
	if start < len(cells) || len(rows) == 0 {
		rows = append(rows, wrappedRow{renderCells(cells[start:]), col - used})
	}
	return rows
}
