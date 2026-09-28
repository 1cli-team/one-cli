package tasks

import "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"

// Development shares the normal mise graph. Allocate a slot for every node so
// long-running services cannot starve other services or finite prerequisites.
func executionOptions(p *Plan, opts *Options) error {
	if opts.Name != "dev" {
		return nil
	}
	if !opts.JobsExplicit {
		opts.Jobs = max(1, len(p.Tasks))
	}
	commands := 0
	for _, task := range p.Tasks {
		if developmentCommand(task) {
			commands++
		}
	}
	if commands > 1 {
		for _, task := range p.Tasks {
			if developmentCommand(task) && (task.Interactive || task.Raw) {
				return i18n.Errorf("tasks.dev_exclusive", task.Name)
			}
		}
		if opts.UI == "raw" {
			return i18n.Errorf("tasks.dev_raw_multiple")
		}
		if opts.Jobs < commands {
			return i18n.Errorf("tasks.dev_concurrency", commands)
		}
	}
	return nil
}

func hasRun(run any) bool {
	switch value := run.(type) {
	case nil:
		return false
	case string:
		return value != ""
	case []string:
		return len(value) > 0
	case []any:
		return len(value) > 0
	default:
		return true
	}
}

// Generated non-dev adapters are finite prerequisites. For custom commands,
// assume they may be long-running; their shell bodies cannot be classified.
func developmentCommand(task Task) bool {
	return hasRun(task.Run) && (task.Operation == "dev" || !task.Managed)
}
