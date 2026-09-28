package taskrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	uv "github.com/charmbracelet/ultraviolet"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
)

type Task struct {
	Name, Directory    string
	Argv, Dependencies []string
}
type Result struct {
	Name, Status string
	ExitCode     int
	Duration     time.Duration
	Err          error
}
type Options struct {
	Mode                   Mode
	Title                  string
	Development, KeepGoing bool
	Concurrency            int
	Output                 io.Writer
	// Run is a testable non-terminal execution boundary. Production uses Argv.
	Run func(context.Context, Task, io.Writer) error
}
type job struct {
	task     Task
	result   Result
	terminal *terminal
	cancel   context.CancelFunc
	started  time.Time
	action   string
	attempt  int
	offset   int
	ioMu     sync.Mutex
	child    *childProcess
}
type Session struct {
	mu            sync.Mutex
	outputMu      sync.Mutex
	jobs          []*job
	opts          Options
	ctx           context.Context
	cancel        context.CancelCauseFunc
	done          chan struct{}
	controls      chan control
	width, height int
	results       []Result
	err           error
}
type control struct {
	index  int
	action string
}
type completion struct {
	index int
	err   error
}

func Run(ctx context.Context, tasks []Task, opts Options) ([]Result, error) {
	if err := validateTasks(tasks); err != nil {
		return nil, err
	}
	if opts.Concurrency < 0 {
		return nil, i18n.Errorf("task.concurrency_positive")
	}
	if opts.Development {
		opts.Concurrency = len(tasks)
	} else if opts.Concurrency == 0 {
		opts.Concurrency = 1
	}
	if opts.Output == nil {
		opts.Output = os.Stderr
	}
	if opts.Mode == "" {
		opts.Mode = Stream
	}
	if opts.Mode == Raw && len(tasks) != 1 {
		return nil, i18n.Errorf("task.raw_single_output")
	}
	ctx, stopSignals := SignalContext(ctx)
	defer stopSignals()
	ctx, cancel := context.WithCancelCause(ctx)
	s := &Session{opts: opts, ctx: ctx, cancel: cancel, done: make(chan struct{}), controls: make(chan control, 32), width: 80, height: 24}
	defer cancel(nil)
	for _, t := range tasks {
		s.jobs = append(s.jobs, &job{task: t, result: Result{Name: t.Name, Status: "pending"}})
	}
	if opts.Mode == TUI {
		for _, j := range s.jobs {
			s.initTerminal(j)
		}
	}
	if opts.Mode == Raw {
		return s.runRaw()
	}
	go s.schedule()
	if opts.Mode == TUI {
		if err := s.show(); err != nil {
			cancel(err)
		}
	}
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	if opts.Mode == TUI {
		s.printFailures()
		for _, j := range s.jobs {
			_ = j.terminal.Close()
		}
	}
	return s.results, s.err
}

func validateTasks(tasks []Task) error {
	names := map[string]int{}
	for i, t := range tasks {
		if t.Name == "" {
			return i18n.Errorf("task.name_empty")
		}
		if _, ok := names[t.Name]; ok {
			return i18n.Errorf("task.name_duplicate", t.Name)
		}
		names[t.Name] = i
	}
	state := make([]int, len(tasks))
	var visit func(int) error
	visit = func(i int) error {
		if state[i] == 1 {
			return i18n.Errorf("task.dependency_cycle", tasks[i].Name)
		}
		if state[i] == 2 {
			return nil
		}
		state[i] = 1
		for _, d := range tasks[i].Dependencies {
			n, ok := names[d]
			if !ok {
				continue
			}
			if err := visit(n); err != nil {
				return err
			}
		}
		state[i] = 2
		return nil
	}
	for i := range tasks {
		if err := visit(i); err != nil {
			return err
		}
	}
	return nil
}
func (s *Session) initTerminal(j *job) {
	j.terminal = newTerminal(s.width, s.height, jobInput{j})
}

type jobInput struct{ j *job }

func (w jobInput) Write(p []byte) (int, error) {
	w.j.ioMu.Lock()
	child := w.j.child
	w.j.ioMu.Unlock()
	// Keep draining emulator replies across process exits and restarts. Writes to
	// an old PTY may fail; that must not leave the emulator's input pipe blocked.
	if child != nil {
		_, _ = child.Write(p)
	}
	return len(p), nil
}

type jobOutput struct {
	s *Session
	j *job
}

func (w jobOutput) Write(p []byte) (int, error) {
	w.s.mu.Lock()
	defer w.s.mu.Unlock()
	t := w.j.terminal
	before := t.ScrollbackLen()
	// Scrollback rows keep their cell storage as the bounded history rotates.
	// Anchor to a retained row so new output cannot move a paused viewport,
	// including after the history buffer has reached its capacity.
	var anchor *uv.Cell
	anchorIndex := max(0, before-w.j.offset)
	if w.j.offset > 0 {
		for ; anchorIndex < before; anchorIndex++ {
			if line := t.Scrollback().Line(anchorIndex); len(line) > 0 {
				anchor = &line[0]
				break
			}
		}
	}
	n, err := t.Write(p)
	if w.j.offset > 0 {
		delta := max(0, t.ScrollbackLen()-before)
		if anchor != nil {
			found := false
			for i, line := range t.Scrollback().Lines() {
				if len(line) > 0 && &line[0] == anchor {
					delta = t.ScrollbackLen() - before + anchorIndex - i
					found = true
					break
				}
			}
			if !found {
				delta = t.ScrollbackLen()
			}
		}
		w.j.offset = min(t.ScrollbackLen(), w.j.offset+delta)
	}
	return n, err
}

func (s *Session) start(index int, finished chan<- completion) {
	j := s.jobs[index]
	ctx, cancel := context.WithCancel(s.ctx)
	j.cancel = cancel
	j.started = time.Now()
	j.result.Status = "running"
	j.result.Err = nil
	j.result.ExitCode = 0
	j.attempt++
	go func() {
		var err error
		if s.opts.Run != nil {
			err = s.opts.Run(ctx, j.task, s.opts.Output)
		} else {
			err = s.execute(ctx, j)
		}
		cancel()
		finished <- completion{index, err}
	}()
}
func (s *Session) execute(ctx context.Context, j *job) error {
	if len(j.task.Argv) == 0 {
		return i18n.Errorf("task.command_missing", j.task.Name)
	}
	if s.opts.Mode == TUI {
		s.mu.Lock()
		width, height := s.width, s.height
		s.mu.Unlock()
		p, err := startPTY(j.task, width, height)
		if err != nil {
			return err
		}
		s.mu.Lock()
		j.ioMu.Lock()
		j.child = p
		_ = p.Resize(s.width, s.height)
		j.ioMu.Unlock()
		s.mu.Unlock()
		defer func() { j.ioMu.Lock(); j.child = nil; j.ioMu.Unlock(); _ = p.Close() }()
		return p.run(ctx, jobOutput{s, j})
	}
	cmd := platformprocess.CommandContext(ctx, j.task.Argv[0], j.task.Argv[1:]...)
	cmd.Dir = j.task.Directory
	cmd.Env = os.Environ()
	platformprocess.CancelProcessTree(cmd)
	prefix := "[" + j.task.Name + "] "
	if s.opts.Development {
		prefix = j.task.Name + " | "
	}
	writer := &lineWriter{out: s.opts.Output, mu: &s.outputMu, prefix: prefix}
	cmd.Stdout, cmd.Stderr = writer, writer
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
	writer.flush()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
func (s *Session) schedule() {
	defer close(s.done)
	finished := make(chan completion, len(s.jobs))
	running := 0
	halt := false
	for {
		s.mu.Lock()
		if s.ctx.Err() != nil {
			halt = true
			for _, j := range s.jobs {
				if j.cancel != nil {
					j.cancel()
				}
			}
		}
		if !halt {
			for i, j := range s.jobs {
				if running >= s.opts.Concurrency {
					break
				}
				if j.result.Status != "pending" {
					continue
				}
				ready := true
				for _, dep := range j.task.Dependencies {
					for _, d := range s.jobs {
						if d.task.Name == dep && d.result.Status != "succeeded" {
							ready = false
						}
					}
				}
				if ready {
					s.start(i, finished)
					running++
				}
			}
		}
		if running == 0 {
			for _, j := range s.jobs {
				if j.result.Status == "pending" {
					j.result.Status = "not_run"
				}
			}
			// Propagate failed dependency status in plan order until stable.
			for range s.jobs {
				for _, j := range s.jobs {
					if j.result.Status != "not_run" {
						continue
					}
					for _, dep := range j.task.Dependencies {
						for _, d := range s.jobs {
							if d.task.Name == dep && (d.result.Status == "failed" || d.result.Status == "blocked") {
								j.result.Status = "blocked"
							}
						}
					}
				}
			}
			for _, j := range s.jobs {
				s.results = append(s.results, j.result)
				if s.err == nil && j.result.Status == "failed" {
					s.err = &platformprocess.ExitStatus{Code: j.result.ExitCode}
				}
			}
			if s.ctx.Err() != nil {
				s.err = context.Cause(s.ctx)
				if errors.Is(s.err, context.Canceled) {
					s.err = &platformprocess.ExitStatus{Code: 130}
				}
			}
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
		select {
		case <-s.ctx.Done():
			// Drain completions without spinning on a permanently ready context.
			s.mu.Lock()
			halt = true
			for _, j := range s.jobs {
				if j.cancel != nil {
					j.cancel()
				}
			}
			s.mu.Unlock()
			for running > 0 {
				ev := <-finished
				s.mu.Lock()
				s.complete(ev)
				s.mu.Unlock()
				running--
			}
		case command := <-s.controls:
			s.mu.Lock()
			if command.index >= 0 && command.index < len(s.jobs) && s.opts.Development && !halt {
				j := s.jobs[command.index]
				if j.result.Status == "running" {
					j.action = command.action
					j.cancel()
					j.result.Status = "stopping"
				} else if command.action == "restart" {
					j.result.Status = "pending"
				}
			}
			s.mu.Unlock()
		case ev := <-finished:
			s.mu.Lock()
			running--
			j := s.jobs[ev.index]
			manual := j.action != ""
			s.complete(ev)
			if !manual && ((s.opts.Development && !s.opts.KeepGoing) || (!s.opts.Development && ev.err != nil)) {
				halt = true
				if s.opts.Development {
					for _, other := range s.jobs {
						if other.cancel != nil && other.result.Status == "running" {
							other.cancel()
						}
					}
				}
			}
			s.mu.Unlock()
		}
	}
}
func (s *Session) complete(ev completion) {
	j := s.jobs[ev.index]
	j.cancel = nil
	j.result.Duration = time.Since(j.started)
	j.result.Err = ev.err
	j.result.ExitCode = exitCode(ev.err)
	switch {
	case j.action == "restart" && s.ctx.Err() == nil:
		j.result.Status = "pending"
	case j.action == "stop" || errors.Is(ev.err, context.Canceled):
		j.result.Status = "stopped"
	case ev.err != nil:
		j.result.Status = "failed"
	default:
		j.result.Status = "succeeded"
	}
	if j.terminal != nil && ev.err != nil && !errors.Is(ev.err, context.Canceled) {
		_, _ = fmt.Fprintf(j.terminal, "\r\n[one] %v\r\n", ev.err)
	}
	if j.result.Status == "failed" && s.opts.Mode == Stream {
		s.outputMu.Lock()
		fmt.Fprintf(s.opts.Output, "[%s] %v\n", j.task.Name, ev.err)
		s.outputMu.Unlock()
	}
	j.action = ""
}
func exitCode(err error) int { return platformprocess.ExitCode(err) }

type lineWriter struct {
	out     io.Writer
	mu      *sync.Mutex
	prefix  string
	pending []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.pending = append(w.pending, p...)
	for len(w.pending) > 0 {
		end := strings.IndexByte(string(w.pending), '\n')
		if end < 0 {
			if len(w.pending) < 65536 {
				break
			}
			end = 65535
		}
		w.mu.Lock()
		_, err := fmt.Fprint(w.out, w.prefix, string(w.pending[:end+1]))
		w.mu.Unlock()
		if err != nil {
			return 0, err
		}
		w.pending = w.pending[end+1:]
	}
	return len(p), nil
}
func (w *lineWriter) flush() {
	if len(w.pending) > 0 {
		w.mu.Lock()
		_, _ = fmt.Fprintln(w.out, w.prefix+string(w.pending))
		w.mu.Unlock()
		w.pending = nil
	}
}

func statusError(err error) error { return &platformprocess.ExitStatus{Code: exitCode(err)} }

// SignalContext covers preparation as well as execution, preserving shell exit codes.
func SignalContext(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
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
	return ctx, func() { signal.Stop(signals); cancel(nil) }
}
