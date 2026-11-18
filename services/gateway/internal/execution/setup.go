package execution

import (
	"context"
	"crypto/rand"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

const setupTimeout = 40 * time.Second

type Setup struct {
	local   func(context.Context, api.SetupInput) api.WorkReply
	mu      sync.Mutex
	pending map[string]*pendingSetup
}

type pendingSetup struct {
	command api.SetupCommand
	reply   chan api.WorkReply
}

func NewSetup(ctx context.Context, runner Runner) *Setup {
	return &Setup{local: newSetupService(ctx, runner).handle, pending: make(map[string]*pendingSetup)}
}

func ValidateWorkerHTTPS(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return i18n.Errorf("gateway.worker_https_address")
	}
	return nil
}

func (s *Setup) SecureWorkers(store *Store, loopbackProxy bool) http.Handler {
	workers := workerAPI(store, s)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if r.TLS == nil && !(loopbackProxy && net.ParseIP(host).IsLoopback() && r.Header.Get("X-Forwarded-Proto") == "https") {
			problem(w, http.StatusForbidden, "setup_https_required")
			return
		}
		workers.ServeHTTP(w, r)
	})
}

func (s *Setup) control(store *Store, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("worker")
	var input api.SetupInput
	if !readBody(w, r, 32<<10, &input) {
		problem(w, http.StatusBadRequest, "setup_invalid")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), setupTimeout)
	defer cancel()
	var reply api.WorkReply
	if id == LocalWorkerID {
		reply = s.local(ctx, input)
	} else {
		store.mu.Lock()
		worker, exists := store.workers[id]
		store.mu.Unlock()
		switch {
		case !exists:
			problem(w, http.StatusNotFound, "worker_not_found")
			return
		case !worker.SecurePairing:
			problem(w, http.StatusConflict, "setup_pair_https")
			return
		case !online(worker):
			problem(w, http.StatusConflict, "worker_offline")
			return
		}
		var ok bool
		reply, ok = s.forward(ctx, id, input)
		if !ok {
			problem(w, http.StatusConflict, "setup_busy")
			return
		}
	}
	respond(w, reply.Status, reply.Body)
}

func (s *Setup) forward(ctx context.Context, worker string, input api.SetupInput) (api.WorkReply, bool) {
	deadline, _ := ctx.Deadline()
	item := &pendingSetup{
		command: api.SetupCommand{ID: rand.Text(), Input: input, Deadline: deadline},
		reply:   make(chan api.WorkReply, 1),
	}
	s.mu.Lock()
	if s.pending[worker] != nil {
		s.mu.Unlock()
		return api.WorkReply{}, false
	}
	s.pending[worker] = item
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, worker)
		s.mu.Unlock()
	}()
	select {
	case reply := <-item.reply:
		return reply, true
	case <-ctx.Done():
		return api.SetupReply(http.StatusGatewayTimeout, map[string]string{"reason": "setup_timeout"}), true
	}
}

func (s *Setup) claim(w http.ResponseWriter, _ *http.Request, id string) {
	s.mu.Lock()
	var command *api.SetupCommand
	if item := s.pending[id]; item != nil {
		command = &item.command
	}
	s.mu.Unlock()
	respond(w, http.StatusOK, command)
}

func (s *Setup) complete(w http.ResponseWriter, r *http.Request, id string) {
	var result api.SetupResult
	if !readBody(w, r, 1<<20, &result) || !validSession(result.ID) ||
		result.Reply.Status < 200 || result.Reply.Status > 599 || len(result.Reply.Body) == 0 {
		problem(w, http.StatusBadRequest, "setup_invalid")
		return
	}
	s.mu.Lock()
	if item := s.pending[id]; item != nil && item.command.ID == result.ID {
		select {
		case item.reply <- result.Reply:
		default:
		}
	}
	s.mu.Unlock()
	respond(w, http.StatusOK, struct{}{})
}

func (l remoteLink) runSetup(ctx context.Context, handle func(context.Context, api.SetupInput) api.WorkReply) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var last api.SetupResult
	for ctx.Err() == nil {
		var cmd *api.SetupCommand
		err := l.send(ctx, l.endpoint("setup/claim"), struct{}{}, &cmd)
		if err == nil && cmd != nil && validSession(cmd.ID) {
			if last.ID != cmd.ID {
				duration := min(time.Until(cmd.Deadline), setupTimeout) - time.Second
				job, cancel := context.WithTimeout(ctx, duration)
				if job.Err() == nil {
					last = api.SetupResult{ID: cmd.ID, Reply: handle(job, cmd.Input)}
				}
				cancel()
			}
			if last.ID == cmd.ID {
				_ = l.send(ctx, l.endpoint("setup/result"), last, new(struct{}))
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
