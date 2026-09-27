package taskrun

import (
	"context"
	"io"
	"os"
	"runtime"
	"time"
)

// Retain the existing process-tree regressions against the shared runner.
type testEntry struct{ Name, Cmd string }
type testOpts struct {
	Out         io.Writer
	GracePeriod time.Duration
}

func runTestProcesses(ctx context.Context, root string, entries []testEntry, opts testOpts) error {
	tasks := make([]Task, 0, len(entries))
	for _, e := range entries {
		argv := []string{"sh", "-c", e.Cmd}
		if runtime.GOOS == "windows" {
			shell := os.Getenv("ComSpec")
			if shell == "" {
				shell = "cmd.exe"
			}
			argv = []string{shell, "/d", "/s", "/c", e.Cmd}
		}
		tasks = append(tasks, Task{Name: e.Name, Directory: root, Argv: argv})
	}
	_, err := Run(ctx, tasks, Options{Mode: Stream, Development: true, Output: opts.Out})
	return err
}
