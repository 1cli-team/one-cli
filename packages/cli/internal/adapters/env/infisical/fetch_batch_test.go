package infisical

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/zalando/go-keyring"
)

func batchWorkspace(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	sharedTestSession(t, server.URL, "user")
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{Version: 2, Env: &workspace.EnvironmentConfig{SiteURL: server.URL, ProjectID: "remote", Environments: []string{"dev", "staging"}}}); err != nil {
		t.Fatal(err)
	}
	return root
}

func waitFolderRequests(t *testing.T, started <-chan string, n int) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for range n {
		select {
		case <-started:
		case <-timer.C:
			t.Fatal("folder reads did not overlap")
		}
	}
}

func TestBatchFetchDeduplicatesAndMergesFreshSnapshots(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]int{}
	var generation atomic.Int32
	started := make(chan string, 32)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	// Registered after server cleanup as well, so a failing test releases handlers.
	folders := map[string]map[string]string{
		"/":                {"SHARED": "root", "OVERRIDE": "root"},
		"/services":        {"OVERRIDE": "services"},
		"/services/server": {"OVERRIDE": "server", "SERVER_ONLY": "yes"},
		"/apps":            {"OVERRIDE": "apps"},
		"/apps/home":       {"OVERRIDE": "home", "HOME_ONLY": "yes"},
	}
	root := batchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		path := q.Get("secretPath")
		if r.Method != http.MethodGet || r.URL.Path != "/api/v3/secrets/raw" || q.Get("workspaceId") != "remote" || q.Get("environment") != "dev" || q.Get("expandSecretReferences") != "true" || q.Get("recursive") != "false" || r.Header.Get("Authorization") == "" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		mu.Lock()
		calls[path]++
		mu.Unlock()
		started <- path
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if path == "/apps/admin" {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Folder with path '/apps/admin' in environment 'dev' was not found."})
			return
		}
		// Parents finish last: completion order must never define precedence.
		if path == "/" {
			time.Sleep(20 * time.Millisecond)
		}
		values := []map[string]string{}
		for key, value := range folders[path] {
			values = append(values, map[string]string{"secretKey": key, "secretValue": fmt.Sprintf("%d:%s", generation.Load(), value)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"secrets": values})
	})
	t.Cleanup(unblock)
	dirs := []string{"services/server", "apps/home", "apps/admin", "apps/home"}
	type result struct {
		values map[string]map[string]string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		values, err := fetchSecretsForProjects(context.Background(), root, dirs, "dev")
		done <- result{values, err}
	}()
	waitFolderRequests(t, started, 6)
	unblock()
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	for _, tc := range []struct{ dir, override string }{{"services/server", "server"}, {"apps/home", "home"}, {"apps/admin", "apps"}} {
		values := got.values[tc.dir]
		if values["SHARED"] != "0:root" || values["OVERRIDE"] != "0:"+tc.override {
			t.Fatalf("wrong inheritance: %v", got.values)
		}
	}
	if got.values["apps/home"]["SERVER_ONLY"] != "" || got.values["apps/admin"]["HOME_ONLY"] != "" {
		t.Fatal("project values leaked")
	}
	got.values["apps/home"]["SHARED"] = "modified"
	if got.values["apps/admin"]["SHARED"] != "0:root" {
		t.Fatal("project maps alias")
	}
	mu.Lock()
	for path, n := range calls {
		if n != 1 {
			t.Errorf("%s read %d times", path, n)
		}
	}
	mu.Unlock()
	generation.Store(1)
	next, err := fetchSecretsForProjects(context.Background(), root, dirs, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if next["apps/home"]["SHARED"] != "1:root" {
		t.Fatal("a later invocation reused stale values")
	}
	mu.Lock()
	defer mu.Unlock()
	for path, n := range calls {
		if n != 2 {
			t.Errorf("%s read %d times after two invocations", path, n)
		}
	}
}

func TestBatchFetchBoundsConcurrency(t *testing.T) {
	var active, peak atomic.Int32
	started := make(chan string, 64)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	root := batchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		started <- r.URL.Query().Get("secretPath")
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"secrets":[]}`))
	})
	t.Cleanup(unblock)
	dirs := []string{}
	for i := range 20 {
		dirs = append(dirs, fmt.Sprintf("apps/p%d", i))
	}
	done := make(chan error, 1)
	go func() { _, err := fetchSecretsForProjects(context.Background(), root, dirs, "dev"); done <- err }()
	waitFolderRequests(t, started, maxFolderRequests)
	unblock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if peak.Load() != maxFolderRequests {
		t.Fatalf("peak concurrent requests = %d", peak.Load())
	}
}

func TestBatchFetchFailureCancelsReads(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		code   string
	}{
		{"unauthorized", 401, "INFISICAL_AUTH_FAILED"},
		{"forbidden", 403, "INFISICAL_API_ERROR"},
		{"project_missing", 404, "INFISICAL_API_ERROR"},
		{"server_error", 500, "INFISICAL_API_ERROR"},
		{"connection_closed", 0, "INFISICAL_API_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started := make(chan string, 6)
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			ctx, cancel := context.WithCancel(context.Background())
			root := batchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
				path := r.URL.Query().Get("secretPath")
				started <- path
				if path != "/" {
					<-r.Context().Done()
					return
				}
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				if tc.status == 0 {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						_ = conn.Close()
					}
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"request rejected"}`))
			})
			t.Cleanup(cancel)
			t.Cleanup(unblock)
			done := make(chan error, 1)
			go func() {
				values, err := fetchSecretsForProjects(ctx, root, []string{"services/server", "apps/home", "apps/admin"}, "dev")
				if values != nil {
					t.Error("partial snapshot returned on failure")
				}
				done <- err
			}()
			waitFolderRequests(t, started, 6)
			unblock()
			select {
			case err := <-done:
				var coded interface{ ErrorCode() string }
				if !errors.As(err, &coded) || coded.ErrorCode() != tc.code {
					t.Fatalf("error = %v, want %s", err, tc.code)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("failed sibling did not cancel outstanding HTTP requests")
			}
		})
	}
}

func TestBatchFetchCancellation(t *testing.T) {
	started := make(chan string, 6)
	root := batchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		started <- r.URL.Query().Get("secretPath")
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { _, err := fetchSecretsForProjects(ctx, root, []string{"apps/home"}, "dev"); done <- err }()
	waitFolderRequests(t, started, 3)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP requests ignored cancellation")
	}
	values, err := fetchSecretsForProjects(ctx, root, []string{"apps/home"}, "dev")
	if !errors.Is(err, context.Canceled) || values != nil {
		t.Fatal("pre-cancelled load did work")
	}
}

func TestBatchFetchEnvironmentIsolation(t *testing.T) {
	root := batchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"secrets": []map[string]string{{"secretKey": "ENV", "secretValue": r.URL.Query().Get("environment")}}})
	})
	var wg sync.WaitGroup
	for _, env := range []string{"dev", "staging"} {
		wg.Go(func() {
			got, err := fetchSecretsForProjects(context.Background(), root, []string{"apps/home", "apps/admin"}, env)
			want := map[string]map[string]string{"apps/home": {"ENV": env}, "apps/admin": {"ENV": env}}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("%s: %v %v", env, got, err)
			}
		})
	}
	wg.Wait()
}
