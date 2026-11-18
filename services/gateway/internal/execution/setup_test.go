package execution

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/openviking"
)

func TestSetupRoutesOnlyToTheHTTPSPairedTarget(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root, "thunder")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	setup := NewSetup(ctx, Runner{})
	secure := setup.SecureWorkers(store, true)
	adminToken := strings.Repeat("a", 32)
	control := Control(adminToken, store, http.NotFoundHandler(), "https://worker.example", setup)

	register := func(name string, handler http.Handler) remoteLink {
		t.Helper()
		cfg := workerConfig{URL: "https://worker.example", ID: "w-" + strings.ToLower(rand.Text()), Name: name, Token: rand.Text() + rand.Text()}
		link, err := newRemote(cfg)
		if err != nil {
			t.Fatal(err)
		}
		link.http.Transport = loopTransport{handler}
		if err := link.send(ctx, link.base.JoinPath("worker", "v1", "join"), joinRequest{ID: cfg.ID, Name: name, Code: store.invite().Code}, new(struct{})); err != nil {
			t.Fatal(err)
		}
		if err := link.heartbeat(ctx, rand.Text(), api.ExecutionWorker{}); err != nil {
			t.Fatal(err)
		}
		return link
	}
	httpsProxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = "127.0.0.1:48633"
		r.Header.Set("X-Forwarded-Proto", "https")
		secure.ServeHTTP(w, r)
	})
	nova := register("nova", httpsProxy)
	other := register("other", httpsProxy)
	legacy := register("legacy", WorkerAPI(store))

	runControl := func(worker, token string) *httptest.ResponseRecorder {
		body := `{"operation":"gitea","gitea":{"url":"https://git.example","token":"fixture-private-token"}}`
		r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/workers/"+worker+"/setup", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		control.ServeHTTP(w, r)
		return w
	}
	if got := runControl(nova.config.ID, nova.config.Token); got.Code != http.StatusUnauthorized {
		t.Fatalf("worker credential allowed to configure machines: %d", got.Code)
	}
	if got := runControl(legacy.config.ID, adminToken); got.Code != http.StatusConflict || !strings.Contains(got.Body.String(), "setup_pair_https") {
		t.Fatalf("legacy worker accepted configuration: %d %s", got.Code, got.Body.String())
	}
	plaintext := nova
	plaintext.http = &http.Client{Transport: loopTransport{WorkerAPI(store)}}
	if err := plaintext.heartbeat(ctx, rand.Text(), api.ExecutionWorker{}); err == nil {
		t.Fatal("HTTPS pairing accepted on the legacy listener")
	}
	forgedProxy := nova
	forgedProxy.http = &http.Client{Transport: loopTransport{http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = "192.0.2.15:40000"
		r.Header.Set("X-Forwarded-Proto", "https")
		secure.ServeHTTP(w, r)
	})}}
	if err := forgedProxy.heartbeat(ctx, rand.Text(), api.ExecutionWorker{}); err == nil {
		t.Fatal("untrusted proxy header authorized a plaintext request")
	}

	called := make(chan struct{}, 1)
	release := make(chan struct{})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- runControl(nova.config.ID, adminToken) }()
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		nova.runSetup(ctx, func(_ context.Context, input api.SetupInput) api.WorkReply {
			if input.Operation != api.SetupGitea || input.Gitea == nil || input.Gitea.Token != "fixture-private-token" {
				t.Error("target received the wrong setup input")
			}
			called <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
			}
			return api.SetupReply(http.StatusOK, codehost.Connection{Host: "git.example", Account: "nova"})
		})
	}()
	defer func() { cancel(); <-workerDone }()
	select {
	case <-called:
	case <-ctx.Done():
		t.Fatal("target never received setup")
	}
	var wrong *api.SetupCommand
	if err := other.send(ctx, other.endpoint("setup/claim"), struct{}{}, &wrong); err != nil || wrong != nil {
		t.Fatalf("another worker could claim the setup request: %v", err)
	}

	close(release)
	response := <-done
	var connection codehost.Connection
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &connection) != nil || connection.Account != "nova" {
		t.Fatalf("setup result was not returned: %d %s", response.Code, response.Body.String())
	}
	if len(called) != 0 || strings.Contains(response.Body.String(), "fixture-private-token") {
		t.Fatal("setup did not run once or response leaked a credential")
	}
	body, err := os.ReadFile(filepath.Join(root, "workers.json"))
	if err != nil || strings.Contains(string(body), "fixture-private-token") {
		t.Fatalf("setup credential was recorded in the worker journal: %v", err)
	}
	setup.mu.Lock()
	defer setup.mu.Unlock()
	if len(setup.pending) != 0 {
		t.Fatal("completed setup inputs were retained")
	}
}

func TestOpenVikingSetupValidatesBeforeSaving(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	const secret = "fixture-openviking-secret"
	service := newSetupService(t.Context(), Runner{})
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/fs/ls":
			requests.Add(1)
			_, _ = w.Write([]byte(`{"status":"ok","result":[]}`))
		case "/health":
			_, _ = w.Write([]byte(`{"account_id":"team","user_id":"test"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	sync := func(key string) api.WorkReply {
		return service.handle(t.Context(), api.SetupInput{
			Operation:  api.SetupOpenViking,
			OpenViking: &knowledge.ConnectionInput{URL: upstream.URL, APIKey: &key},
		})
	}
	reply := sync(secret)
	if reply.Status != http.StatusOK || requests.Load() != 1 || strings.Contains(string(reply.Body), secret) ||
		!strings.Contains(string(reply.Body), upstream.URL) {
		t.Fatalf("OpenViking setup returned HTTP %d", reply.Status)
	}
	if reply := sync("invalid"); reply.Status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid credentials returned HTTP %d", reply.Status)
	}
	public, err := openviking.New().TestConnection(t.Context(), knowledge.ConnectionInput{})
	if err != nil || public.Account != "team" || !public.HasAPIKey || requests.Load() != 2 {
		t.Fatal("failed setup changed the validated connection")
	}
}
