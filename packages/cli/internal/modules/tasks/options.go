package tasks

import (
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"runtime"
	"slices"
	"strings"
)

// Any task may be long-running. Automatic parallelism allocates a slot per
// graph node. Process Compose schedules dependencies without a slot limit.
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
	if opts.JobsExplicit && opts.Jobs < commands {
		return i18n.Errorf("tasks.compose_concurrency")
	}
	if opts.UI == "raw" && commands > 1 {
		return i18n.Errorf("tasks.compose_raw_multiple")
	}
	if runtime.GOOS == "windows" && len(opts.Arguments) > 0 {
		for _, task := range p.Tasks {
			if task.File != "" || !slices.Contains(p.Entries, task.Name) {
				continue
			}
			if len(task.shell) == 0 || !strings.Contains(strings.ToLower(task.shell[0]), "powershell") && !strings.Contains(strings.ToLower(task.shell[0]), "pwsh") {
				return i18n.Errorf("tasks.unsupported", task.Source, task.Name, "arguments with cmd.exe")
			}
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
