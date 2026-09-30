package taskui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

type sidebarRow struct {
	id      int
	running bool
	label   string
}

// Heights depend on terminal size, never on how many tasks happen to be running.
func (m model) sidebarHeights() (running, tree int) {
	_, _, height := m.layout()
	if height < 5 {
		return 0, max(0, height-1)
	}
	running = max(1, (height-3)/3)
	return running, height - running - 3
}
func (m model) sidebarItems() []sidebarRow {
	items := []sidebarRow{{id: 0}}
	for _, name := range m.runningTasks() {
		items = append(items, sidebarRow{id: m.tree.primary[name], running: true})
	}
	for _, row := range m.tree.visible()[1:] {
		items = append(items, sidebarRow{id: row.id})
	}
	return items
}
func (m *model) ensureSidebarSelection() {
	m.clampSidebarOffsets()
	runningHeight, treeHeight := m.sidebarHeights()
	runs := m.runningTasks()
	tree := m.tree.visible()[1:]
	if m.runningFocus {
		for i, name := range runs {
			if name == m.task() {
				m.runningOffset = keepVisible(i, m.runningOffset, runningHeight)
			}
		}
	} else {
		for i, row := range tree {
			if row.id == m.selected {
				m.treeOffset = keepVisible(i, m.treeOffset, treeHeight)
			}
		}
	}
}
func (m *model) clampSidebarOffsets() {
	runningHeight, treeHeight := m.sidebarHeights()
	m.runningOffset = min(max(0, m.runningOffset), max(0, len(m.runningTasks())-runningHeight))
	m.treeOffset = min(max(0, m.treeOffset), max(0, len(m.tree.visible())-1-treeHeight))
}
func keepVisible(index, offset, height int) int {
	if index < offset {
		return index
	}
	if index >= offset+height {
		return max(0, index-height+1)
	}
	return offset
}
func (m model) sidebarRows() []sidebarRow {
	_, _, height := m.layout()
	rows := make([]sidebarRow, height)
	for i := range rows {
		rows[i].id = -1
	}
	if height == 0 {
		return rows
	}
	rows[0] = sidebarRow{id: 0, label: i18n.T("tasks.ui.all")}
	runningHeight, treeHeight := m.sidebarHeights()
	tree := m.tree.visible()[1:]
	start := 1
	if runningHeight > 0 {
		runs := m.runningTasks()
		rows[1].label = i18n.Tf("tasks.ui.running_list", len(runs))
		for i := 0; i < runningHeight; i++ {
			index := m.runningOffset + i
			if index < len(runs) {
				name := runs[index]
				duration := ""
				if at := m.states[name].StartedAt; !at.IsZero() {
					duration = "  " + timeSince(at)
				}
				rows[2+i] = sidebarRow{id: m.tree.primary[name], running: true, label: stateMarker("running") + " " + strings.TrimPrefix(name, "//:") + duration}
			} else if len(runs) == 0 && i == 0 {
				rows[2+i].label = i18n.T("tasks.ui.no_running")
			}
		}
		start = 2 + runningHeight
		rows[start].label = i18n.Tf("tasks.ui.task_tree", len(m.tree.primary))
		start++
	}
	for i := 0; i < treeHeight && start+i < height; i++ {
		index := m.treeOffset + i
		if index < len(tree) {
			row := tree[index]
			rows[start+i] = sidebarRow{id: row.id, label: m.treeLabel(row)}
		}
	}
	return rows
}
func (m model) treeLabel(row treeRow) string {
	label := m.tree.label(row)
	node := m.tree.nodes[row.id]
	prefix := row.prefix
	label = strings.TrimPrefix(label, prefix)
	return prefix + stateMarker(m.state(node.task).Status) + " " + label
}
func (m *model) navigateSidebar(delta int) tea.Cmd {
	items := m.sidebarItems()
	current := 0
	for i, row := range items {
		if row.id == m.selected && row.running == m.runningFocus {
			current = i
			break
		}
	}
	row := items[min(max(0, current+delta), len(items)-1)]
	m.runningFocus = row.running
	cmd := m.selectTask(row.id)
	m.ensureSidebarSelection()
	return cmd
}
func (m *model) clickSidebar(index int) tea.Cmd {
	rows := m.sidebarRows()
	if index < 0 || index >= len(rows) || rows[index].id < 0 {
		return nil
	}
	row := rows[index]
	m.taskFocus = true
	m.runningFocus = row.running
	cmd := m.selectTask(row.id)
	m.ensureSidebarSelection()
	return cmd
}
func (m *model) wheelSidebar(index, delta int) {
	runs, trees := m.sidebarHeights()
	if runs > 0 && index >= 1 && index < 2+runs {
		m.runningOffset = min(max(0, m.runningOffset+delta), max(0, len(m.runningTasks())-runs))
	} else {
		m.treeOffset = min(max(0, m.treeOffset+delta), max(0, len(m.tree.visible())-1-trees))
	}
}
func (m model) headerLines() []string {
	if m.copy != nil && len(m.copy.header) > 0 {
		var lines []string
		for _, line := range m.copy.header {
			lines = append(lines, strings.Split(ansi.Hardwrap(line, max(1, m.width), false), "\n")...)
		}
		return lines[:min(len(lines), max(2, 1+m.height/3))]
	}
	header := i18n.T("tasks.ui.running")
	if m.finished {
		header = i18n.T("tasks.ui.completed")
		if m.err != nil {
			header = i18n.T("tasks.ui.failed")
		}
	}
	if !m.started.IsZero() {
		end := time.Now()
		if m.finished {
			end = m.ended
		}
		header += "  " + end.Sub(m.started).Truncate(time.Second).String()
	}
	focus := i18n.T("tasks.ui.focus_logs")
	if m.taskFocus {
		focus = i18n.T("tasks.ui.focus_tasks")
	}
	header += " · " + focus
	task := m.task()
	if task == "" {
		task = i18n.T("tasks.ui.all")
	}
	if status := m.statusLabel(); status != "" {
		task += " · " + status
	}
	lines := []string{ansi.Truncate(header, m.width, "…")}
	nameLines := strings.Split(ansi.Hardwrap(task, max(1, m.width), false), "\n")
	// The complete identity wraps independently of sidebar truncation. Tiny
	// terminals retain a usable log row instead of allowing headers to overflow.
	limit := max(1, m.height/3)
	lines = append(lines, nameLines[:min(limit, len(nameLines))]...)
	return lines
}

func timeSince(at time.Time) string { return time.Since(at).Truncate(time.Second).String() }
