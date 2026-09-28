package serve

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/devservice"
)

func TestServiceRoutesResolveScopeAndRejectInvalidLaunches(t *testing.T) {
	withIsolatedConfig(t)
	root := seedRegistryWorkspace(t, "dev-id", "Development", "web")
	registry := newRegistryService(t)
	entry := observeRegistryWorkspace(t, registry, root)
	manager := devservice.New()
	manager.Close() // validate payloads without starting a process
	mux := BuildMux(MuxOpts{UIDisabled: true, ExpectedHosts: map[string]struct{}{registryTestHost: {}}, SelfOrigin: "http://" + registryTestHost, WorkspaceRoot: root, RegistryService: registry, ServiceManager: manager})
	for _, base := range []string{"/api/workspace", "/api/workspaces/" + entry.EntryID} {
		for _, tc := range []struct {
			path, body string
			code       int
		}{
			{"/projects/web/service/start", `{"environment":"dev","command":"touch surprise"}`, 400},
			{"/projects/missing/service/start", `{"environment":"dev"}`, 404},
			{"/projects/web/service/start", `{"environment":"../unsafe"}`, 400},
			{"/projects/web/service/start", `{"environment":"dev"}`, 409},
			{"/projects/web/service/restart", `{"environment":"dev"}`, 409},
			{"/projects/web/service/stop", `{}`, 202},
		} {
			rec := registryRequest(t, mux, http.MethodPost, base+tc.path, strings.NewReader(tc.body))
			if rec.Code != tc.code {
				t.Fatalf("%s: %d %s", tc.path, rec.Code, rec.Body.String())
			}
		}
		rec := registryRequest(t, mux, http.MethodGet, base+"/services", nil)
		if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(rec)
		}
	}
	req := httptest.NewRequest("POST", "/api/workspace/projects/web/service/start", strings.NewReader(`{}`))
	req.Host = registryTestHost
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatal(rec.Code)
	}
	rec = registryRequest(t, mux, http.MethodPost, "/api/workspaces/unknown/projects/web/service/start", strings.NewReader(`{}`))
	if rec.Code != 404 {
		t.Fatal(rec.Code)
	}
}
func TestServiceEventsFlushImmediatelyAndCancel(t *testing.T) {
	manager := devservice.New()
	defer manager.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { streamService(w, r, manager, "/test", "web") }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	line, err := bufio.NewReader(res.Body).ReadString('\n')
	if err != nil || !strings.Contains(line, `"status":"stopped"`) {
		t.Fatal(line, err)
	}
	cancel()
}
