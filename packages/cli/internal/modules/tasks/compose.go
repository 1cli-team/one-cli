package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
	"gopkg.in/yaml.v3"
)

const composeVersion = "1.122.0"

type composeProject struct {
	Version          string                    `yaml:"version"`
	Name             string                    `yaml:"name"`
	Strict           bool                      `yaml:"is_strict"`
	DisableExpansion bool                      `yaml:"disable_env_expansion"`
	OrderedShutdown  bool                      `yaml:"ordered_shutdown"`
	LogLength        int                       `yaml:"log_length"`
	Processes        map[string]composeProcess `yaml:"processes"`
}
type composeProcess struct {
	Entrypoint   []string                     `yaml:"entrypoint"`
	Directory    string                       `yaml:"working_dir"`
	Description  string                       `yaml:"description,omitempty"`
	Dependencies map[string]composeDependency `yaml:"depends_on,omitempty"`
	Availability map[string]string            `yaml:"availability"`
	Shutdown     composeShutdown              `yaml:"shutdown"`
}
type composeShutdown struct {
	Signal     int  `yaml:"signal"`
	Timeout    int  `yaml:"timeout_seconds"`
	ParentOnly bool `yaml:"parent_only"`
}
type composeDependency struct {
	Condition string `yaml:"condition"`
}

// Only worker identities and graph metadata enter the config. Commands and
// environment snapshots are delivered in memory by the invocation broker.
func composeConfig(p *Plan, worker func(string) []string) ([]byte, error) {
	project := composeProject{Version: "0.5", Name: "One", Strict: true, DisableExpansion: true, OrderedShutdown: true, LogLength: 10000, Processes: map[string]composeProcess{}}
	for i, task := range p.Tasks {
		dependencies := map[string]composeDependency{}
		for _, name := range append(append([]string(nil), task.Dependencies...), task.waitFor...) {
			dependencies[name] = composeDependency{"process_completed_successfully"}
		}
		project.Processes[task.Name] = composeProcess{Entrypoint: worker(fmt.Sprintf("task-%d", i)), Directory: task.Directory, Description: task.Description, Dependencies: dependencies, Availability: map[string]string{"restart": "exit_on_failure"}, Shutdown: composeShutdown{Signal: 2, Timeout: 3, ParentOnly: true}}
	}
	return yaml.Marshal(project)
}

func (s Service) prepareCompose(ctx context.Context, root string, env []string, log io.Writer) (string, error) {
	// Reuse an installed exact version. This also keeps offline fixture runs and
	// workspaces using a shared mise installation from downloading another copy.
	if path, err := process.LookPathIn("process-compose", root, env); err == nil && checkCompose(ctx, path, env) == nil {
		return path, nil
	}
	fmt.Fprintln(log, i18n.Tf("tasks.preparing_compose", composeVersion))
	command, err := s.Provider.PrepareCLI(ctx, runtimeport.Command{Directory: root, Env: env, Argv: []string{"install", "process-compose@" + composeVersion}})
	if err != nil {
		return "", err
	}
	child := process.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
	child.Dir, child.Env, child.Stdout, child.Stderr = command.Directory, command.Env, log, log
	process.CancelProcessTree(child)
	if err := child.Run(); err != nil {
		return "", i18n.Errorf("tasks.compose_install", err)
	}
	command, err = s.Provider.PrepareCLI(ctx, runtimeport.Command{Directory: root, Env: env, Argv: []string{"which", "process-compose", "--tool", "process-compose@" + composeVersion}})
	if err != nil {
		return "", err
	}
	probe := process.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
	probe.Dir, probe.Env, probe.Stderr = command.Directory, command.Env, log
	process.CancelProcessTree(probe)
	raw, err := probe.Output()
	if err != nil {
		return "", i18n.Errorf("tasks.compose_install", err)
	}
	path := strings.TrimSpace(string(raw))
	if err := checkCompose(ctx, path, env); err != nil {
		return "", err
	}
	return path, nil
}
func checkCompose(ctx context.Context, path string, env []string) error {
	child := process.CommandContext(ctx, path, "version", "--short")
	child.Env = env
	process.CancelProcessTree(child)
	raw, err := child.Output()
	if err != nil {
		return i18n.Errorf("tasks.compose_install", err)
	}
	if strings.TrimSpace(string(raw)) != "v"+composeVersion {
		return i18n.Errorf("tasks.compose_version", composeVersion)
	}
	return nil
}

func (s Service) toolEnvironment(ctx context.Context, task Task, base []string, log io.Writer) ([]string, error) {
	args := append([]string{"env", "--json"}, task.tools...)
	command, err := s.Provider.PrepareCLI(ctx, runtimeport.Command{Directory: task.Directory, Env: base, Argv: args})
	if err != nil {
		return nil, err
	}
	child := process.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
	child.Dir, child.Env, child.Stderr = command.Directory, command.Env, log
	process.CancelProcessTree(child)
	raw, err := child.Output()
	if err != nil {
		return nil, i18n.Errorf("tasks.tool_environment", task.Name, err)
	}
	values := map[string]*string{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, i18n.Errorf("tasks.tool_environment", task.Name, err)
	}
	merged := map[string]string{}
	var unset []string
	for key, value := range values {
		if value == nil {
			unset = append(unset, key)
		} else {
			merged[key] = *value
		}
	}
	return removeEnv(secrets.MergeIntoEnviron(base, merged, true), unset), nil
}
func removeEnv(env []string, keys []string) []string {
	values := map[string]string{}
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	for _, key := range keys {
		for candidate := range values {
			if envKeyEqual(candidate, key) {
				delete(values, candidate)
			}
		}
	}
	return secrets.MergeIntoEnviron(nil, values, true)
}

func defaultWorker(id string) []string {
	executable, _ := os.Executable()
	return []string{filepath.Clean(executable), "__process", id, "-o", "text"}
}
