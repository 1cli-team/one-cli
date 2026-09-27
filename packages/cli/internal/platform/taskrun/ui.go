package taskrun

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
)

type tickMsg struct{}
type model struct {
	s                                *Session
	selected, width, height          int
	input, help, searching, hideList bool
	query                            string
}

func (s *Session) show() error {
	_, err := tea.NewProgram(model{s: s, width: 80, height: 24}, tea.WithoutSignalHandler()).Run()
	return err
}
func (m model) Init() tea.Cmd { return tick() }
func tick() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		select {
		case <-m.s.done:
			return m, tea.Quit
		default:
			return m, tick()
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
	case tea.PasteMsg:
		if m.input {
			m.s.mu.Lock()
			m.s.jobs[m.selected].terminal.Paste(msg.Content)
			m.s.mu.Unlock()
		}
	case tea.KeyPressMsg:
		key := msg.String()
		if m.input {
			if key == "ctrl+]" {
				m.input = false
				return m, nil
			}
			m.s.mu.Lock()
			m.s.jobs[m.selected].terminal.SendKey(uv.KeyPressEvent(msg))
			m.s.mu.Unlock()
			return m, nil
		}
		if key == "ctrl+c" {
			m.s.cancel(&platformprocess.ExitStatus{Code: 130})
			return m, nil
		}
		if m.searching {
			switch key {
			case "enter", "esc":
				m.searching = false
			case "backspace":
				r := []rune(m.query)
				if len(r) > 0 {
					m.query = string(r[:len(r)-1])
				}
			default:
				if msg.Text != "" {
					m.query += msg.Text
				}
			}
			m.ensureSelection()
			return m, nil
		}
		switch key {
		case "q":
			m.s.cancel(&platformprocess.ExitStatus{Code: 130})
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
		case "enter", "i":
			m.s.mu.Lock()
			m.input = m.s.jobs[m.selected].result.Status == "running"
			m.s.jobs[m.selected].offset = 0
			m.s.mu.Unlock()
		case "r", "s":
			if m.s.opts.Development {
				action := "stop"
				if key == "r" {
					action = "restart"
				}
				select {
				case m.s.controls <- control{m.selected, action}:
				default:
				}
			}
		case "/":
			m.searching = true
		case "esc":
			m.query = ""
			m.help = false
		case "?":
			m.help = !m.help
		case "h":
			m.hideList = !m.hideList
			m.resize()
		case "pgup", "u":
			m.scroll(max(1, m.height/2))
		case "pgdown", "d":
			m.scroll(-max(1, m.height/2))
		case "end", "f":
			m.s.mu.Lock()
			m.s.jobs[m.selected].offset = 0
			m.s.mu.Unlock()
		}
	}
	return m, nil
}
func (m model) listWidth() int {
	if m.hideList || m.width < 70 {
		return 0
	}
	return min(28, m.width/3)
}
func (m model) resize() {
	w, h := max(10, m.width-m.listWidth()-3), max(2, m.height-4)
	m.s.mu.Lock()
	defer m.s.mu.Unlock()
	m.s.width, m.s.height = w, h
	for _, j := range m.s.jobs {
		j.terminal.Resize(w, h)
		j.ioMu.Lock()
		if j.child != nil {
			_ = j.child.Resize(w, h)
		}
		j.ioMu.Unlock()
	}
}
func (m model) visible() []int {
	var indexes []int
	for i, j := range m.s.jobs {
		if strings.Contains(strings.ToLower(j.task.Name), strings.ToLower(m.query)) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}
func (m *model) ensureSelection() {
	v := m.visible()
	for _, i := range v {
		if i == m.selected {
			return
		}
	}
	if len(v) > 0 {
		m.selected = v[0]
	}
}
func (m *model) move(delta int) {
	v := m.visible()
	for pos, i := range v {
		if i == m.selected {
			m.selected = v[(pos+delta+len(v))%len(v)]
			return
		}
	}
	m.ensureSelection()
}
func (m model) scroll(delta int) {
	m.s.mu.Lock()
	defer m.s.mu.Unlock()
	j := m.s.jobs[m.selected]
	j.offset = max(0, min(j.terminal.ScrollbackLen(), j.offset+delta))
}

var accent = lipgloss.NewStyle().Foreground(lipgloss.Color("#ea580c")).Bold(true)
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

func (m model) View() tea.View {
	s := m.s
	s.mu.Lock()
	defer s.mu.Unlock()
	width, height := max(20, m.width), max(6, m.height)
	panelHeight := height - 4
	j := s.jobs[m.selected]
	running, failed := 0, 0
	for _, job := range s.jobs {
		if job.result.Status == "running" {
			running++
		}
		if job.result.Status == "failed" {
			failed++
		}
	}
	title := i18n.Tf("task.ui.title", s.opts.Title, running, failed)
	header := accent.Render(ansi.Truncate(title, width, "…"))
	detail := fmt.Sprintf("%s · %s", j.task.Name, i18n.T("task.status."+j.result.Status))
	if j.result.Status == "running" {
		detail += fmt.Sprintf(" · %s", time.Since(j.started).Truncate(time.Second))
	} else if j.result.ExitCode != 0 {
		detail += i18n.Tf("task.ui.exit", j.result.ExitCode)
	}
	if j.attempt > 1 {
		detail += i18n.Tf("task.ui.attempt", j.attempt)
	}
	if j.terminal.ScrollbackLen() >= 3000 {
		detail += i18n.T("task.ui.history_limit")
	}
	if j.offset > 0 {
		detail += i18n.T("task.ui.history")
	}
	body := terminalView(j, panelHeight)
	if m.help {
		body = i18n.T("task.ui.help")
		if s.opts.Development {
			body += i18n.T("task.ui.help_dev")
		}
	}
	lw := m.listWidth()
	if lw > 0 {
		rows := []string{}
		visible := m.visible()
		start := 0
		for n, i := range visible {
			if i == m.selected {
				start = max(0, n-panelHeight+1)
				break
			}
		}
		for _, i := range visible[start:] {
			job := s.jobs[i]
			label := "  " + job.task.Name + " · " + i18n.T("task.status."+job.result.Status)
			if i == m.selected {
				label = "> " + job.task.Name + " · " + i18n.T("task.status."+job.result.Status)
			}
			label = ansi.Truncate(label, lw-2, "…")
			if i == m.selected {
				label = accent.Render(label)
			}
			rows = append(rows, label)
			if len(rows) >= panelHeight {
				break
			}
		}
		left := lipgloss.NewStyle().Width(lw).Height(panelHeight).Render(strings.Join(rows, "\n"))
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, " │ ", body)
	}
	hint := i18n.T("task.ui.hint")
	if s.opts.Development {
		hint = i18n.T("task.ui.hint_dev")
	}
	if m.input {
		hint = i18n.Tf("task.ui.input", j.task.Name)
	}
	if m.searching || m.query != "" {
		hint = i18n.Tf("task.ui.search", m.query)
	}
	v := tea.NewView(header + "\n" + muted.Render(ansi.Truncate(detail, width, "…")) + "\n" + lipgloss.NewStyle().Height(panelHeight).MaxHeight(panelHeight).Render(body) + "\n" + muted.Render(ansi.Truncate(hint, width, "…")))
	v.AltScreen = true
	if m.input && j.offset == 0 {
		pos := j.terminal.CursorPosition()
		v.Cursor = &tea.Cursor{Position: tea.Position{X: pos.X + max(0, lw) + func() int {
			if lw > 0 {
				return 3
			}
			return 0
		}(), Y: pos.Y + 2}}
	}
	return v
}
func terminalView(j *job, height int) string {
	t := j.terminal
	if j.offset == 0 {
		return t.Render()
	}
	end := t.ScrollbackLen() + t.Height() - j.offset
	start := max(0, end-height)
	lines := make([]string, 0, height)
	for y := start; y < end; y++ {
		line := make(uv.Line, t.Width())
		for x := 0; x < t.Width(); x++ {
			var c *uv.Cell
			if y < t.ScrollbackLen() {
				c = t.ScrollbackCellAt(x, y)
			} else {
				c = t.CellAt(x, y-t.ScrollbackLen())
			}
			if c != nil {
				line[x] = *c
			}
		}
		lines = append(lines, line.Render())
	}
	return strings.Join(lines, "\n")
}
func (s *Session) printFailures() {
	for _, j := range s.jobs {
		if j.result.Status != "failed" {
			continue
		}
		fmt.Fprintf(s.opts.Output, i18n.T("task.ui.failed_output"), j.task.Name, j.result.ExitCode, strings.TrimSpace(j.terminal.String()))
	}
}
