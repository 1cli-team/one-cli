package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/dependencies"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/taskrun"
)

type Runner func(context.Context, string, Task, string, io.Writer) error

type Service struct {
	Prepare     func(context.Context, dependencies.Input) error
	UI          taskrun.Mode
	Concurrency int
	Run         Runner
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
	fmt.Fprintf(w, i18n.T("build.summary")+"\n", counts["succeeded"], counts["failed"], counts["skipped"], counts["not_run"]+counts["blocked"]+counts["stopped"])
	if r.Error != "" {
		fmt.Fprintln(w, r.Error)
	}
}

// Execute prepares all selected projects before running any build, then runs
// each finite task to completion. A failure leaves remaining tasks not_run.
func (s Service) Execute(ctx context.Context, w execution.Workspace, plan *Plan, log io.Writer) (*Result, error) {
	ctx, stop := taskrun.SignalContext(ctx)
	defer stop()
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
	binary, err := os.Executable()
	if err != nil {
		return fail(err)
	}
	var tasks []taskrun.Task
	byName := map[string]int{}
	for i, task := range result.Tasks {
		if task.Status == "skipped" {
			continue
		}
		argv := []string{binary, "run", "--project", task.Project, "-o", "json"}
		if plan.Environment != "" {
			argv = append(argv, "--env", plan.Environment)
		}
		argv = append(append(argv, "--"), task.Argv...)
		tasks = append(tasks, taskrun.Task{Name: task.Project, Directory: w.Root(), Argv: argv, Dependencies: task.Dependencies})
		byName[task.Project] = i
	}
	opts := taskrun.Options{Mode: s.UI, Title: "build", Concurrency: s.Concurrency, Output: log}
	if s.Run != nil {
		opts.Run = func(ctx context.Context, t taskrun.Task, out io.Writer) error {
			return s.Run(ctx, w.Root(), result.Tasks[byName[t.Name]], plan.Environment, out)
		}
	}
	outcomes, err := taskrun.Run(ctx, tasks, opts)
	for _, outcome := range outcomes {
		task := &result.Tasks[byName[outcome.Name]]
		task.Status = outcome.Status
		task.ExitCode = outcome.ExitCode
		task.DurationMS = outcome.Duration.Milliseconds()
		if outcome.Status == "failed" && result.Error == "" {
			result.Error = fmt.Sprintf("%s: %v", outcome.Name, outcome.Err)
		}
	}
	if err != nil {
		message := result.Error
		res, e := fail(err)
		if message != "" {
			res.Error = message
		}
		return res, e
	}
	return result, nil
}
