package infisical

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/zalando/go-keyring"
)

func TestFetchProjectEnvironmentsStayIsolated(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	folders := map[string]map[string]string{
		"/":                {"SHARED": "root", "OVERRIDE": "root"},
		"/services":        {"PARENT": "services", "OVERRIDE": "parent"},
		"/services/api":    {"API_ONLY": "api", "OVERRIDE": "api"},
		"/services/worker": {"WORKER_ONLY": "worker", "OVERRIDE": "worker"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/v3/secrets/raw" || q.Get("workspaceId") != "remote" || q.Get("recursive") != "true" || q.Get("secretPath") != "/" {
			t.Errorf("unexpected request %s", r.URL)
		}
		env := q.Get("environment")
		if env != "dev" && env != "staging" {
			t.Errorf("unexpected environment %q", env)
		}
		values := []map[string]string{}
		for path, folder := range folders {
			for k, v := range folder {
				values = append(values, map[string]string{"secretPath": path, "secretKey": k, "secretValue": env + ":" + v})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"secrets": values})
	}))
	defer server.Close()
	sharedTestSession(t, server.URL, "user")
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{Version: 2, Env: &workspace.EnvironmentConfig{SiteURL: server.URL, ProjectID: "remote", Environments: []string{"staging", "dev"}}, Projects: []workspace.ManifestProject{
		{Name: "api", RelativeDir: "services/api", Toolchain: "go"}, {Name: "worker", RelativeDir: "services/worker", Toolchain: "go"},
	}}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(workspace.ManifestPath(root))
	var wg sync.WaitGroup
	for _, project := range []string{"api", "worker"} {
		for _, requested := range []string{"", "staging"} {
			wg.Go(func() {
				got, err := FetchSecretsForSubproject(context.Background(), root, "services/"+project, requested)
				if err != nil {
					t.Error(err)
					return
				}
				env := requested
				if env == "" {
					env = "dev"
				}
				want := map[string]string{"SHARED": env + ":root", "PARENT": env + ":services", "OVERRIDE": env + ":" + project}
				key := "API_ONLY"
				if project == "worker" {
					key = "WORKER_ONLY"
				}
				want[key] = env + ":" + project
				if !reflect.DeepEqual(got, want) {
					t.Error(fmt.Sprintf("%s/%s received wrong scope: %v", project, env, got))
				}
			})
		}
	}
	wg.Wait()
	after, _ := os.ReadFile(workspace.ManifestPath(root))
	if !bytes.Equal(before, after) {
		t.Fatal("fetch changed manifest")
	}
}
