package execution

import (
	"os"
	"runtime"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// OperationArgs resolves the live source command, never a copy in generated TOML.
func OperationArgs(w Workspace, selector, operation string) ([]string, error) {
	p, ok := w.Project(selector)
	if !ok {
		return nil, cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND, i18n.Tf("workspace.unknown_project", selector))
	}
	if operation == "dev" {
		command := workspace.ProjectDev(w.Manifest(), p.Name)
		if command == "" {
			return nil, missingOperation(p.Name, operation)
		}
		if runtime.GOOS == "windows" {
			shell := os.Getenv("ComSpec")
			if shell == "" {
				shell = "cmd.exe"
			}
			return []string{shell, "/d", "/s", "/c", command}, nil
		}
		return []string{"sh", "-c", command}, nil
	}
	return ProjectOperationArgs(w.Root(), *p, operation)
}

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
