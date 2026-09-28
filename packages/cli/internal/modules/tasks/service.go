package tasks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/dependencies"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/taskrun"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type Service struct {
	Provider runtimeport.Provider
	Loaders  *secrets.Registry
	Prepare  func(context.Context, dependencies.Input) error
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
	if opts.Jobs < 1 {
		return i18n.Errorf("build.concurrency_invalid")
	}
	switch opts.Cache {
	case "local-only", "read-write", "read-only", "write-only", "off":
	default:
		return i18n.Errorf("tasks.cache_invalid", opts.Cache)
	}
	switch opts.UI {
	case "auto", "stream", "raw":
	case "tui":
		return i18n.Errorf("tasks.tui_unavailable")
	default:
		return i18n.Errorf("tasks.ui_invalid", opts.UI)
	}
	return nil
}
func (s Service) Execute(ctx context.Context, w execution.Workspace, p *Plan, opts Options, in io.Reader, out, errOut io.Writer) (*Result, error) {
	if err := ValidateOptions(opts); err != nil {
		return nil, err
	}
	ctx, stop := taskrun.SignalContext(ctx)
	defer stop()
	if err := p.configuration.Apply(ctx); err != nil {
		return nil, err
	}
	if s.Provider == nil {
		return nil, i18n.Errorf("exec.mise_missing")
	}
	if err := s.inspectCache(ctx, w, p, opts); err != nil {
		return nil, err
	}
	for _, task := range p.Tasks {
		if task.Interactive || task.Raw {
			opts.Cache = "off"
			if opts.UI == "auto" {
				opts.UI = "raw"
			}
		}
	}
	prepare := s.Prepare
	if prepare == nil {
		prepare = (dependencies.Service{Provider: s.Provider}).Prepare
	}
	if err := prepare(ctx, dependencies.Input{Root: w.Root(), Manifest: w.Manifest(), Projects: projectNames(p), Runtime: runtimeport.Mise, Log: errOut}); err != nil {
		return nil, err
	}
	env, cleanup, err := prepareContext(ctx, w, p, s.Loaders)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	mode := "interleave"
	if len(p.Entries) > 1 {
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
		argv = append(argv, "--raw", "--task-cache", "off")
		fmt.Fprintln(errOut, i18n.T("tasks.raw_cache_off"))
	}
	for i, entry := range p.Entries {
		if i > 0 {
			argv = append(argv, ":::")
		}
		argv = append(argv, entry)
		if len(opts.Arguments) > 0 {
			argv = append(argv, "--")
			argv = append(argv, opts.Arguments...)
		}
	}
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
	err = child.Run()
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
