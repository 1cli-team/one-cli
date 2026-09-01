// Package build implements local pre-deploy project builds.
package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
	deployport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

// Local runs build commands on the current machine.
type Local struct{}

func (Local) Build(ctx context.Context, input deployport.BuildInput) ([]string, error) {
	if !shouldAutoBuild(input) {
		return nil, nil
	}
	projectDir := projectDirectory(input.Apply)
	scripts, err := readPackageScripts(projectDir)
	if err != nil {
		return nil, err
	}
	if _, ok := scripts["build"]; !ok {
		return nil, nil
	}
	argv := nodeBuildArgv(input.PackageManager)
	line := strings.Join(argv, " ")
	if input.Apply.DryRun {
		return []string{line}, nil
	}
	return nil, prompt.Spin(fmt.Sprintf("正在构建项目 %s", input.Apply.Project.Name), func() error {
		cmd := platformprocess.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Dir = projectDir
		cmd.Stdout = input.Apply.Stdout
		cmd.Stderr = input.Apply.Stderr
		cmd.Env = augmentBuildEnv(
			os.Environ(), input.Apply.ProjectRoot, projectDir, input.Apply.InjectedEnv,
		)
		return cmd.Run()
	})
}

func shouldAutoBuild(input deployport.BuildInput) bool {
	if input.Toolchain != "node" {
		return false
	}
	switch input.Backend {
	case workspace.DeployBackendCloudflare, workspace.DeployBackendEdgeOne:
		return true
	default:
		return false
	}
}

func projectDirectory(input deployport.ApplyInput) string {
	if input.Project.TargetDir != "" {
		return input.Project.TargetDir
	}
	return filepath.Join(input.ProjectRoot, filepath.FromSlash(input.Project.RelativeDir))
}

func readPackageScripts(projectDir string) (map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(projectDir, "package.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return nil, err
	}
	return pkg.Scripts, nil
}

func nodeBuildArgv(packageManager string) []string {
	switch strings.TrimSpace(packageManager) {
	case "npm":
		return []string{"npm", "run", "build"}
	case "yarn":
		return []string{"yarn", "build"}
	default:
		return []string{"pnpm", "run", "build"}
	}
}

func augmentBuildEnv(parent []string, projectRoot, projectDir string, injected map[string]string) []string {
	base := secrets.MergeIntoEnviron(parent, injected, true)
	binPaths := []string{
		filepath.Join(projectDir, "node_modules", ".bin"),
		filepath.Join(projectRoot, "node_modules", ".bin"),
	}
	separator := string(os.PathListSeparator)
	out := make([]string, 0, len(base)+1)
	replaced := false
	for _, value := range base {
		key, existing, found := strings.Cut(value, "=")
		isPath := key == "PATH" || (runtime.GOOS == "windows" && strings.EqualFold(key, "PATH"))
		if !replaced && found && isPath {
			parts := append([]string{}, binPaths...)
			if existing != "" {
				parts = append(parts, existing)
			}
			out = append(out, "PATH="+strings.Join(parts, separator))
			replaced = true
			continue
		}
		out = append(out, value)
	}
	if !replaced {
		out = append(out, "PATH="+strings.Join(binPaths, separator))
	}
	return out
}
