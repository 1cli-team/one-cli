package execution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"gopkg.in/yaml.v3"
)

// OperationArgs resolves the live source command, never a copy in generated TOML.
func OperationArgs(w Workspace, selector, operation string) ([]string, error) {
	p, ok := w.Project(selector)
	if !ok {
		return nil, cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND, "Unknown project: "+selector)
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
	if operation != "build" && operation != "test" && operation != "lint" {
		return nil, missingOperation(p.Name, operation)
	}
	if p.Toolchain == "node" {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		raw, err := os.ReadFile(filepath.Join(p.TargetDir, "package.json"))
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &pkg); err != nil {
			return nil, err
		}
		manager, err := workspace.ResolvePackageManager(root, p.PackageManager)
		if err != nil {
			return nil, err
		}
		if pkg.Scripts[operation] == "" {
			return nil, missingOperation(p.Name, operation)
		}
		return []string{manager, "run", operation}, nil
	}
	if p.Toolchain == "go" {
		raw, err := os.ReadFile(filepath.Join(p.TargetDir, "Taskfile.yml"))
		if err == nil {
			var tasks struct {
				Tasks map[string]any `yaml:"tasks"`
			}
			if err = yaml.Unmarshal(raw, &tasks); err != nil {
				return nil, err
			}
			if _, exists := tasks.Tasks[operation]; exists {
				return []string{"task", operation}, nil
			}
			return nil, missingOperation(p.Name, operation)
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		switch operation {
		case "build":
			return nil, fmt.Errorf("project %s requires Taskfile.yml with a build task", p.Name)
		case "test":
			return []string{"go", "test", "./..."}, nil
		case "lint":
			return []string{"go", "vet", "./..."}, nil
		}
	}
	return nil, missingOperation(p.Name, operation)
}

func missingOperation(project, operation string) error {
	return cliErrors.New(cliErrors.RUNTIME_TASK_NOT_FOUND, fmt.Sprintf("Project %s has no %s task.", project, operation))
}
