package tasks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/dependencies"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/taskui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type Service struct {
	Provider runtimeport.Provider
	Loaders  *secrets.Registry
	Prepare  func(context.Context, dependencies.Input) error
	// WorkerCommand substitutes the private leaf transport in process integration tests.
	WorkerCommand func(string) []string
}
type Result struct {
	Schema   string   `json:"schema"`
	Status   string   `json:"status"`
	ExitCode int      `json:"exit_code"`
	Entries  []string `json:"entries"`
	Tasks    []Task   `json:"tasks"`
}

func (r *Result) RenderTTY(w io.Writer) { fmt.Fprintln(w, i18n.T("tasks.result."+r.Status)) }
func ValidateOptions(opts Options) error {
	for _, arg := range opts.Arguments {
		if arg == ":::" {
			return i18n.Errorf("tasks.reserved_argument")
		}
	}
	if opts.Jobs < 1 {
		return i18n.Errorf("build.concurrency_invalid")
	}
	switch opts.Cache {
	case "local-only", "read-write", "read-only", "write-only", "off":
	default:
		return i18n.Errorf("tasks.cache_invalid", opts.Cache)
	}
	switch opts.UI {
	case "auto", "stream", "raw", "tui":
	default:
		return i18n.Errorf("tasks.ui_invalid", opts.UI)
	}
	return nil
}
func (s Service) Execute(ctx context.Context, w execution.Workspace, p *Plan, opts Options, in io.Reader, out, errOut io.Writer) (*Result, error) {
	if err := ValidateOptions(opts); err != nil {
		return nil, err
	}
	if err := executionOptions(p, &opts); err != nil {
		return nil, err
	}
	if s.Provider == nil {
		return nil, i18n.Errorf("exec.mise_missing")
	}
	ctx, stop := process.SignalContext(ctx)
	defer stop()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	opts.UI = taskui.Mode(opts.UI, len(p.Tasks), in, errOut)
	prepare := s.Prepare
	if prepare == nil {
		prepare = (dependencies.Service{Provider: s.Provider}).Prepare
	}
	fmt.Fprintln(errOut, i18n.T("tasks.preparing_dependencies"))
	if err := prepare(ctx, dependencies.Input{Root: w.Root(), Manifest: w.Manifest(), Projects: projectNames(p), Runtime: runtimeport.Mise, Development: true, Log: errOut}); err != nil {
		return nil, err
	}
	base, err := s.Provider.PrepareCLI(ctx, runtimeport.Command{Directory: w.Root(), Env: os.Environ()})
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(errOut, i18n.Tf("tasks.preparing_environment", p.Environment))
	session, err := s.prepareEnvironment(ctx, w, p, base.Env)
	if err != nil {
		return nil, err
	}
	specs := map[string]leafSpec{}
	toolEnvironments := map[string][]string{}
	for i, task := range p.Tasks {
		commands, err := runCommands(task)
		if err != nil {
			return nil, err
		}
		env := base.Env
		if len(commands) > 0 || task.File != "" {
			key := task.Directory + "\x00" + strings.Join(task.tools, "\x00")
			env = toolEnvironments[key]
			if env == nil {
				env, err = s.toolEnvironment(ctx, task, base.Env, errOut)
				if err != nil {
					return nil, err
				}
				toolEnvironments[key] = env
			}
			env = secrets.MergeIntoEnviron(env, task.env, true)
			env = removeEnv(env, task.unsetEnv)
			env = secrets.MergeIntoEnviron(env, session.snapshots[task.Project], true)
		}
		env = removeEnv(env, []string{"ONE_PROCESS_ENDPOINT", "ONE_PROCESS_TOKEN", "MISE_TASK_PGID_MANAGED"})
		env = secrets.MergeIntoEnviron(env, map[string]string{"MISE_PROJECT_ROOT": w.Root(), "MISE_TASK_NAME": strings.TrimPrefix(task.Name, "//:"), "ONE_WORKSPACE_ROOT": w.Root()}, true)
		if opts.UI == "tui" {
			env = taskui.ColorEnvironment(env)
		}
		task.Source = filepath.Join(w.Root(), task.Source)
		arguments := []string(nil)
		if slices.Contains(p.Entries, task.Name) {
			arguments = opts.Arguments
		}
		specs[fmt.Sprintf("task-%d", i)] = leafSpec{Task: task, Commands: commands, Environment: env, Arguments: arguments, Shell: task.shell, Force: opts.Force || opts.UI == "raw" || task.Raw || task.Interactive || len(arguments) > 0 || session.uncached[task.Name]}
	}
	if len(session.snapshots) > 0 {
		fmt.Fprintln(errOut, i18n.T("tasks.environment_cache_off"))
	}
	binary, err := s.prepareCompose(ctx, w.Root(), base.Env, errOut)
	if err != nil {
		return nil, err
	}
	broker, err := newInvocationBroker(specs)
	if err != nil {
		return nil, err
	}
	defer broker.close()
	directory, err := os.MkdirTemp("", "one-compose-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)
	worker := s.WorkerCommand
	if worker == nil {
		worker = defaultWorker
	}
	config, err := composeConfig(p, worker)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(directory, "process-compose.yaml")
	if err := os.WriteFile(path, config, 0600); err != nil {
		return nil, err
	}

	argv := []string{"up", "--config", path, "--no-server", "--disable-dotenv", "--ordered-shutdown", "--read-only", "--log-file", filepath.Join(directory, "runtime.log"), "--tui=false"}
	argv = append(argv, p.Entries...)
	if opts.UI == "raw" {
		entry := ""
		for _, task := range p.Tasks {
			if hasRun(task.Run) || task.File != "" {
				entry = task.Name
			}
		}
		argv = []string{"run", entry, "--config", path, "--no-server", "--disable-dotenv", "--read-only", "--log-file", filepath.Join(directory, "runtime.log")}
	}
	child := process.CommandContext(ctx, binary, argv...)
	child.Dir, child.Env = w.Root(), broker.environment(secrets.MergeIntoEnviron(composeEnvironment(base.Env), map[string]string{"PROC_COMP_CONFIG": directory}, true))
	child.Stdin, child.Stdout, child.Stderr = in, out, errOut
	if output.IsStructured() || opts.UI == "tui" {
		child.Stdout = errOut
	}
	child.Cancel = func() error {
		if runtime.GOOS == "windows" {
			return process.StopTree(child.Process)
		}
		return child.Process.Signal(os.Interrupt)
	}
	child.WaitDelay = 8 * time.Second
	if opts.UI == "tui" {
		err = taskui.Run(ctx, func() { cancel(&process.ExitStatus{Code: 130}) }, child, taskGraph(p), in, errOut, broker.snapshot)
	} else {
		err = child.Start()
		if err == nil {
			err = child.Wait()
		}
	}

	result := &Result{Schema: "one-cli/task-result/v1", Status: "succeeded", Entries: p.Entries}
	if err != nil {
		result.Status = "failed"
		result.ExitCode = process.ExitCode(err)
	}
	if ctx.Err() != nil {
		result.Status = "cancelled"
		result.ExitCode = 130
		var exit *process.ExitStatus
		if errors.As(context.Cause(ctx), &exit) {
			result.ExitCode = exit.Code
		}
	}
	// Preserve the first business failure when shutdown cancels sibling tasks.
	if ctx.Err() == nil {
		broker.mu.Lock()
		failed, cancelled := broker.firstFailure, 0
		for _, event := range broker.events {
			if event.Status == "cancelled" && cancelled == 0 {
				cancelled = event.ExitCode
			}
		}
		broker.mu.Unlock()
		if failed != 0 {
			result.Status = "failed"
			result.ExitCode = failed
		} else if cancelled != 0 {
			result.Status = "cancelled"
			result.ExitCode = cancelled
		}
	}
	result.Tasks = broker.tasks(p, result.Status)
	if result.ExitCode != 0 {
		return result, &process.ExitStatus{Code: result.ExitCode}
	}
	return result, nil
}

// Runtime flags belong to One. Ambient PC configuration must not redirect this
// invocation to another project, enable its HTTP API or load a separate dotenv.
func composeEnvironment(env []string) []string {
	return slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.HasPrefix(strings.ToUpper(key), "PC_")
	})
}
