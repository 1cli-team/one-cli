package execution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
		manager := p.PackageManager
		if rootPkg, err := workspace.ReadPackageJSON(w.Root()); err == nil && rootPkg != nil && rootPkg.PackageManager != "" {
			manager = rootPkg.PackageManager
		}
		manager, _, _ = strings.Cut(manager, "@")
		if manager == "" {
			manager = "pnpm"
		}
		switch manager {
		case "pnpm", "npm", "yarn", "bun":
		default:
			return nil, fmt.Errorf("unsupported package manager %q", manager)
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
			return []string{"go", "build", "./..."}, nil
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
