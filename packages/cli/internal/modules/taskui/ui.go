// Package taskui presents the task scheduler's output and lifecycle snapshots.
package taskui

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"golang.org/x/term"
)

func Mode(request string, count int, in io.Reader, out io.Writer) string {
	if request == "raw" {
		return request
	}
	input, iok := in.(*os.File)
	terminal, ook := out.(*os.File)
	if output.IsStructured() || os.Getenv("TERM") == "dumb" || os.Getenv("CI") != "" && os.Getenv("CI") != "false" || !iok || !ook || !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(terminal.Fd())) {
		return "stream"
	}
	if request == "tui" || request == "auto" && count > 1 {
		return "tui"
	}
	return "stream"
}

// Pipes keep Process Compose's task framing intact. Advertise color support to common
// tools, while preserving explicit caller/task color choices (including off).
func ColorEnvironment(env []string) []string {
	env = append([]string(nil), env...)
	values := colorValues(env)
	if colorDisabled(values) {
		return env
	}
	level := "1"
	if values["COLORTERM"] == "truecolor" || values["COLORTERM"] == "24bit" {
		level = "3"
	} else if strings.Contains(values["TERM"], "256color") {
		level = "2"
	}
	for _, kv := range [][2]string{{"FORCE_COLOR", level}, {"CLICOLOR_FORCE", "1"}, {"MISE_COLOR", "1"}} {
		if _, ok := values[kv[0]]; !ok {
			env = append(env, kv[0]+"="+kv[1])
		}
	}
	return env
}

func colorValues(env []string) map[string]string {
	values := map[string]string{}
	for _, entry := range env {
		k, v, _ := strings.Cut(entry, "=")
		values[strings.ToUpper(k)] = v
	}
	return values
}

func colorDisabled(values map[string]string) bool {
	return values["NO_COLOR"] != "" || values["CLICOLOR"] == "0" || values["FORCE_COLOR"] == "0" || values["MISE_COLOR"] == "0" || values["MISE_COLOR"] == "false"
}

// Run restores the terminal and removes the private log journal before returning.
// The caller owns cancellation and the subprocess tree.
func Run(ctx context.Context, cancel context.CancelFunc, child *exec.Cmd, graph Graph, in io.Reader, out io.Writer, started func(), sources ...StateSource) error {
	logs, err := newLogStore(graph.names())
	if err != nil {
		err = i18n.Errorf("tasks.ui.log_error", err)
		fmt.Fprintln(out, err)
		return err
	}
	defer logs.close()
	defer func() {
		if err := logs.failure(); err != nil {
			fmt.Fprintln(out, i18n.Errorf("tasks.ui.log_error", err))
		}
	}()
	stdout, stderr := newLogWriter(logs), newLogWriter(logs)
	child.Stdout = stdout
	child.Stderr = stderr
	child.Stdin = nil
	if child.Env == nil {
		child.Env = os.Environ()
	}
	child.Env = ColorEnvironment(child.Env)
	if colorDisabled(colorValues(child.Env)) {
		logs.prefixStyles = nil
	}
	done := make(chan struct{})
	var childErr error
	initial := newModel(logs, graph)
	if len(sources) > 0 {
		initial.source = sources[0]
	}
	uiCtx, stopUI := context.WithCancel(ctx)
	defer stopUI()
	initial.ctx = uiCtx
	initial.cancel = cancel
	initial.done = done
	initial.result = &childErr
	initial.started = time.Now()
	program := tea.NewProgram(initial, tea.WithInput(in), tea.WithOutput(out), tea.WithoutSignalHandler())
	go func() {
		err := child.Start()
		if err == nil {
			if started != nil {
				started()
			}
			err = child.Wait()
		}
		stdout.flush()
		stderr.flush()
		childErr = err
		close(done)
	}()
	final, uiErr := program.Run()
	if uiErr != nil {
		cancel()
		<-done
		if childErr != nil {
			return childErr
		}
		return uiErr
	}
	result := final.(model)
	if result.searchCancel != nil {
		result.searchCancel()
	}
	// Keep the usual short tail in the parent terminal, with prefixes and styles.
	count := logs.count("")
	for i := max(0, count-30); i < count; i++ {
		fmt.Fprintln(out, logs.read(i))
	}
	if err := logs.failure(); err != nil {
		return i18n.Errorf("tasks.ui.log_error", err)
	}
	return result.err
}

type tickMsg struct{}
type finishMsg struct{ err error }
type searchMsg struct {
	generation, through int
	ids                 []int
}
type cursor struct{ line, column int }
type reading struct {
	top    cursor
	follow bool
	seen   int
}
type rowCache struct {
	width, bytes int
	rows         map[int][]wrappedRow
}
type model struct {
	logs                                             *logStore
	tree                                             taskTree
	selected, width, height                          int
	readings                                         map[string]reading
	cache                                            *rowCache
	taskFocus, search, searching, finished, stopping bool
	query, editQuery                                 string
	matches                                          []int
	generation, searchThrough                        int
	searchCancel                                     context.CancelFunc
	ctx                                              context.Context
	cancel                                           context.CancelFunc
	done                                             <-chan struct{}
	result                                           *error
	err                                              error
	started, ended                                   time.Time
	source                                           StateSource
	states                                           map[string]State
	runningFocus                                     bool
	runningOffset, treeOffset                        int
	copy                                             *copyFrame
	copyMenu                                         bool
	copyNote                                         string
	clipboard                                        clipboardWriter
	rendered                                         *renderedFrame
}

func newModel(logs *logStore, graph Graph) model {
	m := model{logs: logs, tree: newTaskTree(graph), width: 80, height: 24, readings: map[string]reading{}, cache: &rowCache{}, ctx: context.Background(), states: map[string]State{}, clipboard: systemClipboard, rendered: &renderedFrame{}}
	for id, node := range m.tree.nodes {
		if node.parent > 0 && len(node.children) > 0 {
			m.tree.collapsed[id] = true
		}
	}
	return m
}
func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}
func (m model) Init() tea.Cmd {
	return tea.Batch(tick(), func() tea.Msg { <-m.done; return finishMsg{*m.result} })
}
func (m model) task() string {
	if m.selected == 0 {
		return ""
	}
	return m.tree.nodes[m.selected].task
}
func (m model) reading() reading {
	if r, ok := m.readings[m.task()]; ok {
		return r
	}
	return reading{follow: true}
}
func (m model) layout() (left, width, height int) {
	if m.width >= 70 {
		left = min(max(28, m.tree.width), min(52, m.width/2))
	} else if m.taskFocus {
		left = m.width
	}
	width = max(1, m.width-left)
	if left == m.width {
		width = m.width
	} else if left > 0 {
		width = max(1, width-2)
	}
	return left, width, max(1, m.height-len(m.headerLines())-1-len(m.footerLines()))
}
func (m model) count() int {
	if m.copy != nil {
		return len(m.copy.lines)
	}
	if m.query != "" {
		return len(m.matches)
	}
	return m.logs.count(m.task()) + len(m.logs.unfinished(m.task()))
}
func (m model) rows(line int) []wrappedRow {
	_, width, _ := m.layout()
	if m.copy != nil {
		if line < 0 || line >= len(m.copy.lines) {
			return []wrappedRow{{}}
		}
		cache := &m.copy.cache
		if cache.width != width || cache.rows == nil {
			cache.width = width
			cache.bytes = 0
			cache.rows = map[int][]wrappedRow{}
		}
		if rows, ok := cache.rows[line]; ok {
			return rows
		}
		text := m.copy.lines[line]
		rows := wrapText(text, width)
		if cache.bytes+len(text) > 2<<20 || len(cache.rows) >= 512 {
			cache.rows = map[int][]wrappedRow{}
			cache.bytes = 0
		}
		if len(text) <= 2<<20 {
			cache.rows[line] = rows
			cache.bytes += len(text)
		}
		return rows
	}

	if m.cache.width != width {
		m.cache.width = width
		m.cache.rows = map[int][]wrappedRow{}
		m.cache.bytes = 0
	}
	count := m.logs.count(m.task())
	var id int
	if m.query != "" {
		if line < 0 || line >= len(m.matches) {
			return []wrappedRow{{}}
		}
		id = m.matches[line]
	} else {
		if line >= count {
			pending := m.logs.unfinished(m.task())
			if line-count < len(pending) {
				return wrapText(pending[line-count], width)
			}
			return []wrappedRow{{}}
		}
		id = m.logs.id(m.task(), line)
	}
	if rows, ok := m.cache.rows[id]; ok {
		return rows
	}
	text := m.logs.read(id)
	rows := wrapText(text, width)
	if m.cache.bytes+len(text) > 2<<20 || len(m.cache.rows) >= 512 {
		m.cache.rows = map[int][]wrappedRow{}
		m.cache.bytes = 0
	}
	if len(text) <= 2<<20 {
		m.cache.rows[id] = rows
		m.cache.bytes += len(text)
	}
	return rows
}
func rowAt(rows []wrappedRow, column int) int {
	i := 0
	for i+1 < len(rows) && rows[i+1].column <= column {
		i++
	}
	return i
}
func (m model) move(c cursor, delta int) cursor {
	count := m.count()
	if count == 0 {
		return cursor{}
	}
	c.line = min(max(0, c.line), count-1)
	rows := m.rows(c.line)
	row := rowAt(rows, c.column)
	for delta < 0 {
		if row+delta >= 0 {
			row += delta
			delta = 0
			break
		}
		delta += row + 1
		if c.line == 0 {
			row = 0
			break
		}
		c.line--
		rows = m.rows(c.line)
		row = len(rows) - 1
	}
	for delta > 0 {
		if row+delta < len(rows) {
			row += delta
			break
		}
		delta -= len(rows) - row
		if c.line == count-1 {
			row = len(rows) - 1
			break
		}
		c.line++
		rows = m.rows(c.line)
		row = 0
	}
	c.column = rows[row].column
	return c
}
func (m model) bottom() cursor {
	count := m.count()
	if count == 0 {
		return cursor{}
	}
	_, _, height := m.layout()
	rows := m.rows(count - 1)
	return m.move(cursor{count - 1, rows[len(rows)-1].column}, -(height - 1))
}
func (m model) top() cursor {
	if m.copy != nil {
		return m.move(m.copy.top, 0)
	}
	r := m.reading()
	if r.follow {
		return m.bottom()
	}
	return m.move(r.top, 0)
}
func before(a, b cursor) bool { return a.line < b.line || a.line == b.line && a.column < b.column }
func (m *model) scroll(delta int) {
	if m.copy != nil {
		m.copy.top = m.move(m.top(), delta)
		return
	}
	r := m.reading()
	top := m.top()
	if r.follow {
		r.seen = m.count()
	}
	r.follow = false
	r.top = m.move(top, delta)
	if delta > 0 {
		bottom := m.bottom()
		if !before(r.top, bottom) {
			r.top = bottom
			r.follow = true
		}
	}
	m.readings[m.task()] = r
}
func (m *model) follow() { m.readings[m.task()] = reading{follow: true} }
func (m *model) beginSearch(reset bool) tea.Cmd {
	if reset {
		if m.searchCancel != nil {
			m.searchCancel()
		}
		m.generation++
		m.matches = nil
		m.searchThrough = 0
		m.searching = false
		m.follow()
	}
	if m.query == "" || m.searching || m.copy != nil {
		return nil
	}
	through := m.logs.count(m.task())
	if through == m.searchThrough {
		return nil
	}
	if m.searchCancel != nil {
		m.searchCancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.searchCancel = cancel
	m.searching = true
	generation, from, task, query, logs := m.generation, m.searchThrough, m.task(), m.query, m.logs
	return func() tea.Msg { return searchMsg{generation, through, logs.search(ctx, task, query, from, through)} }
}
func (m *model) selectTask(selected int) tea.Cmd {
	previous := m.task()
	m.selected = min(max(0, selected), len(m.tree.nodes)-1)
	if previous != m.task() {
		m.resumeCopy(false)
	}
	if m.query != "" && previous != m.task() {
		return m.beginSearch(true)
	}
	return nil
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case finishMsg:
		m.err = msg.err
		m.finished = true
		m.ended = time.Now()
		// The UI follows the scheduler's lifetime. Finite tasks must return to
		// the shell without input, even while the user is browsing or searching.
		return m, tea.Quit
	case tickMsg:
		m.refreshStates()
		if m.logs.failure() != nil && !m.stopping {
			m.stopping = true
			m.cancel()
		}
		if m.finished {
			return m, tea.Quit
		}
		cmd := m.beginSearch(false)
		return m, tea.Batch(tick(), cmd)
	case searchMsg:
		if msg.generation == m.generation {
			m.matches = append(m.matches, msg.ids...)
			m.searchThrough = msg.through
			m.searching = false
		}
	case copyResultMsg:
		if msg.err != nil {
			m.copyNote = i18n.Tf("tasks.ui.copy_failed", msg.err)
		} else if msg.remote {
			m.copyNote = i18n.T("tasks.ui.copy_requested")
			return m, tea.SetClipboard(msg.text)
		} else {
			m.copyNote = i18n.T("tasks.ui.copied")
		}
	case tea.WindowSizeMsg:
		// Keep the original column, not the nearest newly wrapped row: repeated
		// shrink/grow cycles must not gradually drift toward the start of a line.
		m.width = max(1, msg.Width)
		m.height = max(1, msg.Height)
		m.ensureSidebarSelection()
	case tea.MouseWheelMsg:
		if m.copy != nil && m.copy.native {
			return m, nil
		}
		delta := 0
		switch msg.Button {
		case tea.MouseWheelUp:
			delta = -3
		case tea.MouseWheelDown:
			delta = 3
		}
		left, _, height := m.layout()
		if left > 0 && msg.X < left && msg.Y >= len(m.headerLines()) && msg.Y < len(m.headerLines())+height {
			m.wheelSidebar(msg.Y-len(m.headerLines()), delta)
			return m, nil
		}
		m.scroll(delta)
	case tea.MouseClickMsg:
		if m.copy != nil && m.copy.native {
			return m, nil
		}
		left, _, height := m.layout()
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		if left > 0 && msg.X < left && msg.Y >= len(m.headerLines()) && msg.Y < len(m.headerLines())+height {
			cmd := m.clickSidebar(msg.Y - len(m.headerLines()))
			return m, cmd
		}
		if left != m.width && msg.X >= left && msg.Y >= len(m.headerLines()) && msg.Y < len(m.headerLines())+height {
			m.startSelection(msg.X, msg.Y)
		}
	case tea.MouseMotionMsg:
		if m.copy != nil && m.copy.dragging {
			m.copy.head = m.mouseCursor(msg.X, msg.Y)
		}
	case tea.MouseReleaseMsg:
		if m.copy != nil && m.copy.dragging {
			m.copy.head = m.mouseCursor(msg.X, msg.Y)
			m.copy.dragging = false
		}
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" || !m.search && key == "q" {
			if m.finished {
				return m, tea.Quit
			}
			m.stopping = true
			m.cancel()
			return m, nil
		}
		if m.search {
			switch key {
			case "enter":
				m.search = false
				m.query = m.editQuery
				cmd := m.beginSearch(true)
				return m, cmd
			case "esc":
				m.search = false
				m.editQuery = m.query
			case "backspace":
				_, n := utf8.DecodeLastRuneInString(m.editQuery)
				if n > 0 {
					m.editQuery = m.editQuery[:len(m.editQuery)-n]
				}
			default:
				if msg.Key().Text != "" {
					m.editQuery += msg.Key().Text
				}
			}
			return m, nil
		}
		if m.copy != nil && m.copy.native {
			switch key {
			case "esc", "c", "ctrl+s":
				m.resumeCopy(false)
			case "f", "f5":
				m.resumeCopy(true)
			}
			return m, nil
		}
		if m.copyMenu {
			m.copyMenu = false
			switch key {
			case "v":
				return m, m.clipboardCmd(m.visibleText())
			case "n":
				name := m.task()
				if name == "" {
					name = i18n.T("tasks.ui.all")
				}
				return m, m.clipboardCmd(name)
			case "l":
				return m, m.historyCopyCmd()
			case "esc":
				return m, nil
			default:
				m.copyMenu = true
				return m, nil
			}
		}

		_, _, height := m.layout()
		switch key {
		case "tab", "shift+tab":
			m.taskFocus = !m.taskFocus
		case "up", "k":
			if m.taskFocus {
				cmd := m.navigateTask(-1)
				return m, cmd
			}
			m.scroll(-1)
		case "down", "j":
			if m.taskFocus {
				cmd := m.navigateTask(1)
				return m, cmd
			}
			m.scroll(1)
		case "[":
			cmd := m.navigateTask(-1)
			return m, cmd
		case "]":
			cmd := m.navigateTask(1)
			return m, cmd
		case "a":
			m.runningFocus = false
			cmd := m.selectTask(0)
			return m, cmd
		case "left", "right", "enter", "space":
			if m.taskFocus {
				cmd := m.navigateTree(key)
				return m, cmd
			}
		case "pgup":
			if m.taskFocus {
				cmd := m.navigateTask(-height)
				return m, cmd
			}
			m.scroll(-height)
		case "pgdown":
			if m.taskFocus {
				cmd := m.navigateTask(height)
				return m, cmd
			}
			m.scroll(height)
		case "home", "end":
			if m.taskFocus {
				cmd := m.navigateTree(key)
				return m, cmd
			}
			if m.copy != nil {
				if key == "home" {
					m.copy.top = cursor{}
				} else {
					m.copy.top = m.bottom()
				}
				return m, nil
			}
			if key == "home" {
				m.readings[m.task()] = reading{seen: m.count()}
			} else {
				m.follow()
			}
		case "f", "f5":
			m.resumeCopy(true)
		case "c", "ctrl+s":
			m.nativeCopy()
		case "y":
			if text := m.selectedText(); text != "" {
				return m, m.clipboardCmd(text)
			}
			m.copyMenu = true
		case "/":
			m.resumeCopy(false)
			m.search = true
			m.editQuery = m.query
		case "esc":
			if m.copy != nil {
				m.resumeCopy(false)
				return m, nil
			}
			m.query = ""
			cmd := m.beginSearch(true)
			return m, cmd
		}
	}
	return m, nil
}
func (m model) page() []string {
	_, _, height := m.layout()
	page := make([]string, height)
	for i, row := range m.pageRows() {
		page[i] = m.highlighted(row)
	}
	return page
}
func (m model) View() tea.View {
	if m.copy != nil && m.copy.native {
		return m.copyView()
	}
	left, _, height := m.layout()
	rows := make([]string, height)
	if left != m.width {
		if m.copy == nil {
			// Render and retain the same immutable viewport. Incoming journal writes
			// between this frame and a mouse click cannot move the selection's text.
			snapshot := m.captureFrame()
			frozen := m
			frozen.copy = snapshot
			rows = frozen.page()
			if m.rendered != nil {
				*m.rendered = renderedFrame{frame: snapshot, task: m.task(), query: m.query, width: m.width, height: m.height}
			}
		} else {
			rows = m.page()
		}
	} else if m.rendered != nil {
		m.rendered.frame = nil
	}
	sidebar := m.sidebarRows()
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	for i := range rows {
		if left == 0 {
			continue
		}
		label := ""
		selected := false
		if i < len(sidebar) {
			row := sidebar[i]
			label = row.label
			selected = row.id >= 0 && row.id == m.selected && row.running == m.runningFocus
			marker := "  "
			if selected {
				marker = "> "
			} else if row.id > 0 && m.tree.nodes[row.id].task == m.task() {
				marker = "· "
			}
			label = marker + label
		}

		label = ansi.Truncate(label, left, "…")
		label += strings.Repeat(" ", max(0, left-ansi.StringWidth(label)))
		if selected {
			label = selectedStyle.Render(label)
		}
		if left == m.width {
			rows[i] = label
		} else {
			rows[i] = label + "│ " + rows[i]
		}
	}

	r := m.reading()
	note := i18n.T("tasks.ui.following")
	if !r.follow {
		note = i18n.Tf("tasks.ui.paused", max(0, m.count()-r.seen))
	}
	if m.searching {
		note = i18n.T("tasks.ui.searching")
	}
	if m.query != "" {
		note += " · / " + m.query
	}
	if m.tree.nodes[m.selected].reference {
		note = i18n.T("tasks.ui.shared") + " · " + note
	}
	if m.copy != nil {
		note = i18n.T("tasks.ui.selection")
	}
	if m.copyNote != "" {
		note = m.copyNote
	}
	content := m.headerLines()
	content = append(content, rows...)
	content = append(content, ansi.Truncate(note, m.width, "…"))
	content = append(content, m.footerLines()...)
	if len(content) > m.height {
		content = content[:m.height]
	}
	for i, line := range content {
		content[i] = ansi.Truncate(line, m.width, "…")
	}
	view := tea.NewView(strings.Join(content, "\n"))
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

func (m model) footerLines() []string {
	footer := i18n.T("tasks.ui.keys")
	if m.width < 70 {
		footer = i18n.T("tasks.ui.keys_compact")
	}
	if m.taskFocus {
		footer = i18n.T("tasks.ui.tree_keys")
		if m.width < 70 {
			footer = i18n.T("tasks.ui.tree_keys_compact")
		}
	}
	if m.copyMenu {
		footer = i18n.T("tasks.ui.copy_menu")
	} else if m.copy != nil {
		footer = i18n.T("tasks.ui.selection_keys")
	}
	if m.search {
		footer = "/ " + m.editQuery + "  " + i18n.T("tasks.ui.search")
	}
	lines := strings.Split(ansi.Hardwrap(footer, max(1, m.width), false), "\n")
	slots := max(1, min(3, m.height-3))
	if m.copy != nil {
		slots = min(slots, m.copy.footerSlots)
	}
	return lines[:min(len(lines), slots)]
}
