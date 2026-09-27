package execution

import (
	"fmt"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

// SelectProjects resolves names and paths once, preserving selection order.
// An empty selection denotes all projects. Legacy -p can alias one positional.
func (w Workspace) SelectProjects(args []string, legacy string) ([]string, error) {
	if legacy != "" {
		if len(args) > 1 {
			return nil, fmt.Errorf("use multiple positional projects or --project, not both")
		}
		if len(args) == 1 {
			a, aOK := w.Project(args[0])
			b, bOK := w.Project(legacy)
			if !aOK || !bOK || a.Name != b.Name {
				return nil, fmt.Errorf("positional project and --project must select the same project")
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
			return nil, cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND, "Unknown project: "+selector).WithContext(map[string]any{"selector": selector, "available_projects": w.ProjectNames()})
		}
		if !seen[p.Name] {
			names = append(names, p.Name)
			seen[p.Name] = true
		}
	}
	return names, nil
}
