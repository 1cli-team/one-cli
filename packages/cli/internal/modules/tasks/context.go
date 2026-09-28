package tasks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

const contextVariable = "ONE_TASK_CONTEXT"

type projectContext struct {
	Variables  map[string]string   `json:"variables"`
	Operations map[string][]string `json:"operations"`
}
type runContext struct {
	Protocol    int                       `json:"protocol"`
	Root        string                    `json:"root"`
	Environment string                    `json:"environment"`
	Executable  string                    `json:"executable"`
	Projects    map[string]projectContext `json:"projects"`
}

func prepareContext(ctx context.Context, w execution.Workspace, p *Plan, loaders *secrets.Registry) ([]string, func(), error) {
	environment, _, err := secrets.ResolveEnvName(w.Root(), p.Environment, false)
	if err != nil {
		return nil, nil, err
	}
	state := runContext{Protocol: 1, Root: w.Root(), Environment: environment, Projects: map[string]projectContext{}}
	binary, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(binary)
	if err != nil {
		return nil, nil, err
	}
	h := sha256.New()
	_, err = io.Copy(h, f)
	f.Close()
	if err != nil {
		return nil, nil, err
	}
	state.Executable = hex.EncodeToString(h.Sum(nil))
	provider := workspace.EnvBackend(w.Manifest())
	if provider == "" {
		provider = "dotenv"
	}
	for _, task := range p.Tasks {
		if task.Project == "" || !task.Managed {
			continue
		}
		project, ok := state.Projects[task.Project]
		if !ok {
			if loaders == nil || loaders.Find(provider) == nil {
				return nil, nil, i18n.Errorf("exec.provider_unregistered", provider)
			}
			entry, _ := w.Project(task.Project)
			variables, err := loaders.Find(provider).Load(ctx, w.Root(), entry.RelativeDir, environment)
			if err != nil {
				return nil, nil, err
			}
			project = projectContext{Variables: variables, Operations: map[string][]string{}}
		}
		if task.Managed {
			argv, err := execution.OperationArgs(w, task.Project, task.Operation)
			if err != nil {
				return nil, nil, err
			}
			project.Operations[task.Operation] = argv
		}
		state.Projects[task.Project] = project
	}
	dir, err := os.MkdirTemp("", "one-task-context-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	raw, err := json.Marshal(state)
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "context.json"), raw, 0600)
	}
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	launcher := filepath.Join(dir, "one")
	if runtime.GOOS == "windows" {
		launcher += ".exe"
	}
	if err = os.Symlink(binary, launcher); err != nil {
		source, e := os.Open(binary)
		if e != nil {
			cleanup()
			return nil, nil, e
		}
		dest, e := os.OpenFile(launcher, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
		if e != nil {
			source.Close()
			cleanup()
			return nil, nil, e
		}
		_, e = io.Copy(dest, source)
		source.Close()
		closeErr := dest.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			cleanup()
			return nil, nil, e
		}
	}
	env := secrets.MergeIntoEnviron(os.Environ(), map[string]string{contextVariable: filepath.Join(dir, "context.json"), "PATH": dir + string(os.PathListSeparator) + os.Getenv("PATH")}, true)
	return env, cleanup, nil
}
func loadContext(w execution.Workspace, project, operation string) (runContext, projectContext, error) {
	var state runContext
	path := os.Getenv(contextVariable)
	if path == "" {
		return state, projectContext{}, i18n.Errorf("tasks.context_required", operation, project)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return state, projectContext{}, err
	}
	if err = json.Unmarshal(raw, &state); err != nil {
		return state, projectContext{}, err
	}
	if state.Protocol != 1 || filepath.Clean(state.Root) != filepath.Clean(w.Root()) {
		return state, projectContext{}, i18n.Errorf("tasks.context_invalid")
	}
	values, ok := state.Projects[project]
	if !ok {
		return state, values, i18n.Errorf("tasks.context_required", operation, project)
	}
	argv, ok := values.Operations[operation]
	if !ok {
		return state, values, i18n.Errorf("tasks.context_required", operation, project)
	}
	current, err := execution.OperationArgs(w, project, operation)
	if err != nil {
		return state, values, err
	}
	if !reflect.DeepEqual(argv, current) {
		return state, values, i18n.Errorf("tasks.context_changed", project)
	}
	return state, values, nil
}

// InputFingerprint runs before mise's lookup, including cache hits. It returns no values.
func InputFingerprint(ctx context.Context, w execution.Workspace, project, operation string) (string, error) {
	state, values, err := loadContext(w, project, operation)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	// json sorts map keys; unset and empty variables remain distinct. All injected
	// project values are potential build inputs. Account credentials aren't injected.
	raw, _ := json.Marshal(struct {
		Protocol                         int
		Environment, Executable, Project string
		Variables                        map[string]string
		Argv                             []string
	}{state.Protocol, state.Environment, state.Executable, project, values.Variables, values.Operations[operation]})
	h.Write(raw)
	p, _ := w.Project(project)
	if p.Toolchain == "go" {
		env := secrets.MergeIntoEnviron(os.Environ(), values.Variables, true)
		binary, err := process.LookPathIn("go", p.TargetDir, env)
		if err != nil {
			return "", err
		}
		cmd := exec.CommandContext(ctx, binary, "env", "-json", "GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED", "CC", "CXX")
		cmd.Dir = p.TargetDir
		cmd.Env = env
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		h.Write(out)
		var tools map[string]string
		if err = json.Unmarshal(out, &tools); err != nil {
			return "", err
		}
		if tools["CGO_ENABLED"] == "1" {
			for _, key := range []string{"CC", "CXX"} {
				args := strings.Fields(tools[key])
				if len(args) == 0 {
					continue
				}
				binary, err = process.LookPathIn(args[0], p.TargetDir, env)
				if err != nil {
					return "", err
				}
				cmd = exec.CommandContext(ctx, binary, append(args[1:], "--version")...)
				cmd.Dir = p.TargetDir
				cmd.Env = env
				out, err = cmd.Output()
				if err != nil {
					return "", i18n.Errorf("tasks.compiler_input", tools[key], err)
				}
				h.Write(out)
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func ExecuteLeaf(ctx context.Context, w execution.Workspace, project, operation string, args []string, in io.Reader, out, errOut io.Writer) error {
	_, values, err := loadContext(w, project, operation)
	if err != nil {
		return err
	}
	argv := append([]string{}, values.Operations[operation]...)
	p, _ := w.Project(project)
	if len(args) > 0 {
		if operation == "dev" && workspace.ProjectDev(w.Manifest(), p.Name) != "" {
			if runtime.GOOS != "windows" {
				argv[2] += ` "$@"`
				argv = append(argv, "one-dev")
			}
		} else if p.Toolchain == "go" {
			argv = append(argv, "--")
		}
		argv = append(argv, args...)
	}
	env := secrets.MergeIntoEnviron(os.Environ(), values.Variables, true)
	path := os.Getenv("PATH")
	for _, entry := range env {
		if key, v, ok := strings.Cut(entry, "="); ok && strings.EqualFold(key, "PATH") {
			path = v
		}
	}
	bins := []string{filepath.Join(p.TargetDir, "node_modules/.bin"), filepath.Join(w.Root(), "node_modules/.bin"), path}
	env = secrets.MergeIntoEnviron(env, map[string]string{"PATH": strings.Join(bins, string(os.PathListSeparator))}, true)
	binary, err := process.LookPathIn(argv[0], p.TargetDir, env)
	if err != nil {
		return err
	}
	child := process.Command(binary, argv[1:]...)
	child.Dir = p.TargetDir
	child.Env = env
	child.Stdin = in
	child.Stdout = out
	child.Stderr = errOut
	return process.RunForwarded(ctx, child)
}
func projectNames(p *Plan) []string {
	set := map[string]bool{}
	for _, task := range p.Tasks {
		if task.Project != "" {
			set[task.Project] = true
		}
	}
	names := []string{}
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
