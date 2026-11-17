package issueworkflow

import (
	"slices"
	"strings"

	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
)

func defaultTeamFlow(team tracking.Team) TeamFlow {
	review := pickState(team.States, "started", "", false, []string{
		"in review", "review", "code review", "in code review", "ready for review", "awaiting review",
		"待审", "待审核", "待评审", "审核中", "评审中", "代码审查",
	})
	running := pickState(team.States, "started", review, true, []string{
		"in progress", "started", "in development", "implementing", "进行中", "处理中", "开发中",
	})
	done := pickState(team.States, "completed", "", true, []string{"done", "completed", "完成", "已完成"})
	if review == "" {
		review = running
	}
	return TeamFlow{Running: running, Review: review, Done: done}
}

func pickState(states []tracking.State, kind, excludedID string, allowSingle bool, names []string) string {
	var available, named []string
	for _, state := range states {
		if state.Type != kind || state.ID == "" || state.ID == excludedID {
			continue
		}
		available = append(available, state.ID)
		name := strings.ToLower(strings.Join(strings.Fields(state.Name), " "))
		if slices.Contains(names, name) {
			named = append(named, state.ID)
		}
	}
	if len(named) == 1 {
		return named[0]
	}
	if allowSingle && len(available) == 1 {
		return available[0]
	}
	return ""
}
