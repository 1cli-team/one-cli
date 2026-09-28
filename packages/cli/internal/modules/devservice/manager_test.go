package devservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestServiceProcessHelper(t *testing.T) {
	mode := os.Getenv("ONE_TEST_DEV_SERVICE")
	if mode == "" {
		return
	}
	if mode == "child" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestServiceProcessHelper$")
		cmd.Env = append(os.Environ(), "ONE_TEST_DEV_SERVICE=stubborn")
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			os.Exit(2)
		}
	} else {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			os.Exit(3)
		}
		fmt.Fprintln(os.Stderr, "local http://"+listener.Addr().String()+"/")
		defer listener.Close()
	}
	_ = json.NewEncoder(os.Stdout).Encode(Event{Status: "running"})
	if mode == "normal" {
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	for {
		<-signals
		if mode == "normal" {
			os.Exit(0)
		}
	}
}
func testManager(t *testing.T, mode string) *Manager {
	t.Helper()
	m := New()
	m.grace = 100 * time.Millisecond
	m.command = func(in Input) (*exec.Cmd, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestServiceProcessHelper$")
		cmd.Env = append(os.Environ(), "ONE_TEST_DEV_SERVICE="+mode)
		cmd.WaitDelay = time.Second
		return cmd, nil
	}
	t.Cleanup(m.Close)
	return m
}
func awaitState(t *testing.T, m *Manager, in Input, check func(Snapshot) bool) Snapshot {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		s := m.Get(in.Root, in.Project, "", 0)
		if check(s) {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("unexpected service state: %+v", m.Get(in.Root, in.Project, "", 0))
	return Snapshot{}
}
func TestLifecycleDeduplicatesRestartsAndReleasesPort(t *testing.T) {
	for _, mode := range []string{"normal", "child"} {
		t.Run(mode, func(t *testing.T) {
			m := testManager(t, mode)
			in := Input{Root: t.TempDir(), Project: "web", Environment: "dev"}
			var wg sync.WaitGroup
			ids := make(chan string, 16)
			for i := 0; i < 16; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					s, err := m.Start(in)
					if err != nil {
						t.Error(err)
					}
					ids <- s.ID
				}()
			}
			wg.Wait()
			close(ids)
			for id := range ids {
				if id != "1" {
					t.Errorf("duplicate run: %s", id)
				}
			}
			first := awaitState(t, m, in, func(s Snapshot) bool {
				return s.Status == "running" && len(s.Endpoints) > 0 && s.Endpoints[0].Reachable
			})
			in.Environment = "prod"
			if _, err := m.Restart(in); err != nil {
				t.Fatal(err)
			}
			second := awaitState(t, m, in, func(s Snapshot) bool {
				return s.ID != first.ID && s.Status == "running" && len(s.Endpoints) > 0 && s.Endpoints[0].Reachable
			})
			if second.Environment != "prod" {
				t.Fatal(second)
			}
			m.Stop(in.Root, in.Project)
			awaitState(t, m, in, func(s Snapshot) bool { return s.Status == "stopped" })
			for _, endpoint := range append(first.Endpoints, second.Endpoints...) {
				conn, err := net.DialTimeout("tcp", strings.TrimSuffix(strings.TrimPrefix(endpoint.URL, "http://"), "/"), 100*time.Millisecond)
				if err == nil {
					conn.Close()
					t.Fatal("service child port survived shutdown")
				}
			}
		})
	}
}
func TestStopAndCloseCancelQueuedPreparation(t *testing.T) {
	m := testManager(t, "normal")
	in := Input{Root: t.TempDir(), Project: "web"}
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	m.gates[key(in.Root, "")] = gate
	first, _ := m.Start(in)
	m.Stop(in.Root, in.Project)
	awaitState(t, m, in, func(s Snapshot) bool { return s.Status == "stopped" })
	second, _ := m.Start(in)
	if first.ID == second.ID {
		t.Fatal("run not replaced")
	}
	m.Close()
	if _, err := m.Start(in); err == nil {
		t.Fatal("started after close")
	}
}
func TestLogsBoundedCursorAndWorkspaceIsolation(t *testing.T) {
	m := New()
	r := &run{input: Input{Root: t.TempDir(), Project: "web"}, state: Snapshot{ID: "1", NextSeq: 1}}
	m.runs[key(r.input.Root, r.input.Project)] = r
	for i := 0; i < 100; i++ {
		m.append(r, strings.Repeat("hello", 1000))
	}
	s := m.Get(r.input.Root, "web", "", 0)
	size := 0
	for _, line := range s.Logs {
		size += len(line.Text)
	}
	if size > maxLogBytes || s.FirstSeq <= 1 {
		t.Fatal("unbounded logs")
	}
	m.append(r, "tail\n")
	delta := m.Get(r.input.Root, "web", s.ID, s.NextSeq-1)
	if len(delta.Logs) != 1 || delta.Logs[0].Text != "tail\n" {
		t.Fatal(delta)
	}
	if got := m.Get(t.TempDir(), "web", "", 0); got.ID != "" {
		t.Fatal("workspace isolation failed")
	}
	if got := m.Get(r.input.Root, "web", "old-run", 99999); len(got.Logs) == 0 {
		t.Fatal("new run did not reset cursor")
	}
}
func TestDiscoverOnlyLocalEndpoints(t *testing.T) {
	urls := discoverURLs("\x1b[32mhttp://0.0.0.0:3000/\x1b[0m\nhttps://docs.example.com http://localhost:3000/?token=secret http://user:pw@127.0.0.1:3000/")
	if len(urls) != 1 || urls[0] != "http://127.0.0.1:3000/" {
		t.Fatal(urls)
	}
	m := New()
	r := &run{state: Snapshot{Endpoints: []Endpoint{{URL: "http://127.0.0.1:1/"}}, Status: "running"}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.probe(ctx, r)
	if r.state.Endpoints[0].Reachable {
		t.Fatal("closed port is reachable")
	}
}

func TestLogsPreserveUTF8AcrossWrites(t *testing.T) {
	m := New()
	r := &run{state: Snapshot{NextSeq: 1}}
	raw := []byte("中文开发服务\n")
	for _, b := range raw {
		m.append(r, string([]byte{b}))
	}
	var combined strings.Builder
	for _, line := range r.state.Logs {
		combined.WriteString(line.Text)
	}
	if combined.String() != string(raw) {
		t.Fatal(combined.String())
	}
}

func TestPartialURLIsNotPublished(t *testing.T) {
	m := New()
	r := &run{state: Snapshot{NextSeq: 1}}
	m.append(r, "http://localhost:3")
	if len(r.state.Endpoints) != 0 {
		t.Fatal("partial port was published")
	}
	m.append(r, "000/?token=test\n")
	if len(r.state.Endpoints) != 0 {
		t.Fatal("credential URL was published")
	}
	m.append(r, "http://localhost:4000/\n")
	if len(r.state.Endpoints) != 1 || r.state.Endpoints[0].URL != "http://localhost:4000/" {
		t.Fatal(r.state.Endpoints)
	}
}
