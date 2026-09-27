package execution

import (
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// SelectProjects resolves names and paths once, preserving selection order.
// An empty selection denotes all projects. Legacy -p can alias one positional.
func (w Workspace) SelectProjects(args []string, legacy string) ([]string, error) {
	if legacy != "" {
		if len(args) > 1 {
			return nil, i18n.Errorf("task.selector_conflict")
		}
		if len(args) == 1 {
			a, aOK := w.Project(args[0])
			b, bOK := w.Project(legacy)
			if !aOK || !bOK || a.Name != b.Name {
				return nil, i18n.Errorf("task.selector_mismatch")
			}
		} else {
			args = []string{legacy}
		}
	}
	var names []string
	seen := map[string]bool{}
	for _, selector := range args {
		p, ok := w.Project(strings.TrimSpace(selector))
		if !ok {
			return nil, cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND, i18n.Tf("workspace.unknown_project", selector)).WithContext(map[string]any{"selector": selector, "available_projects": w.ProjectNames()})
		}
		if !seen[p.Name] {
			names = append(names, p.Name)
			seen[p.Name] = true
		}
	}
	return names, nil
}
