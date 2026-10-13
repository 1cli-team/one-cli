package taskui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type sidebarRow struct {
	id      int
	running bool
	label   string
}

// Running tasks take priority; the tree uses the rows above the pinned All logs entry.
func (m model) sidebarHeights() (running, tree int) {
	_, _, height := m.layout()
	running = min(max(1, len(m.runningTasks())), max(0, height-3))
	return running, max(0, height-running-5)
}
func (m model) sidebarItems() []sidebarRow {
	items := []sidebarRow{}
	for _, row := range m.sidebarRows() {
		if row.running {
			items = append(items, row)
		}
	}
	if _, treeHeight := m.sidebarHeights(); treeHeight > 0 {
		for _, row := range m.tree.visible()[1:] {
			items = append(items, sidebarRow{id: row.id})
		}
	}
	return append(items, sidebarRow{id: 0})
}
func (m *model) ensureSidebarSelection() {
	m.clampSidebarOffsets()
	if m.runningFocus {
		return
	}
	_, treeHeight := m.sidebarHeights()
	if treeHeight == 0 {
		return
	}
	for i, row := range m.tree.visible()[1:] {
		if row.id == m.selected {
			m.treeOffset = keepVisible(i, m.treeOffset, treeHeight)
		}
	}
}
func (m *model) clampSidebarOffsets() {
	_, treeHeight := m.sidebarHeights()
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
	rows[height-1] = sidebarRow{id: 0, label: i18n.T("tasks.ui.all")}
	if height < 3 {
		return rows
	}
	runningHeight, treeHeight := m.sidebarHeights()
	tree := m.tree.visible()[1:]
	runs := m.runningTasks()
	rows[0].label = i18n.Tf("tasks.ui.running_list", len(runs))
	for i := 0; i < runningHeight; i++ {
		if len(runs) > runningHeight && i == runningHeight-1 {
			rows[1+i].label = i18n.Tf("tasks.ui.running_overflow", len(runs)-i)
		} else if i < len(runs) {
			name := runs[i]
			duration := ""
			if at := m.states[name].StartedAt; !at.IsZero() {
				duration = "  " + timeSince(at)
			}
			rows[1+i] = sidebarRow{id: m.tree.primary[name], running: true, label: stateMarker("running") + " " + strings.TrimPrefix(name, "//:") + duration}
		} else if len(runs) == 0 && i == 0 {
			rows[1+i].label = i18n.T("tasks.ui.no_running")
		}
	}
	if treeHeight == 0 {
		return rows
	}
	start := 2 + runningHeight
	rows[start].label = i18n.Tf("tasks.ui.task_tree", len(m.tree.primary))
	start++
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
	start := 2 + runs
	if trees > 0 && index >= start && index < start+1+trees {
		m.treeOffset = min(max(0, m.treeOffset+delta), max(0, len(m.tree.visible())-1-trees))
	}
}
func timeSince(at time.Time) string { return time.Since(at).Truncate(time.Second).String() }
