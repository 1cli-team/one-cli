package infisical

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/infisical/go-sdk/packages/models"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
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

func waitSnapshotRequest(t *testing.T, started <-chan struct{}) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("snapshot request did not start")
	}
}

func TestBatchFetchReadsOnceAndMergesFreshSnapshots(t *testing.T) {
	var calls, generation atomic.Int32
	root := batchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Path != "/api/v3/secrets/raw" || q.Get("workspaceId") != "remote" || q.Get("environment") != "dev" || q.Get("secretPath") != "/" || q.Get("expandSecretReferences") != "true" || q.Get("recursive") != "true" || q.Get("include_imports") != "false" || r.Header.Get("Authorization") == "" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		calls.Add(1)
		// Children precede their parents in the response. Only inheritance
		// order, never response order, may decide which value wins.
		folders := []struct {
			path   string
			values map[string]string
		}{
			{"/services/server", map[string]string{"OVERRIDE": "server", "SERVER_ONLY": "yes"}},
			{"/apps/home", map[string]string{"OVERRIDE": "home", "HOME_ONLY": "yes"}},
			{"/apps/home/nested", map[string]string{"NESTED_ONLY": "yes", "OVERRIDE": "nested"}},
			{"/apps/home-other", map[string]string{"PREFIX_ONLY": "yes"}},
			{"/unrelated", map[string]string{"UNRELATED_ONLY": "yes"}},
			{"/apps", map[string]string{"OVERRIDE": "apps"}},
			{"/services", map[string]string{"OVERRIDE": "services"}},
			{"/", map[string]string{"SHARED": "root", "OVERRIDE": "root"}},
		}
		values := []models.Secret{}
		for _, folder := range folders {
			for key, value := range folder.values {
				values = append(values, models.Secret{SecretPath: folder.path, SecretKey: key, SecretValue: fmt.Sprintf("%d:%s", generation.Load(), value)})
			}
		}
		values = append(values, models.Secret{SecretPath: "/", SecretKey: "EMPTY", SecretValue: ""})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"secrets": values})
	})
	dirs := []string{"services/server", "apps/home", "apps/admin", "apps/home", "", "missing/project"}
	got, err := secrets.LoadProjects(context.Background(), Loader(), root, dirs, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || len(got) != 5 {
		t.Fatalf("requests = %d, snapshots = %d", calls.Load(), len(got))
	}
	for _, tc := range []struct{ dir, override string }{{"services/server", "server"}, {"apps/home", "home"}, {"apps/admin", "apps"}, {"", "root"}, {"missing/project", "root"}} {
		values := got[tc.dir]
		if values["SHARED"] != "0:root" || values["OVERRIDE"] != "0:"+tc.override {
			t.Fatalf("wrong inheritance: %v", got)
		}
		if empty, exists := values["EMPTY"]; !exists || empty != "" {
			t.Fatal("empty value was lost")
		}
		for _, key := range []string{"NESTED_ONLY", "PREFIX_ONLY", "UNRELATED_ONLY"} {
			if _, exists := values[key]; exists {
				t.Fatalf("%s leaked into %s", key, tc.dir)
			}
		}
	}
	if got["apps/home"]["SERVER_ONLY"] != "" || got["apps/admin"]["HOME_ONLY"] != "" || got[""]["HOME_ONLY"] != "" {
		t.Fatal("project values leaked")
	}
	got["apps/home"]["SHARED"] = "modified"
	if got["apps/admin"]["SHARED"] != "0:root" {
		t.Fatal("project maps alias")
	}
	generation.Store(1)
	next, err := secrets.LoadProjects(context.Background(), Loader(), root, dirs, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if next["apps/home"]["SHARED"] != "1:root" || calls.Load() != 2 {
		t.Fatal("a later invocation did not read exactly one fresh snapshot")
	}
}

func TestBatchFetchReadsManyProjectsOnce(t *testing.T) {
	var calls atomic.Int32
	root := batchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"secrets":[]}`))
	})
	dirs := []string{}
	for i := range 20 {
		dirs = append(dirs, fmt.Sprintf("apps/p%d", i))
	}
	got, err := fetchSecretsForProjects(context.Background(), root, dirs, "dev")
	if err != nil || len(got) != len(dirs) || calls.Load() != 1 {
		t.Fatalf("snapshots = %d, requests = %d, error = %v", len(got), calls.Load(), err)
	}
	for _, dir := range dirs {
		if got[dir] == nil || len(got[dir]) != 0 {
			t.Fatalf("missing empty snapshot for %s", dir)
		}
	}
	got, err = fetchSecretsForProjects(context.Background(), t.TempDir(), nil, "dev")
	if err != nil || len(got) != 0 || calls.Load() != 1 {
		t.Fatal("empty batch required configuration or performed a request")
	}
}

func TestBatchFetchFailureReturnsNoSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		code   string
	}{
		{"unauthorized", 401, "INFISICAL_AUTH_FAILED"},
		{"forbidden", 403, "INFISICAL_API_ERROR"},
		{"project_missing", 404, "INFISICAL_API_ERROR"},
		{"rate_limited", 429, "INFISICAL_API_ERROR"},
		{"server_error", 500, "INFISICAL_API_ERROR"},
		{"connection_closed", 0, "INFISICAL_API_ERROR"},
		{"invalid_json", 200, "INFISICAL_API_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			root := batchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if tc.status == 0 {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						_ = conn.Close()
					}
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				if tc.status == 200 {
					_, _ = w.Write([]byte(`{"secrets":`))
					return
				}
				_, _ = w.Write([]byte(`{"message":"request rejected: synthetic-secret-value"}`))
			})
			values, err := fetchSecretsForProjects(context.Background(), root, []string{"services/server", "apps/home", "apps/admin"}, "dev")
			var coded interface{ ErrorCode() string }
			if values != nil || !errors.As(err, &coded) || coded.ErrorCode() != tc.code || calls.Load() != 1 {
				t.Fatalf("error = %v, requests = %d, want %s", err, calls.Load(), tc.code)
			}
			if strings.Contains(err.Error(), "synthetic-secret-value") {
				t.Fatal("error exposed the response body")
			}
		})
	}
}

func TestBatchFetchRejectsMissingSecretPaths(t *testing.T) {
	for _, locale := range []string{"en-US", "zh-CN"} {
		t.Run(locale, func(t *testing.T) {
			if err := i18n.Init(locale); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = i18n.Init("en-US") })
			root := batchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"secrets":[{"secretPath":"/apps/home","secretKey":"VALID","secretValue":"synthetic-secret-value"},{"secretKey":"PRIVATE","secretValue":"synthetic-secret-value"}]}`))
			})
			values, err := fetchSecretsForProjects(context.Background(), root, []string{"apps/home", "apps/admin"}, "dev")
			var coded interface{ ErrorCode() string }
			if values != nil || !errors.As(err, &coded) || coded.ErrorCode() != "INFISICAL_API_ERROR" || err.Error() != i18n.T("infisical.snapshot_path_missing") {
				t.Fatalf("missing secret path was not rejected: %v", err)
			}
			if strings.Contains(err.Error(), "synthetic-secret-value") {
				t.Fatal("error exposed secret values")
			}
		})
	}
}

func TestBatchFetchChecksRecursiveDepth(t *testing.T) {
	dir := strings.Repeat("nested/", maxSnapshotDepth-1) + "project"
	var calls atomic.Int32
	root := batchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"secrets": []models.Secret{{SecretPath: "/" + dir, SecretKey: "DEEPEST", SecretValue: "yes"}}})
	})
	got, err := fetchSecretsForProjects(context.Background(), root, []string{dir}, "dev")
	if err != nil || got[dir]["DEEPEST"] != "yes" || calls.Load() != 1 {
		t.Fatalf("supported depth failed: %v %v", got, err)
	}
	for _, locale := range []string{"en-US", "zh-CN"} {
		t.Run(locale, func(t *testing.T) {
			if err := i18n.Init(locale); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = i18n.Init("en-US") })
			tooDeep := dir + "/child"
			got, err := fetchSecretsForProjects(context.Background(), root, []string{"apps/home", tooDeep}, "dev")
			if got != nil || err == nil || err.Error() != i18n.Tf("infisical.snapshot_depth_exceeded", tooDeep, maxSnapshotDepth) || calls.Load() != 1 {
				t.Fatalf("unsupported depth was not rejected before fetching: %v", err)
			}
		})
	}
}

func TestBatchFetchCancellation(t *testing.T) {
	started := make(chan struct{}, 1)
	cancelled := make(chan struct{}, 1)
	var calls atomic.Int32
	root := batchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		started <- struct{}{}
		<-r.Context().Done()
		cancelled <- struct{}{}
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() {
		_, err := fetchSecretsForProjects(ctx, root, []string{"apps/home", "apps/admin"}, "dev")
		done <- err
	}()
	waitSnapshotRequest(t, started)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP request ignored cancellation")
	}
	waitSnapshotRequest(t, cancelled)
	values, err := fetchSecretsForProjects(ctx, root, []string{"apps/home"}, "dev")
	if !errors.Is(err, context.Canceled) || values != nil || calls.Load() != 1 {
		t.Fatal("pre-cancelled load did work")
	}
}

func TestBatchFetchEnvironmentIsolation(t *testing.T) {
	var calls atomic.Int32
	root := batchWorkspace(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"secrets": []models.Secret{{SecretPath: "/", SecretKey: "ENV", SecretValue: r.URL.Query().Get("environment")}}})
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
	if calls.Load() != 2 {
		t.Fatalf("requests = %d, want one per environment", calls.Load())
	}
}
