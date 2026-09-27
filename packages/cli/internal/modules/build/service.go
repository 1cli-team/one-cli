package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/dependencies"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
)

type Runner func(context.Context, string, Task, string, io.Writer) error

type Service struct {
	Prepare func(context.Context, dependencies.Input) error
	Run     Runner
}

type Result struct {
	*Plan
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code"`
}

func (p *Plan) RenderTTY(w io.Writer) {
	for _, task := range p.Tasks {
		detail := strings.Join(task.Argv, " ")
		if task.Reason != "" {
			detail = task.Reason
		}
		if task.Status == "failed" {
			detail = fmt.Sprintf("%s (exit %d)", detail, task.ExitCode)
		}
		fmt.Fprintf(w, "[%s] %s: %s\n", task.Project, i18n.T("build.status."+task.Status), detail)
	}
}

func (r *Result) RenderTTY(w io.Writer) {
	r.Plan.RenderTTY(w)
	counts := map[string]int{}
	for _, task := range r.Tasks {
		counts[task.Status]++
	}
	fmt.Fprintf(w, i18n.T("build.summary")+"\n", counts["succeeded"], counts["failed"], counts["skipped"], counts["not_run"])
	if r.Error != "" {
		fmt.Fprintln(w, r.Error)
	}
}

// Execute prepares all selected projects before running any build, then runs
// each finite task to completion. A failure leaves remaining tasks not_run.
func (s Service) Execute(ctx context.Context, w execution.Workspace, plan *Plan, log io.Writer) (*Result, error) {
	copyPlan := *plan
	copyPlan.Tasks = append([]Task{}, plan.Tasks...)
	copyPlan.Schema, copyPlan.DryRun = "one-cli/build-result/v1", false
	result := &Result{Plan: &copyPlan}
	selected := []string{}
	for i := range result.Tasks {
		if result.Tasks[i].Status == "pending" {
			selected = append(selected, result.Tasks[i].Project)
			result.Tasks[i].Status = "not_run"
		}
	}
	if log == nil {
		log = io.Discard
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		select {
		case sig := <-signals:
			code := 130
			if sig == syscall.SIGTERM {
				code = 143
			}
			cancel(&platformprocess.ExitStatus{Code: code})
		case <-ctx.Done():
		}
	}()
	fail := func(err error) (*Result, error) {
		if ctx.Err() != nil {
			err = context.Cause(ctx)
		}
		result.ExitCode = 1
		var exit *platformprocess.ExitStatus
		if errors.As(err, &exit) {
			result.ExitCode = exit.Code
		} else if errors.Is(err, context.Canceled) {
			result.ExitCode = 130
		}
		result.Error = err.Error()
		return result, &platformprocess.ExitStatus{Code: result.ExitCode}
	}
	if ctx.Err() != nil {
		return fail(ctx.Err())
	}
	if s.Prepare != nil {
		if err := s.Prepare(ctx, dependencies.Input{Root: w.Root(), Manifest: w.Manifest(), Projects: selected, Runtime: plan.Runtime, Log: log}); err != nil {
			return fail(err)
		}
	}
	run := s.Run
	if run == nil {
		run = runProject
	}
	for i := range result.Tasks {
		task := &result.Tasks[i]
		if task.Status == "skipped" {
			continue
		}
		if ctx.Err() != nil {
			return fail(ctx.Err())
		}
		fmt.Fprintf(log, "[%s] %s\n", task.Project, strings.Join(task.Argv, " "))
		start := time.Now()
		err := run(ctx, w.Root(), *task, plan.Environment, log)
		task.DurationMS = time.Since(start).Milliseconds()
		if err != nil {
			task.Status = "failed"
			res, exit := fail(fmt.Errorf("%s: %w", task.Project, err))
			task.ExitCode = res.ExitCode
			return res, exit
		}
		task.Status = "succeeded"
	}
	return result, nil
}

func runProject(ctx context.Context, root string, task Task, environment string, log io.Writer) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{"run", "--project", task.Project, "-o", "json"}
	if environment != "" {
		args = append(args, "--env", environment)
	}
	args = append(append(args, "--"), task.Argv...)
	child := platformprocess.CommandContext(ctx, binary, args...)
	child.Dir, child.Stdin = root, os.Stdin
	// The runner is a process boundary: one run remains the single owner of
	// mise preparation, project secrets, PATH augmentation, and argv execution.
	child.Env = os.Environ()
	out := &prefixWriter{out: log, prefix: "[" + task.Project + "] "}
	child.Stdout, child.Stderr = out, out
	platformprocess.CancelProcessTree(child)
	err = child.Run()
	out.Flush()
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code := exit.ExitCode()
		if code < 0 {
			code = 1
		}
		return &platformprocess.ExitStatus{Code: code}
	}
	return err
}

// exec serializes writes when stdout and stderr share the same comparable
// writer. Chunking long lines bounds memory without dropping any child output.
type prefixWriter struct {
	out     io.Writer
	prefix  string
	pending string
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	w.pending += string(p)
	for len(w.pending) > 0 {
		end := strings.IndexByte(w.pending, '\n')
		if end < 0 {
			if len(w.pending) < 64*1024 {
				break
			}
			end = 64*1024 - 1
		}
		if _, err := fmt.Fprint(w.out, w.prefix, w.pending[:end+1]); err != nil {
			return 0, err
		}
		w.pending = w.pending[end+1:]
	}
	return len(p), nil
}
func (w *prefixWriter) Flush() {
	if w.pending != "" {
		fmt.Fprintln(w.out, w.prefix+w.pending)
		w.pending = ""
	}
}
