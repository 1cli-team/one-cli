package taskui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// Graph contains the selected invocation's resolved task identities and edges.
// The caller resolves mise scopes, expressions and aliases; this is display data.
type Graph struct {
	Entries []string
	Tasks   []Task
}
type Task struct {
	Name         string
	Dependencies []string
}

func (g Graph) names() []string {
	names := make([]string, 0, len(g.Tasks))
	for _, task := range g.Tasks {
		names = append(names, task.Name)
	}
	return names
}

type taskNode struct {
	task      string
	parent    int
	children  []int
	reference bool
}
type treeRow struct {
	id     int
	prefix string
}
type taskTree struct {
	nodes     []taskNode
	primary   map[string]int
	collapsed map[int]bool
	width     int
}

// Expand each task once. Other incoming edges become references, keeping a DAG
// readable without exponentially duplicating shared dependency subtrees.
func newTaskTree(g Graph) taskTree {
	tree := taskTree{nodes: []taskNode{{parent: -1}}, primary: map[string]int{}, collapsed: map[int]bool{}}
	tasks := map[string]Task{}
	referenced := map[string]bool{}
	for _, task := range g.Tasks {
		if task.Name != "" {
			tasks[task.Name] = task
		}
		for _, dep := range task.Dependencies {
			referenced[dep] = true
		}
	}
	var add func(string, int) int
	add = func(name string, parent int) int {
		id := len(tree.nodes)
		_, seen := tree.primary[name]
		tree.nodes = append(tree.nodes, taskNode{task: name, parent: parent, reference: seen})
		if seen {
			return id
		}
		tree.primary[name] = id
		dependencies := map[string]bool{}
		for _, dep := range tasks[name].Dependencies {
			if _, ok := tasks[dep]; !ok || dependencies[dep] {
				continue
			}
			dependencies[dep] = true
			child := add(dep, id)
			tree.nodes[id].children = append(tree.nodes[id].children, child)
		}
		return id
	}
	roots := g.Entries
	if len(roots) == 0 {
		for _, task := range g.Tasks {
			if !referenced[task.Name] {
				roots = append(roots, task.Name)
			}
		}
	}
	rootSeen := map[string]bool{}
	for _, name := range roots {
		if _, ok := tasks[name]; !ok || rootSeen[name] {
			continue
		}
		rootSeen[name] = true
		id := add(name, 0)
		tree.nodes[0].children = append(tree.nodes[0].children, id)
	}
	// Keep every planned task accessible, including disconnected or cyclic input.
	for _, task := range g.Tasks {
		if task.Name == "" {
			continue
		}
		if _, seen := tree.primary[task.Name]; !seen {
			id := add(task.Name, 0)
			tree.nodes[0].children = append(tree.nodes[0].children, id)
		}
	}
	for _, row := range tree.visible() {
		tree.width = max(tree.width, ansi.StringWidth(tree.label(row))+2)
	}
	return tree
}
func (t taskTree) visible() []treeRow {
	rows := []treeRow{{id: 0}}
	var walk func(int, string)
	walk = func(id int, prefix string) {
		if t.collapsed[id] {
			return
		}
		children := t.nodes[id].children
		for i, child := range children {
			branch, next := "├─ ", "│  "
			if i == len(children)-1 {
				branch, next = "└─ ", "   "
			}
			rows = append(rows, treeRow{child, prefix + branch})
			walk(child, prefix+next)
		}
	}
	for _, id := range t.nodes[0].children {
		rows = append(rows, treeRow{id: id})
		walk(id, "")
	}
	return rows
}
func (t taskTree) label(row treeRow) string {
	if row.id == 0 {
		return i18n.T("tasks.ui.all")
	}
	node := t.nodes[row.id]
	marker := ""
	if node.reference {
		marker = "↪ "
	} else if len(node.children) > 0 {
		marker = "▾ "
		if t.collapsed[row.id] {
			marker = "▸ "
		}
	}
	return row.prefix + marker + strings.TrimPrefix(node.task, "//:")
}
func (m model) treePage() ([]treeRow, int) {
	rows := m.tree.visible()
	selected := 0
	for i, row := range rows {
		if row.id == m.selected {
			selected = i
			break
		}
	}
	_, _, height := m.layout()
	return rows, max(0, selected-height+1)
}
func (m *model) moveTask(delta int) {
	rows := m.tree.visible()
	for i, row := range rows {
		if row.id == m.selected {
			m.selected = rows[min(max(0, i+delta), len(rows)-1)].id
			return
		}
	}
}
func (m *model) reveal(id int) {
	for parent := m.tree.nodes[id].parent; parent > 0; parent = m.tree.nodes[parent].parent {
		delete(m.tree.collapsed, parent)
	}
	m.selected = id
}

// treeKey handles navigation only; log selection/search is updated by the caller.
func (m *model) treeKey(key string) {
	node := m.tree.nodes[m.selected]
	switch key {
	case "left":
		if m.selected != 0 && len(node.children) > 0 && !m.tree.collapsed[m.selected] {
			m.tree.collapsed[m.selected] = true
		} else if node.parent >= 0 {
			m.selected = node.parent
		}
	case "right":
		if node.reference {
			m.reveal(m.tree.primary[node.task])
		} else if len(node.children) > 0 {
			if m.tree.collapsed[m.selected] {
				delete(m.tree.collapsed, m.selected)
			} else {
				m.selected = node.children[0]
			}
		}
	case "enter", "space":
		if node.reference {
			m.reveal(m.tree.primary[node.task])
		} else if m.selected != 0 && len(node.children) > 0 {
			m.tree.collapsed[m.selected] = !m.tree.collapsed[m.selected]
		}
	case "home":
		m.selected = 0
	case "end":
		rows := m.tree.visible()
		m.selected = rows[len(rows)-1].id
	}
}

func (m *model) navigateTask(delta int) tea.Cmd {
	previous := m.selected
	m.moveTask(delta)
	selected := m.selected
	m.selected = previous
	return m.selectTask(selected)
}
func (m *model) navigateTree(key string) tea.Cmd {
	previous := m.selected
	m.treeKey(key)
	selected := m.selected
	m.selected = previous
	return m.selectTask(selected)
}
