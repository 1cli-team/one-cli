//go:build !unix

package taskrun

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
)

const supportsPTY = false

type childProcess struct{}

func startPTY(Task, int, int) (*childProcess, error) {
	return nil, i18n.Errorf("task.terminal_unavailable")
}
func (*childProcess) Write([]byte) (int, error)            { return 0, io.ErrClosedPipe }
func (*childProcess) Resize(int, int) error                { return nil }
func (*childProcess) Close() error                         { return nil }
func (*childProcess) run(context.Context, io.Writer) error { return io.ErrClosedPipe }
func (s *Session) runRaw() ([]Result, error) {
	defer close(s.done)
	j := s.jobs[0]
	start := time.Now()
	cmd := platformprocess.CommandContext(s.ctx, j.task.Argv[0], j.task.Argv[1:]...)
	cmd.Dir = j.task.Directory
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	platformprocess.CancelProcessTree(cmd)
	err := cmd.Start()
	if err == nil {
		cleanup, guardErr := guardProcess(cmd)
		if guardErr != nil {
			_ = cmd.Cancel()
			_ = cmd.Wait()
			err = guardErr
		} else {
			err = cmd.Wait()
			cleanup()
		}
	}
	if s.ctx.Err() != nil {
		err = context.Cause(s.ctx)
	}
	r := Result{Name: j.task.Name, Status: "succeeded", Duration: time.Since(start), ExitCode: exitCode(err), Err: err}
	if err != nil {
		r.Status = "failed"
		return []Result{r}, statusError(err)
	}
	return []Result{r}, nil
}
