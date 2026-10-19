package oneclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
)

func TestProjectRoutingAndFreshEnvironmentSnapshots(t *testing.T) {
	t.Setenv("ONE_SERVER_TOKEN", "test-token")
	t.Setenv("ONE_SERVER_URL", "")
	ctx := context.Background()
	root := t.TempDir()
	m := &workspace.Manifest{Version: 2, Workspace: &workspace.ManifestWorkspace{ID: "test-workspace", Name: "Test"}, Projects: []workspace.ManifestProject{{Name: "api", RelativeDir: "services/api", Toolchain: "go"}, {Name: "ui", RelativeDir: "apps/ui", Toolchain: "node"}}}
	if err := workspace.WriteManifest(root, m); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	counts := map[string]int{}
	server := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Error("missing credential")
			}
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/v1/workspaces":
				copy := *m
				copy.Env = &workspace.EnvironmentConfig{ProjectID: "provider-" + name}
				json.NewEncoder(w).Encode(Snapshot{Manifest: &copy, EntryID: m.Workspace.ID, Revision: "2"})
			case "/api/v1/workspaces/test-workspace/operations/snapshot":
				var body struct {
					Dirs []string `json:"dirs"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				mu.Lock()
				counts[name]++
				count := counts[name]
				mu.Unlock()
				values := map[string]map[string]string{}
				for _, dir := range body.Dirs {
					values[dir] = map[string]string{"SOURCE": fmt.Sprintf("%s-%d", name, count)}
				}
				json.NewEncoder(w).Encode(values)
			default:
				t.Errorf("unexpected path: %s", r.URL.Path)
				http.NotFound(w, r)
			}
		}))
	}
	a, b := server("a"), server("b")
	defer a.Close()
	defer b.Close()
	c := &Client{Root: t.TempDir()}
	if err := c.UpdateConnections(ctx, func(p *Connections) error {
		p.Default = "a"
		p.Servers = map[string]Profile{"a": {URL: a.URL}, "b": {URL: b.URL}}
		p.Bindings[root] = Binding{Server: "a", Projects: map[string]string{"ui": "b"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	connection, err := c.ConnectionFor(ctx, root, filepath.Join(root, "apps", "ui"))
	if err != nil || connection.URL != b.URL {
		t.Fatalf("path selector routed incorrectly: %+v %v", connection, err)
	}
	loader := Loader{Client: c}
	for i := 1; i <= 2; i++ {
		values, err := loader.LoadProjects(ctx, root, []string{"services/api", "apps/ui"}, "dev")
		if err != nil {
			t.Fatal(err)
		}
		if values["services/api"]["SOURCE"] != fmt.Sprintf("a-%d", i) || values["apps/ui"]["SOURCE"] != fmt.Sprintf("b-%d", i) {
			t.Fatal(values)
		}
	}
	raw, err := os.ReadFile(filepath.Join(c.Root, "config", "connections.json"))
	if err != nil || strings.Contains(string(raw), "test-token") {
		t.Fatal("credential persisted in bootstrap config")
	}
}
func TestFreshLocalExecutionNeedsNoService(t *testing.T) {
	t.Setenv("ONE_SERVER_URL", "")
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{Version: 2, Workspace: &workspace.ManifestWorkspace{ID: "fresh", Name: "Fresh"}}); err != nil {
		t.Fatal(err)
	}
	c := &Client{Root: t.TempDir(), Binary: "/missing/helper"}
	enabled, err := (Loader{Client: c}).Enabled(context.Background(), root, "")
	if err != nil || enabled {
		t.Fatalf("unconfigured execution needs server: %t %v", enabled, err)
	}
	if err = c.Select(context.Background(), "local", root, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = (Loader{Client: c}).Enabled(context.Background(), root, ""); err == nil {
		t.Fatal("configured service failed silently")
	}
}
func TestRedirectDoesNotForwardCredentials(t *testing.T) {
	reached := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer source.Close()
	err := New().Request(context.Background(), Connection{URL: source.URL, Token: "private"}, "GET", "/server/info", nil, nil)
	if err == nil || reached {
		t.Fatalf("credential redirect: reached=%t error=%v", reached, err)
	}
}
func TestURLPolicy(t *testing.T) {
	for _, value := range []string{"http://one.example", "https://user:pass@one.example", "https://one.example/path", "https://one.example?token=x", "file:///etc/passwd"} {
		if _, err := NormalizeURL(value); err == nil {
			t.Errorf("accepted %s", value)
		}
	}
}
