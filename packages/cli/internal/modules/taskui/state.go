package taskui

import (
	"sort"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// StateSource returns immutable snapshots from the invocation's worker events.
// Neither log activity nor time without output changes task state.
type StateSource func() []State
type State struct {
	Name, Status       string
	StartedAt, EndedAt time.Time
	ExitCode           int
}

func (m *model) refreshStates() {
	if m.source == nil {
		return
	}
	for _, state := range m.source() {
		if _, exists := m.tree.primary[state.Name]; exists {
			m.states[state.Name] = state
		}
	}
	if m.runningFocus && m.states[m.task()].Status != "running" {
		m.runningFocus = false
		m.reveal(m.tree.primary[m.task()])
		m.ensureSidebarSelection()
	} else {
		m.clampSidebarOffsets()
	}
}
func (m model) runningTasks() []string {
	names := []string{}
	for name, state := range m.states {
		if state.Status == "running" {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := m.states[names[i]].StartedAt, m.states[names[j]].StartedAt
		if a.Equal(b) {
			return m.tree.primary[names[i]] < m.tree.primary[names[j]]
		}
		return a.Before(b)
	})
	return names
}
func (m model) state(name string) State {
	state := m.states[name]
	if state.Status == "" {
		state.Status = "waiting"
	}
	return state
}
func stateSymbol(status string) string {
	switch status {
	case "running":
		return "●"
	case "succeeded":
		return "✓"
	case "failed":
		return "×"
	case "cancelled":
		return "■"
	case "cached":
		return "≡"
	case "waiting":
		return "○"
	default:
		return "?"
	}
}
func (m model) statusLabel() string {
	if m.task() == "" {
		return ""
	}
	state := m.state(m.task())
	label := i18n.T("tasks.ui.state." + state.Status)
	if state.Status == "failed" {
		label = i18n.Tf("tasks.ui.state.exit", state.ExitCode)
	}
	return stateMarker(state.Status) + " " + label
}

func stateMarker(status string) string {
	color := "8"
	switch status {
	case "running":
		color = "10"
	case "failed":
		color = "9"
	case "cancelled":
		color = "11"
	case "cached":
		color = "14"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(stateSymbol(status))
}
