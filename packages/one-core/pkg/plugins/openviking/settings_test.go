package openviking

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
)

func TestSharedConnectionPreservesWorkspaceBindingAcrossKeyRotation(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	const originalKey = "original-test-key"
	const rotatedKey = "rotated-test-key"
	const otherKey = "other-test-key"
	const uri = "viking://resources/team"
	const workspace = "workspace-a"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account := "team"
		switch r.Header.Get("Authorization") {
		case "Bearer " + originalKey, "Bearer " + rotatedKey:
		case "Bearer " + otherKey:
			account = "another-team"
		default:
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var result any
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"account_id": account, "user_id": "alice"})
			return
		case "/api/v1/fs/ls":
			result = []any{}
		case "/api/v1/fs/stat":
			result = resourceStat{URI: r.URL.Query().Get("uri"), IsDir: true}
		default:
			t.Errorf("unexpected upstream request: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "result": result})
	}))
	defer server.Close()

	dir, err := configDirectory()
	if err != nil {
		t.Fatal(err)
	}
	bindingFile := bindingPath(dir, workspace)
	if err := writeConfig(bindingFile, workspaceBinding{URI: uri, URL: server.URL, APIKey: originalKey}); err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	plugin := New()
	checkStatus := func(needsRebind bool) {
		t.Helper()
		status, err := plugin.Status(ctx, workspace)
		if err != nil || !status.Configured || status.NeedsRebind != needsRebind || status.Binding == nil || status.Binding.URI != uri {
			t.Fatalf("unexpected workspace status: %+v, %v", status, err)
		}
	}
	checkStatus(false)
	migrated, err := readConfig[workspaceBinding](bindingFile)
	if err != nil || migrated.URI != uri || migrated.Scope == "" || migrated.URL != "" || migrated.APIKey != "" {
		t.Fatal("workspace binding was not migrated to a directory and scope")
	}
	if _, err := plugin.Configure(ctx, knowledge.ConnectionInput{URL: server.URL}); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.Configure(ctx, knowledge.ConnectionInput{URL: server.URL, APIKey: new(rotatedKey)}); err != nil {
		t.Fatal(err)
	}
	checkStatus(false)

	other := knowledge.ConnectionInput{URL: server.URL, APIKey: new(otherKey)}
	checked, err := plugin.TestConnection(ctx, other)
	if err != nil || checked.Account != "another-team" {
		t.Fatalf("unexpected connection test result: %+v, %v", checked, err)
	}
	if _, err := plugin.Configure(ctx, knowledge.ConnectionInput{URL: server.URL, APIKey: new("invalid-test-key")}); err == nil {
		t.Fatal("saved a connection that failed authentication")
	}
	saved, err := readConfig[connection](connectionPath(dir))
	if err != nil || saved.APIKey != rotatedKey || saved.Account != "team" {
		t.Fatal("a connection test or failed save changed the saved connection")
	}
	checkStatus(false)

	if _, err := plugin.Configure(ctx, other); err != nil {
		t.Fatal(err)
	}
	checkStatus(true)
	if _, err := plugin.Find(ctx, workspace, "test"); err == nil {
		t.Fatal("searched a workspace bound to another account")
	}
	if _, err := plugin.Bind(ctx, workspace, uri); err != nil {
		t.Fatal(err)
	}
	checkStatus(false)
}
