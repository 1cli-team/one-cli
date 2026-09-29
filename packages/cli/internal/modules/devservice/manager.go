// Package devservice owns Dashboard dev processes for the lifetime of one serve.
// Workers use the normal task pipeline; child output is forwarded unchanged.
package devservice

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/localurl"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
)

const maxLogBytes = 256 * 1024
const maxRuns = 128

type Input struct{ Root, Project, Environment, URL string }
type Event struct {
	Status string `json:"status"`
}
type Log struct {
	Seq  int64  `json:"seq"`
	Text string `json:"text"`
}
type Endpoint struct {
	URL       string `json:"url"`
	Reachable bool   `json:"reachable"`
}
type Snapshot struct {
	ID          string     `json:"id"`
	Project     string     `json:"project"`
	Environment string     `json:"environment"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"startedAt"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	ExitCode    *int       `json:"exitCode,omitempty"`
	Endpoints   []Endpoint `json:"endpoints"`
	Logs        []Log      `json:"logs"`
	FirstSeq    int64      `json:"firstSeq"`
	NextSeq     int64      `json:"nextSeq"`
}

func Active(status string) bool {
	return status == "preparing" || status == "running" || status == "stopping"
}

type run struct {
	input        Input
	state        Snapshot
	cancel       context.CancelFunc
	done         chan struct{}
	bytes        int
	tail         string
	pendingUTF8  string
	restart      bool
	restartInput Input
}
type Manager struct {
	mu       sync.Mutex
	runs     map[string]*run
	gates    map[string]chan struct{}
	closed   bool
	sequence uint64
	command  func(Input) (*exec.Cmd, error)
	grace    time.Duration
}

func New() *Manager {
	return &Manager{runs: map[string]*run{}, gates: map[string]chan struct{}{}, grace: 4 * time.Second, command: func(in Input) (*exec.Cmd, error) {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		cmd := process.Command(exe, "__service", "--project", in.Project, "--env", in.Environment, "-o", "json")
		cmd.Dir = in.Root
		// Bound waiting for inherited pipes even when a misbehaving child daemonizes.
		cmd.WaitDelay = 2 * time.Second
		return cmd, nil
	}}
}
func key(root, project string) string {
	if p, err := filepath.EvalSymlinks(root); err == nil {
		root = p
	}
	return filepath.Clean(root) + "\x00" + project
}
func (m *Manager) Start(in Input) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.start(in)
}
func (m *Manager) start(in Input) (Snapshot, error) {
	if m.closed {
		return Snapshot{}, i18n.Errorf("devservice.closed")
	}
	k := key(in.Root, in.Project)
	if previous := m.runs[k]; previous != nil && Active(previous.state.Status) {
		return snapshot(previous, "", 0), nil
	}
	if len(m.runs) >= maxRuns && m.runs[k] == nil {
		var oldestKey string
		var oldest *run
		for candidate, r := range m.runs {
			if !Active(r.state.Status) && (oldest == nil || r.state.StartedAt.Before(oldest.state.StartedAt)) {
				oldestKey, oldest = candidate, r
			}
		}
		if oldest == nil {
			return Snapshot{}, i18n.Errorf("devservice.limit")
		}
		delete(m.runs, oldestKey)
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.sequence++
	r := &run{input: in, cancel: cancel, done: make(chan struct{}), state: Snapshot{ID: fmt.Sprint(m.sequence), Project: in.Project, Environment: in.Environment, Status: "preparing", StartedAt: time.Now(), Endpoints: []Endpoint{}, NextSeq: 1}}
	if url, err := localurl.Normalize(in.URL); err == nil && url != "" {
		r.state.Endpoints = append(r.state.Endpoints, Endpoint{URL: url})
	}
	m.runs[k] = r
	rootKey := key(in.Root, "")
	gate := m.gates[rootKey]
	if gate == nil {
		gate = make(chan struct{}, 1)
		m.gates[rootKey] = gate
	}
	go m.execute(ctx, r, gate)
	return snapshot(r, "", 0), nil
}
func snapshot(r *run, id string, after int64) Snapshot {
	s := r.state
	s.Endpoints = append([]Endpoint{}, s.Endpoints...)
	s.Logs = []Log{}
	s.FirstSeq = s.NextSeq
	if len(r.state.Logs) > 0 {
		s.FirstSeq = r.state.Logs[0].Seq
	}
	if id != s.ID {
		after = 0
	}
	for _, line := range r.state.Logs {
		if line.Seq > after {
			s.Logs = append(s.Logs, line)
		}
	}
	return s
}
func (m *Manager) Get(root, project, id string, after int64) Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r := m.runs[key(root, project)]; r != nil {
		return snapshot(r, id, after)
	}
	return Snapshot{Project: project, Status: "stopped", Endpoints: []Endpoint{}, Logs: []Log{}, NextSeq: 1, FirstSeq: 1}
}
func (m *Manager) List(root string) []Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := []Snapshot{}
	for _, r := range m.runs {
		if key(r.input.Root, "") == key(root, "") {
			s := snapshot(r, r.state.ID, r.state.NextSeq)
			result = append(result, s)
		}
	}
	return result
}
func (m *Manager) Stop(root, project string) Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r := m.runs[key(root, project)]; r != nil {
		r.restart = false
		if Active(r.state.Status) {
			r.state.Status = "stopping"
			r.cancel()
		}
		return snapshot(r, "", 0)
	}
	return Snapshot{Project: project, Status: "stopped", Endpoints: []Endpoint{}, Logs: []Log{}, NextSeq: 1, FirstSeq: 1}
}
func (m *Manager) Restart(in Input) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Snapshot{}, i18n.Errorf("devservice.closed")
	}
	if r := m.runs[key(in.Root, in.Project)]; r != nil && Active(r.state.Status) {
		r.restart = true
		r.restartInput = in
		r.state.Status = "stopping"
		r.cancel()
		return snapshot(r, "", 0), nil
	}
	return m.start(in)
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	pending := []chan struct{}{}
	for _, r := range m.runs {
		if Active(r.state.Status) {
			r.state.Status = "stopping"
			r.cancel()
			pending = append(pending, r.done)
		}
	}
	m.mu.Unlock()
	for _, done := range pending {
		<-done
	}
}
func (m *Manager) execute(ctx context.Context, r *run, gate chan struct{}) {
	defer close(r.done)
	defer r.cancel()
	select {
	case gate <- struct{}{}:
	case <-ctx.Done():
		m.finish(r, 0)
		return
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { <-gate }) }
	defer release()
	if ctx.Err() != nil {
		m.finish(r, 0)
		return
	}
	cmd, err := m.command(r.input)
	if err != nil {
		m.append(r, err.Error()+"\n")
		m.finish(r, 1)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.append(r, err.Error()+"\n")
		m.finish(r, 1)
		return
	}
	control, err := cmd.StdinPipe()
	if err != nil {
		_ = stdout.Close()
		m.append(r, err.Error()+"\n")
		m.finish(r, 1)
		return
	}
	defer control.Close()
	cmd.Stderr = logWriter{m, r}
	if err = cmd.Start(); err != nil {
		_ = stdout.Close()
		m.append(r, err.Error()+"\n")
		m.finish(r, 1)
		return
	}
	eventsDone := make(chan struct{})
	go func() {
		defer close(eventsDone)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 4096), 16*1024)
		for scanner.Scan() {
			var event Event
			if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Status == "running" {
				m.mu.Lock()
				if r.state.Status == "preparing" {
					r.state.Status = "running"
				}
				m.mu.Unlock()
				release()
			}
		}
	}()
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case err = <-waited:
			_ = stdout.Close()
			<-eventsDone
			code := 0
			if err != nil {
				code = process.ExitCode(err)
			}
			m.finish(r, code)
			return
		case <-ticker.C:
			m.probe(ctx, r)
		case <-ctx.Done():
			// Closing the private control pipe lets the worker cancel and run
			// its secret-context cleanup on every platform, before forcing it.
			_ = control.Close()
			timer := time.NewTimer(m.grace)
			select {
			case err = <-waited:
				if !timer.Stop() {
					<-timer.C
				}
			case <-timer.C:
				_ = process.StopTree(cmd.Process)
				err = <-waited
			}
			_ = stdout.Close()
			<-eventsDone
			code := 0
			if err != nil {
				code = process.ExitCode(err)
			}
			m.finish(r, code)
			return
		}
	}
}
func (m *Manager) finish(r *run, code int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	r.state.FinishedAt = &now
	r.state.ExitCode = &code
	if r.state.Status == "stopping" {
		r.state.Status = "stopped"
	} else if code == 0 {
		r.state.Status = "exited"
	} else {
		r.state.Status = "failed"
	}
	for i := range r.state.Endpoints {
		r.state.Endpoints[i].Reachable = false
	}
	if r.restart && !m.closed {
		_, _ = m.start(r.restartInput)
	}
}

type logWriter struct {
	m *Manager
	r *run
}

func (w logWriter) Write(p []byte) (int, error) { w.m.append(w.r, string(p)); return len(p), nil }

var _ io.Writer = logWriter{}

func (m *Manager) append(r *run, text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	text = r.pendingUTF8 + text
	r.pendingUTF8 = ""
	if len(text) > 0 {
		last := len(text) - 1
		for last > 0 && !utf8.RuneStart(text[last]) {
			last--
		}
		if !utf8.FullRuneInString(text[last:]) {
			r.pendingUTF8 = text[last:]
			text = text[:last]
		}
	}
	r.tail += text
	if len(r.tail) > 8192 {
		r.tail = r.tail[len(r.tail)-8192:]
	}
	for _, url := range discoverURLs(r.tail) {
		found := false
		for _, entry := range r.state.Endpoints {
			if entry.URL == url {
				found = true
			}
		}
		if !found && len(r.state.Endpoints) < 8 {
			r.state.Endpoints = append(r.state.Endpoints, Endpoint{URL: url})
		}
	}
	for len(text) > 0 {
		n := min(len(text), 4096)
		if n < len(text) {
			for n > 0 && !utf8.RuneStart(text[n]) {
				n--
			}
			if n == 0 {
				n = min(len(text), 4096)
			}
		}
		r.state.Logs = append(r.state.Logs, Log{Seq: r.state.NextSeq, Text: text[:n]})
		r.state.NextSeq++
		r.bytes += n
		text = text[n:]
	}
	for r.bytes > maxLogBytes || len(r.state.Logs) > 2048 {
		r.bytes -= len(r.state.Logs[0].Text)
		r.state.Logs[0] = Log{}
		r.state.Logs = r.state.Logs[1:]
	}
}
