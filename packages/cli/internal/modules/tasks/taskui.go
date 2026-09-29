package tasks

import (
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/taskui"
)

// taskGraph projects an already-selected plan; it never schedules tasks or
// changes the commands. Runtime identities also match mise's emitted labels.
func taskGraph(p *Plan) taskui.Graph {
	canonical := func(name string) string {
		if strings.HasPrefix(name, "//") {
			return name
		}
		return "//:" + name
	}
	identities := map[string]string{}
	for _, task := range p.Tasks {
		identities[task.Name] = canonical(task.runtimeName())
	}
	graph := taskui.Graph{}
	for _, entry := range p.Entries {
		if name, ok := identities[entry]; ok {
			graph.Entries = append(graph.Entries, name)
		}
	}
	added := map[string]bool{}
	for _, task := range p.Tasks {
		name := identities[task.Name]
		if added[name] {
			continue
		}
		added[name] = true
		node := taskui.Task{Name: name}
		seen := map[string]bool{}
		for _, pattern := range task.Dependencies {
			if !strings.HasPrefix(pattern, "//") {
				scope, _, _ := strings.Cut(task.Name, ":")
				pattern = scope + ":" + pattern
			}
			for _, candidate := range p.Tasks {
				matches := candidate.Name == pattern
				if strings.ContainsAny(pattern, "*?[") {
					matches, _ = filepath.Match(pattern, candidate.Name)
				}
				dep := identities[candidate.Name]
				if matches && !seen[dep] {
					node.Dependencies = append(node.Dependencies, dep)
					seen[dep] = true
				}
			}
		}
		graph.Tasks = append(graph.Tasks, node)
	}
	return graph
}
