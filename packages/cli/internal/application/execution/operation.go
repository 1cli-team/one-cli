package execution

import (
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// ProjectOperationArgs resolves a task from the project's current source files.
// It is shared by execution and the Dashboard and never installs or runs tools.
func ProjectOperationArgs(root string, p workspace.Project, operation string) ([]string, error) {
	if p.Toolchain == "node" {
		if _, err := workspace.ResolvePackageManager(root, p.PackageManager); err != nil {
			return nil, err
		}
	}
	tasks, err := workspace.DiscoverTasks(root, p, nil)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		if task.Name == operation {
			return task.Argv, nil
		}
	}
	return nil, missingOperation(p.Name, operation)
}

func missingOperation(project, operation string) error {
	return cliErrors.New(cliErrors.RUNTIME_TASK_NOT_FOUND, i18n.Tf("task.operation_missing", project, operation))
}
