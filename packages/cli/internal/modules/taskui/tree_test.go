package taskui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func sharedGraph() Graph {
	return Graph{Entries: []string{"//:dev"}, Tasks: []Task{
		{Name: "//:prepare"},
		{Name: "//:api:dev", Dependencies: []string{"//:prepare"}},
		{Name: "//:web:dev", Dependencies: []string{"//:prepare"}},
		{Name: "//:dev", Dependencies: []string{"//:api:dev", "//:web:dev"}},
	}}
}
func TestDependencyTreePreservesSharedEdges(t *testing.T) {
	tree := newTaskTree(sharedGraph())
	var labels []string
	for _, row := range tree.visible()[1:] {
		labels = append(labels, tree.label(row))
	}
	want := []string{"▾ dev", "├─ ▾ api:dev", "│  └─ prepare", "└─ ▾ web:dev", "   └─ ↪ prepare"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("tree=%q want=%q", labels, want)
	}
	// Multiple explicit entries remain roots even if one is also a dependency.
	graph := sharedGraph()
	graph.Entries = []string{"//:api:dev", "//:web:dev", "//:prepare"}
	tree = newTaskTree(graph)
	if roots := tree.nodes[0].children; len(roots) != 4 || !tree.nodes[roots[2]].reference {
		t.Fatalf("entries or disconnected task lost: %+v", tree.nodes)
	}
	graph = Graph{Tasks: []Task{{Name: "a", Dependencies: []string{"b"}}, {Name: "b", Dependencies: []string{"a", "missing"}}}}
	tree = newTaskTree(graph)
	if len(tree.nodes) != 4 || !tree.nodes[3].reference {
		t.Fatalf("cycle not bounded: %+v", tree.nodes)
	}
}
func TestTreeNavigationSharesTaskLogsAndReadingPosition(t *testing.T) {
	graph := sharedGraph()
	s := testStore(t, graph.names()...)
	w := newLogWriter(s)
	for range 40 {
		writeLog(t, w, "[prepare] shared output\n[web:dev] web only\n")
	}
	m := newModel(s, graph)
	m.height = 14
	m = key(m, tea.KeyTab, "")
	m = key(m, tea.KeyDown, "") // dev
	m = key(m, tea.KeyLeft, "") // collapse dev
	if len(m.tree.visible()) != 2 {
		t.Fatal("root did not collapse")
	}
	m = key(m, tea.KeyRight, "") // expand dev
	m = key(m, tea.KeyRight, "") // api
	m = key(m, tea.KeyRight, "") // prepare (primary)
	primary := m.selected
	if m.task() != "//:prepare" {
		t.Fatal("wrong dependency selected", m.task())
	}
	m = key(m, tea.KeyTab, "") // logs
	m = key(m, tea.KeyHome, "")
	m = key(m, tea.KeyDown, "")
	position := m.reading()
	m = key(m, tea.KeyTab, "") // tree
	m = key(m, tea.KeyEnd, "") // prepare (reference)
	if !m.tree.nodes[m.selected].reference || m.reading() != position {
		t.Fatal("shared reference lost reading position")
	}
	if page := plainPage(m); !strings.Contains(page, "shared output") || strings.Contains(page, "web only") {
		t.Fatal("reference selected wrong logs", page)
	}
	// A search stays valid when selecting another occurrence of the same task.
	m.query = "shared"
	cmd := m.beginSearch(true)
	updated, _ := m.Update(cmd())
	m = updated.(model)
	generation := m.generation
	m.tree.collapsed[m.tree.primary["//:api:dev"]] = true
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(model)
	if cmd != nil || m.selected != primary || m.tree.collapsed[m.tree.primary["//:api:dev"]] || m.generation != generation || len(m.matches) != 40 {
		t.Fatal("reference jump did not reveal primary with same search")
	}
	// Switching to a different node must search only that task's history.
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m = updated.(model)
	if m.task() != "//:web:dev" || cmd == nil {
		t.Fatal("task switch did not restart search")
	}
	updated, _ = m.Update(cmd())
	m = updated.(model)
	if len(m.matches) != 0 {
		t.Fatal("search results leaked between tasks")
	}
}
func TestTreeMouseTargetsVisibleNodes(t *testing.T) {
	graph := sharedGraph()
	m := newModel(testStore(t, graph.names()...), graph)
	m.selected = m.tree.primary["//:api:dev"]
	m.tree.collapsed[m.selected] = true
	updated, _ := m.Update(tea.MouseClickMsg{X: 3, Y: 4, Button: tea.MouseLeft})
	m = updated.(model)
	if m.task() != "//:web:dev" || !m.taskFocus {
		t.Fatal("click selected a hidden node", m.task())
	}
	updated, _ = m.Update(tea.MouseWheelMsg{X: 3, Y: 4, Button: tea.MouseWheelDown})
	m = updated.(model)
	if !m.tree.nodes[m.selected].reference || !m.reading().follow {
		t.Fatal("tree wheel scrolled logs")
	}
	updated, _ = m.Update(tea.MouseWheelMsg{X: 75, Y: 4, Button: tea.MouseWheelUp})
	m = updated.(model)
	if m.reading().follow {
		t.Fatal("log wheel navigated tree")
	}
}
func TestTreeResizeAndLocales(t *testing.T) {
	old := i18n.Active()
	t.Cleanup(func() { i18n.Init(old) })
	graph := Graph{Entries: []string{"//:开发"}, Tasks: []Task{
		{Name: "//:开发", Dependencies: []string{"//:中文🧑‍💻任务很长很长很长很长很长很长"}},
		{Name: "//:中文🧑‍💻任务很长很长很长很长很长很长"},
	}}
	s := testStore(t, graph.names()...)
	writeLog(t, newLogWriter(s), "[开发] LOG_VISIBLE\n")
	for _, locale := range []string{"zh-CN", "en-US"} {
		i18n.Init(locale)
		for _, size := range [][2]int{{1, 1}, {2, 2}, {38, 10}, {69, 20}, {70, 20}, {110, 25}, {180, 30}} {
			for _, focus := range []bool{false, true} {
				m := newModel(s, graph)
				m.width, m.height, m.taskFocus = size[0], size[1], focus
				lines := strings.Split(m.View().Content, "\n")
				if len(lines) > m.height {
					t.Fatal("height overflow", size)
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > m.width {
						t.Fatalf("width overflow %v: %q", size, line)
					}
				}
			}
		}
		m := newModel(s, graph)
		m.width = 45
		if !strings.Contains(m.View().Content, "LOG_VISIBLE") {
			t.Fatal("narrow log pane missing")
		}
		m = key(m, tea.KeyTab, "")
		if view := m.View().Content; strings.Contains(view, "LOG_VISIBLE") || !strings.Contains(view, "└─ ") || !strings.Contains(view, i18n.T("tasks.ui.focus_tasks")) {
			t.Fatal("narrow tree pane missing", view)
		}
		m = key(m, tea.KeyTab, "")
		if !strings.Contains(m.View().Content, "LOG_VISIBLE") {
			t.Fatal("tab did not restore narrow logs")
		}
	}
}

func TestTreeClickUsesRowsBeforeFocusChangesFooter(t *testing.T) {
	old := i18n.Active()
	i18n.Init("en-US")
	t.Cleanup(func() { i18n.Init(old) })
	graph := Graph{}
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		graph.Tasks = append(graph.Tasks, Task{Name: name})
	}
	m := newModel(testStore(t, graph.names()...), graph)
	m.width, m.height, m.selected = 80, 10, len(graph.Tasks)
	rows, first := m.treePage()
	want := rows[first+1].id
	updated, _ := m.Update(tea.MouseClickMsg{X: 3, Y: 2, Button: tea.MouseLeft})
	if got := updated.(model); got.selected != want {
		t.Fatalf("focus changed clicked row: got=%d want=%d", got.selected, want)
	}
}
