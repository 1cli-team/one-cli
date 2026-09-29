package taskui

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSilentRunningTasksAndCompletionKeepIdentity(t *testing.T) {
	graph := sharedGraph()
	logs := testStore(t, graph.names()...)
	m := newModel(logs, graph)
	now := time.Now()
	states := []State{{Name: "//:prepare", Status: "succeeded"}, {Name: "//:web:dev", Status: "running", StartedAt: now}, {Name: "//:api:dev", Status: "running", StartedAt: now.Add(-time.Second)}}
	m.source = func() []State { return states }
	m.refreshStates()
	if got := m.runningTasks(); !reflect.DeepEqual(got, []string{"//:api:dev", "//:web:dev"}) {
		t.Fatal(got)
	}
	if logs.count("") != 0 {
		t.Fatal("fixture should be silent")
	}
	m.navigateTask(1)
	if !m.runningFocus || m.task() != "//:api:dev" {
		t.Fatal("running shortcut not selected")
	}
	writeLog(t, newLogWriter(logs), "[//:api:dev\t] API_ONLY\n[//:web:dev\t] WEB_ONLY\n")
	m.readings[m.task()] = reading{top: cursor{}, seen: 1}
	m.query = "API"
	m.matches = []int{0}
	m.generation = 7
	m.freeze()
	before := m.reading()
	frame := m.copy
	states[2].Status = "succeeded"
	m.refreshStates()
	if m.runningFocus || m.task() != "//:api:dev" || m.reading() != before || m.copy != frame || m.generation != 7 || m.query != "API" {
		t.Fatal("completion changed selected logs or reading state")
	}
	if m.tree.collapsed[m.tree.primary["//:dev"]] {
		t.Fatal("completion failed to reveal selected node")
	}
	if page := plainPage(m); !strings.Contains(page, "API_ONLY") || strings.Contains(page, "WEB_ONLY") {
		t.Fatal(page)
	}
	if len(m.runningTasks()) != 1 {
		t.Fatal("finished task remains running")
	}
	if !strings.Contains(m.treeLabel(treeRow{id: m.selected}), "✓") {
		t.Fatal("tree lost completed status")
	}
}
func TestSidebarIndependentScrollAndStableSections(t *testing.T) {
	graph := Graph{}
	for i := range 40 {
		graph.Tasks = append(graph.Tasks, Task{Name: fmt.Sprintf("//:task-%02d", i)})
	}
	m := newModel(testStore(t, graph.names()...), graph)
	m.height = 24
	for i, task := range graph.Tasks {
		m.states[task.Name] = State{Name: task.Name, Status: "running", StartedAt: time.Unix(int64(i), 0)}
	}
	runHeight, treeHeight := m.sidebarHeights()
	m.wheelSidebar(2, 10)
	if m.runningOffset != 10 || m.treeOffset != 0 {
		t.Fatal("running wheel changed tree")
	}
	m.wheelSidebar(3+runHeight, 10)
	if m.treeOffset != 10 || m.runningOffset != 10 {
		t.Fatal("tree wheel changed running list")
	}
	m.selected = m.tree.primary[graph.Tasks[30].Name]
	m.runningFocus = true
	m.ensureSidebarSelection()
	if m.runningOffset <= 10 || m.treeOffset != 10 {
		t.Fatal("selection failed to scroll its own section")
	}
	m.states[graph.Tasks[0].Name] = State{Status: "succeeded"}
	if a, b := m.sidebarHeights(); a != runHeight || b != treeHeight {
		t.Fatal("completion moved tree section")
	}
	m.taskFocus = true
	m = key(m, tea.KeyRight, "")
	if m.runningFocus || m.task() != graph.Tasks[30].Name {
		t.Fatal("right failed to locate same identity in tree")
	}
	if !strings.Contains(strings.Join(sidebarLabels(m), "\n"), "task-30") {
		t.Fatal("located node not visible")
	}
}
func sidebarLabels(m model) []string {
	var labels []string
	for _, row := range m.sidebarRows() {
		labels = append(labels, row.label)
	}
	return labels
}
func TestInitialTreeDepthAndWaitReferences(t *testing.T) {
	graph := sharedGraph()
	graph.Tasks[2].WaitFor = []string{"//:api:dev"}
	m := newModel(testStore(t, graph.names()...), graph)
	if !m.tree.collapsed[m.tree.primary["//:api:dev"]] || m.tree.collapsed[m.tree.primary["//:dev"]] {
		t.Fatal("initial tree did not expose just entries and first dependencies")
	}
	m.reveal(m.tree.primary["//:prepare"])
	if m.tree.collapsed[m.tree.primary["//:api:dev"]] {
		t.Fatal("reveal did not expand ancestors")
	}
	found := false
	for id, node := range m.tree.nodes {
		if node.wait {
			found = true
			if !strings.Contains(m.treeLabel(treeRow{id: id}), "◇") {
				t.Fatal("waiting edge lost")
			}
		}
	}
	if !found {
		t.Fatal("wait edge missing")
	}
	old := i18n.Active()
	defer i18n.Init(old)
	for _, locale := range []string{"zh-CN", "en-US"} {
		i18n.Init(locale)
		if view := m.View().Content; !strings.Contains(view, i18n.T("tasks.ui.no_running")) {
			t.Fatal("empty state untranslated", view)
		}
	}
}

func TestStateRefreshDoesNotUndoIndependentWheelBrowsing(t *testing.T) {
	graph := Graph{}
	states := []State{}
	for i := range 40 {
		name := fmt.Sprintf("//:task-%02d", i)
		graph.Tasks = append(graph.Tasks, Task{Name: name})
		states = append(states, State{Name: name, Status: "running", StartedAt: time.Unix(int64(i), 0)})
	}
	m := newModel(testStore(t, graph.names()...), graph)
	m.source = func() []State { return states }
	m.refreshStates()
	m.navigateSidebar(1)
	m.wheelSidebar(2, 10)
	offset := m.runningOffset
	m.refreshStates()
	if m.runningOffset != offset {
		t.Fatal("state tick undid running-list browsing")
	}
	m.runningFocus = false
	m.selected = m.tree.primary[states[0].Name]
	runHeight, _ := m.sidebarHeights()
	m.wheelSidebar(3+runHeight, 10)
	offset = m.treeOffset
	m.refreshStates()
	if m.treeOffset != offset {
		t.Fatal("state tick undid tree browsing")
	}
}
