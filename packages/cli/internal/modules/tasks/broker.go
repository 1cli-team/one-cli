package tasks

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/taskui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
	"net"
	"net/http"
	"sync"
	"time"
)

type leafSpec struct {
	Task        Task     `json:"task"`
	Commands    []string `json:"commands"`
	Environment []string `json:"environment"`
	Arguments   []string `json:"arguments,omitempty"`
	Shell       []string `json:"shell,omitempty"`
	Force       bool     `json:"force"`
}
type leafEvent struct {
	Status    string    `json:"status"`
	ExitCode  int       `json:"exit_code"`
	StartedAt time.Time `json:"-"`
	EndedAt   time.Time `json:"-"`
}
type invocationBroker struct {
	endpoint, token string
	server          *http.Server
	specs           map[string]leafSpec
	mu              sync.Mutex
	events          map[string]leafEvent
	firstFailure    int
}

func newInvocationBroker(specs map[string]leafSpec) (*invocationBroker, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	b := &invocationBroker{endpoint: "http://" + listener.Addr().String(), token: hex.EncodeToString(token), specs: specs, events: map[string]leafEvent{}}
	b.server = &http.Server{Handler: b, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, IdleTimeout: 3 * time.Second, MaxHeaderBytes: 4096}
	go func() { _ = b.server.Serve(listener) }()
	return b, nil
}
func (b *invocationBroker) close() { _ = b.server.Close() }
func (b *invocationBroker) environment(env []string) []string {
	return secrets.MergeIntoEnviron(env, map[string]string{"ONE_PROCESS_ENDPOINT": b.endpoint, "ONE_PROCESS_TOKEN": b.token}, true)
}
func (b *invocationBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+b.token)) != 1 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	id := r.URL.Path[1:]
	spec, ok := b.specs[id]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(spec)
	case http.MethodPost:
		var event leafEvent
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&event); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch event.Status {
		case "running", "succeeded", "failed", "cancelled", "cached":
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		b.mu.Lock()
		previous := b.events[id]
		event.StartedAt = previous.StartedAt
		if event.Status == "running" && event.StartedAt.IsZero() {
			event.StartedAt = time.Now()
		}
		if event.Status != "running" {
			event.EndedAt = time.Now()
		}
		b.events[id] = event
		if event.Status == "failed" && event.ExitCode != 0 && b.firstFailure == 0 {
			b.firstFailure = event.ExitCode
		}
		b.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
func (b *invocationBroker) tasks(plan *Plan, status string) []Task {
	b.mu.Lock()
	defer b.mu.Unlock()
	tasks := append([]Task(nil), plan.Tasks...)
	for id, spec := range b.specs {
		for i := range tasks {
			if tasks[i].Name != spec.Task.Name {
				continue
			}
			event, ok := b.events[id]
			if !ok {
				tasks[i].Status = "skipped"
			} else if event.Status == "running" && status != "succeeded" {
				tasks[i].Status = "cancelled"
			} else {
				tasks[i].Status = event.Status
			}
		}
	}
	return tasks
}

// snapshot contains display data only. No command or environment leaves the broker.
func (b *invocationBroker) snapshot() []taskui.State {
	b.mu.Lock()
	defer b.mu.Unlock()
	states := make([]taskui.State, 0, len(b.events))
	for id, event := range b.events {
		states = append(states, taskui.State{Name: b.specs[id].Task.Name, Status: event.Status, StartedAt: event.StartedAt, EndedAt: event.EndedAt, ExitCode: event.ExitCode})
	}
	return states
}
