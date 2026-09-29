package tasks

import "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"

// Any task may be long-running. Automatic parallelism allocates a slot per
// graph node; an explicit concurrency limit remains the caller's choice.
func executionOptions(p *Plan, opts *Options) error {
	if !opts.JobsExplicit {
		opts.Jobs = max(1, len(p.Tasks))
	}
	commands := 0
	for _, task := range p.Tasks {
		if hasRun(task.Run) || task.File != "" {
			commands++
		}
	}
	for _, task := range p.Tasks {
		if task.Interactive || task.Raw {
			if commands > 1 {
				return i18n.Errorf("tasks.interactive_multiple", task.Name)
			}
			opts.UI = "raw"
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
