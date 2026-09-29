package tasks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
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
	// OnStarted runs after the scheduler starts, before waiting for its exit.
	OnStarted func()
	Provider  runtimeport.Provider
	Loaders   *secrets.Registry
	Prepare   func(context.Context, dependencies.Input) error
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx, stop := process.SignalContext(ctx)
	defer stop()
	if s.Provider == nil {
		return nil, i18n.Errorf("exec.mise_missing")
	}
	if err := executionOptions(p, &opts); err != nil {
		return nil, err
	}
	if err := s.inspectCache(ctx, w, p); err != nil {
		return nil, err
	}
	for _, task := range p.Tasks {
		if task.Interactive || task.Raw {
			opts.Cache = "off"
		}
	}
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
	defer session.close()
	if len(session.bindings) > 0 {
		fmt.Fprintln(errOut, i18n.T("tasks.verifying_environment"))
	}
	if err = s.verifyBindings(ctx, w, p, session); err != nil {
		return nil, err
	}
	for i := range p.Tasks {
		if session.uncached[p.Tasks[i].Name] {
			p.Tasks[i].Cached = false
		}
	}
	env := session.env
	if len(session.bindings) > 0 {
		fmt.Fprintln(errOut, i18n.T("tasks.environment_cache_off"))
	}
	opts.UI = taskui.Mode(opts.UI, len(p.Tasks), in, errOut)
	mode := "interleave"
	if len(p.Tasks) > 1 || opts.UI == "tui" {
		mode = "prefix"
	}
	argv := []string{"run", "--jobs", strconv.Itoa(opts.Jobs), "--task-cache", opts.Cache, "--output", mode}
	if opts.Force {
		argv = append(argv, "--force")
	}
	// Extra flags may redirect output or select watch mode. A generated output
	// contract describes the default invocation only.
	if len(opts.Arguments) > 0 {
		argv = append(argv, "--task-cache", "off", "--force")
	}
	if opts.UI == "raw" {
		argv = append(argv, "--raw", "--task-cache", "off", "--force")
		fmt.Fprintln(errOut, i18n.T("tasks.raw_cache_off"))
	}
	for i, entry := range p.Entries {
		if i > 0 {
			argv = append(argv, ":::")
		}
		for _, task := range p.Tasks {
			if task.Name == entry {
				entry = task.runtimeName()
				break
			}
		}
		argv = append(argv, entry)
		if len(opts.Arguments) > 0 {
			argv = append(argv, "--")
			argv = append(argv, opts.Arguments...)
		}
	}
	// One starts its own scheduler, even when invoked from another mise task.
	// Inheriting the parent's marker disables child process groups in mise,
	// preventing it from stopping sibling services when one task fails.
	env = slices.DeleteFunc(env, func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, "MISE_TASK_PGID_MANAGED")
	})
	command, err := s.Provider.PrepareCLI(ctx, runtimeport.Command{Directory: w.Root(), Argv: argv, Env: env})
	if err != nil {
		return nil, err
	}
	child := process.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
	child.Cancel = func() error { return process.StopTree(child.Process) }
	child.WaitDelay = 3 * time.Second
	child.Dir = command.Directory
	child.Env = command.Env
	child.Stdin = in
	child.Stdout = out
	child.Stderr = errOut
	if output.IsStructured() {
		child.Stdout = errOut
	}
	if opts.UI == "tui" {
		err = taskui.Run(ctx, cancel, child, taskGraph(p), in, errOut, s.OnStarted)
	} else {
		err = child.Start()
		if err == nil {
			if s.OnStarted != nil {
				s.OnStarted()
			}
			err = child.Wait()
		}
	}
	result := &Result{Schema: "one-cli/task-result/v1", Status: "succeeded", Entries: p.Entries, Tasks: p.Tasks}
	if err != nil {
		result.Status = "failed"
		result.ExitCode = process.ExitCode(err)
		var exit *process.ExitStatus
		if errors.As(err, &exit) {
			result.ExitCode = exit.Code
		}
		if ctx.Err() != nil {
			result.Status = "cancelled"
			result.ExitCode = 130
			if errors.As(context.Cause(ctx), &exit) {
				result.ExitCode = exit.Code
			}
		}
		return result, &process.ExitStatus{Code: result.ExitCode}
	}
	return result, nil
}
