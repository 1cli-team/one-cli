package taskui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// Only the viewport's logical lines are frozen. The session journal continues
// receiving output, including progress lines that overwrite themselves.
type copyFrame struct {
	lines                      []string
	header                     []string
	footerSlots                int
	cache                      rowCache
	top, start, end            cursor
	anchor, head               cursor
	selected, dragging, native bool
}

// The latest frame supplied to the terminal renderer, not the live log tail.
type renderedFrame struct {
	frame         *copyFrame
	task, query   string
	width, height int
}
type copyResultMsg struct {
	text   string
	remote bool
	err    error
}
type pageRow struct {
	at   cursor
	text string
}

func (m model) lineText(line int) string {
	if m.copy != nil {
		if line < len(m.copy.lines) {
			return m.copy.lines[line]
		}
		return ""
	}
	if m.query != "" {
		if line < len(m.matches) {
			return m.logs.read(m.matches[line])
		}
		return ""
	}
	count := m.logs.count(m.task())
	if line < count {
		return m.logs.read(m.logs.id(m.task(), line))
	}
	pending := m.logs.unfinished(m.task())
	if line-count < len(pending) {
		return pending[line-count]
	}
	return ""
}
func (m model) pageRows() []pageRow {
	_, _, height := m.layout()
	page := make([]pageRow, 0, height)
	if m.count() == 0 {
		return page
	}
	top := m.top()
	line := top.line
	row := rowAt(m.rows(line), top.column)
	for len(page) < height && line < m.count() {
		rows := m.rows(line)
		row = min(row, len(rows)-1)
		page = append(page, pageRow{cursor{line, rows[row].column}, rows[row].text})
		row++
		if row == len(rows) {
			line++
			row = 0
		}
	}
	return page
}
func (m model) captureFrame() *copyFrame {
	page := m.pageRows()
	frame := &copyFrame{header: m.headerLines(), footerSlots: len(m.footerLines())}
	if len(page) > 0 {
		first, last := page[0], page[len(page)-1]
		for line := first.at.line; line <= last.at.line; line++ {
			frame.lines = append(frame.lines, m.lineText(line))
		}
		frame.top = cursor{0, first.at.column}
		frame.start = frame.top
		frame.end = cursor{len(frame.lines) - 1, last.at.column + ansi.StringWidth(last.text)}
	}
	return frame
}
func (m *model) freeze() {
	if m.copy != nil {
		return
	}
	previous := m.rendered
	if previous != nil && previous.frame != nil && previous.task == m.task() && previous.query == m.query && previous.width == m.width && previous.height == m.height {
		snapshot := *previous.frame
		m.copy = &snapshot
	} else {
		m.copy = m.captureFrame()
	}
}
func (m *model) nativeCopy() {
	m.freeze()
	page := m.pageRows()
	if len(page) > 0 {
		m.copy.start = page[0].at
		last := page[len(page)-1]
		m.copy.end = cursor{last.at.line, last.at.column + ansi.StringWidth(last.text)}
	}
	m.copy.native = true
	m.copyMenu = false
	m.copyNote = ""
}
func (m *model) resumeCopy(follow bool) {
	m.copy = nil
	m.copyMenu = false
	m.copyNote = ""
	if follow {
		m.follow()
	}
}

// Columns are snapped to complete graphemes, including emoji and combining marks.
func graphemeColumn(text string, column int, end bool) int {
	plain := ansi.Strip(text)
	x := 0
	var state byte
	for len(plain) > 0 {
		_, width, n, next := ansi.DecodeSequence(plain, state, nil)
		if n == 0 {
			break
		}
		state = next
		if x+width > column {
			if end {
				return x + width
			}
			return x
		}
		x += width
		plain = plain[n:]
	}
	return x
}
func textRange(lines []string, from, to cursor) string {
	if len(lines) == 0 {
		return ""
	}
	if before(to, from) {
		from, to = to, from
	}
	from.line = min(max(0, from.line), len(lines)-1)
	to.line = min(max(0, to.line), len(lines)-1)
	selected := make([]string, 0, to.line-from.line+1)
	for i := from.line; i <= to.line; i++ {
		text := ansi.Strip(lines[i])
		lo, hi := 0, ansi.StringWidth(text)
		if i == from.line {
			lo = graphemeColumn(text, from.column, false)
		}
		if i == to.line {
			hi = graphemeColumn(text, to.column, false)
		}
		selected = append(selected, ansi.Cut(text, lo, max(lo, hi)))
	}
	return strings.Join(selected, "\n")
}
func (f *copyFrame) selection() (cursor, cursor) {
	a, b := f.anchor, f.head
	if before(b, a) {
		a, b = b, a
	}
	if b.line < len(f.lines) {
		b.column = graphemeColumn(f.lines[b.line], b.column, true)
	}
	return a, b
}
func (m model) selectedText() string {
	if m.copy == nil || !m.copy.selected {
		return ""
	}
	a, b := m.copy.selection()
	return textRange(m.copy.lines, a, b)
}
func (m model) visibleText() string {
	if m.copy != nil {
		if !m.copy.native {
			page := m.pageRows()
			if len(page) == 0 {
				return ""
			}
			last := page[len(page)-1]
			return textRange(m.copy.lines, page[0].at, cursor{last.at.line, last.at.column + ansi.StringWidth(last.text)})
		}
		return textRange(m.copy.lines, m.copy.start, m.copy.end)
	}
	m.freeze()
	return textRange(m.copy.lines, m.copy.start, m.copy.end)
}
func (m model) mouseCursor(x, y int) cursor {
	left, width, _ := m.layout()
	origin := 0
	if left > 0 {
		origin = left + 2
	}
	page := m.pageRows()
	if len(page) == 0 {
		return cursor{}
	}
	row := page[min(max(0, y-len(m.headerLines())), len(page)-1)]
	column := row.at.column + min(max(0, x-origin), width)
	column = graphemeColumn(m.lineText(row.at.line), column, false)
	return cursor{row.at.line, column}
}
func (m *model) startSelection(x, y int) {
	m.taskFocus = false
	m.copyMenu = false
	m.copyNote = ""
	m.freeze()
	m.copy.native = false
	m.copy.selected = true
	m.copy.dragging = true
	m.copy.anchor = m.mouseCursor(x, y)
	m.copy.head = m.copy.anchor
}
func (m model) highlighted(row pageRow) string {
	if m.copy == nil || !m.copy.selected {
		return row.text
	}
	a, b := m.copy.selection()
	if row.at.line < a.line || row.at.line > b.line {
		return row.text
	}
	lo, hi := row.at.column, row.at.column+ansi.StringWidth(row.text)
	if row.at.line == a.line {
		lo = max(lo, a.column)
	}
	if row.at.line == b.line {
		hi = min(hi, b.column)
	}
	lo -= row.at.column
	hi -= row.at.column
	if hi <= lo {
		return row.text
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("14"))
	return ansi.Cut(row.text, 0, lo) + style.Render(ansi.Strip(ansi.Cut(row.text, lo, hi))) + ansi.Cut(row.text, hi, ansi.StringWidth(row.text))
}
func (m model) clipboardCmd(text string) tea.Cmd {
	ctx, write := m.ctx, m.clipboard
	return func() tea.Msg {
		if ctx.Err() != nil {
			return nil
		}
		remote, err := write(ctx, text)
		return copyResultMsg{text: text, remote: remote, err: err}
	}
}
func (m model) historyCopyCmd() tea.Cmd {
	// Capture the committed identities and current progress text at the key event.
	// Searching copies only matching records, never hidden unmatched lines.
	ids := append([]int(nil), m.matches...)
	var pending []string
	if m.query == "" {
		ids = m.logs.ids(m.task())
		pending = m.logs.unfinished(m.task())
	}
	ctx, logs, write := m.ctx, m.logs, m.clipboard
	return func() tea.Msg {
		var text strings.Builder
		for i, id := range ids {
			if ctx.Err() != nil {
				return nil
			}
			if i > 0 {
				text.WriteByte('\n')
			}
			text.WriteString(ansi.Strip(logs.read(id)))
		}
		for i, line := range pending {
			if len(ids) > 0 || i > 0 {
				text.WriteByte('\n')
			}
			text.WriteString(ansi.Strip(line))
		}
		if ctx.Err() != nil {
			return nil
		}
		if err := logs.failure(); err != nil {
			return copyResultMsg{err: err}
		}
		value := text.String()
		remote, err := write(ctx, value)
		return copyResultMsg{text: value, remote: remote, err: err}
	}
}
func (m model) copyView() tea.View {
	text := m.visibleText()
	lines := strings.Split(ansi.Hardwrap(text, max(1, m.width), false), "\n")
	content := []string{ansi.Truncate(i18n.T("tasks.ui.native_copy"), m.width, "…")}
	available := max(0, m.height-2)
	content = append(content, lines[:min(available, len(lines))]...)
	for len(content) < m.height-1 {
		content = append(content, "")
	}
	if len(content) < m.height {
		content = append(content, ansi.Truncate(i18n.T("tasks.ui.native_copy_keys"), m.width, "…"))
	}
	for i, line := range content {
		content[i] = ansi.Truncate(line, m.width, "…")
	}
	view := tea.NewView(strings.Join(content, "\n"))
	view.AltScreen = true
	view.MouseMode = tea.MouseModeNone
	return view
}
