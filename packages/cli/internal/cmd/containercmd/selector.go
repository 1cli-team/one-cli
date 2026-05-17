package containercmd

// selector.go translates user input (positional arg, -p flag) into a
// concrete subproject name, and enumerates which projects in the
// manifest are container-enabled. No I/O beyond the manifest read.

import (
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// containerSubprojects returns the names of subprojects that have
// container enabled (i.e. should have / produce a Dockerfile). Returns
// NOT_ONE_PROJECT if the cwd isn't inside a workspace, or
// BACKEND_NOT_ENABLED if no project (and no workspace-level fallback)
// has container configured.
func containerSubprojects(projectRoot string) ([]string, error) {
	if !workspace.HasManifest(projectRoot) {
		return nil, cliErrors.New(cliErrors.NOT_ONE_PROJECT,
			"未检测到 One CLI 项目，请在工作区根目录执行。")
	}
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return nil, err
	}
	var names []string
	anyEnabled := false
	for _, s := range m.Projects {
		enabled, _ := workspace.ContainerForProject(m, s.Name)
		if enabled {
			anyEnabled = true
			names = append(names, s.Name)
		}
	}
	if !anyEnabled {
		return nil, cliErrors.New(cliErrors.BACKEND_NOT_ENABLED,
			"当前工作区未启用 container 后端。在 manifest 中为某个项目配置 container 后再试。").
			WithContext(map[string]any{"domain": "container"})
	}
	return names, nil
}

// containerProjectSelector reconciles the positional arg and the -p
// flag, returning the chosen subproject selector or an error when the
// two disagree.
func containerProjectSelector(projectFlag string, args []string) (string, error) {
	projectFlag = strings.TrimSpace(projectFlag)
	arg := ""
	if len(args) > 0 {
		arg = strings.TrimSpace(args[0])
	}
	if projectFlag != "" && arg != "" && projectFlag != arg {
		return "", cliErrors.New(cliErrors.ONE_CLI_ERROR,
			"subproject 只能指定一次：使用位置参数或 -p/--project 其一。")
	}
	if projectFlag != "" {
		return projectFlag, nil
	}
	return arg, nil
}

// normalizeContainerProject canonicalises the -p value (manifest name
// or relative path) into a subproject name so downstream filters that
// key on name keep working unchanged. Empty selector → empty result
// (meaning "all").
func normalizeContainerProject(projectRoot, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", nil
	}
	sub, err := workspace.ResolveProjectFromSelector(projectRoot, selector)
	if err != nil {
		return "", err
	}
	if sub == nil {
		// Pass the raw selector through; the caller's enabled-name
		// check surfaces the actionable "no such project" envelope.
		return selector, nil
	}
	return sub.Name, nil
}
