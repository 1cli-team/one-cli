package openviking

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
)

func TestImportDocumentTracksIndexingWithinWorkspace(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	const (
		root      = "viking://resources/example"
		sourceURI = root + "/guide"
		fileName  = "发布流程.md"
		contents  = "# 发布流程"
		token     = "local-test-token"
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		var result any
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"account_id": "team", "user_id": "alice"})
			return
		case "/api/v1/fs/stat":
			uri := r.URL.Query().Get("uri")
			result = resourceStat{URI: uri, IsDir: true}
		case "/api/v1/fs/ls":
			result = []map[string]any{
				{"name": "example", "uri": root, "isDir": true, "access": "write"},
				{"name": "restricted", "uri": "viking://resources/restricted", "isDir": true, "access": "denied"},
				{"name": "readme", "uri": "viking://resources/readme", "isDir": false},
			}
		case "/api/v1/resources/temp_upload":
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil || header.Filename != fileName || string(data) != contents {
				t.Errorf("wrong upload filename or content: %q, %v", header.Filename, err)
			}
			result = map[string]string{"temp_file_id": "tmp-file"}
		case "/api/v1/resources":
			var input map[string]any
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil ||
				input["temp_file_id"] != "tmp-file" || input["parent"] != root || input["source_name"] != fileName {
				t.Errorf("unexpected import request: %+v, %v", input, err)
			}
			result = map[string]string{"task_id": "new-task", "root_uri": sourceURI, "status": "success"}
		case "/api/v1/tasks/new-task":
			result = map[string]string{"task_id": "new-task", "task_type": "add_resource", "resource_id": sourceURI, "status": "completed"}
		case "/api/v1/tasks/failed-task":
			result = map[string]string{"task_id": "failed-task", "task_type": "add_resource", "resource_id": sourceURI, "status": "failed", "error": "index error " + token}
		default:
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "result": result})
	}))
	defer upstream.Close()

	plugin, ctx := New(), t.Context()
	input := knowledge.ConnectionInput{URL: upstream.URL, APIKey: new(token)}
	if _, err := plugin.Configure(ctx, input); err != nil {
		t.Fatal(err)
	}
	folders, err := plugin.Browse(ctx, "")
	if err != nil || folders.Current != "viking://resources" || len(folders.Entries) != 1 || folders.Entries[0].URI != root {
		t.Fatalf("unexpected directory choices: %+v, %v", folders, err)
	}
	if _, err := plugin.Bind(ctx, "workspace-a", root); err != nil {
		t.Fatal(err)
	}
	if saved, err := plugin.Check(ctx, "workspace-a"); err != nil || saved.URI != root {
		t.Fatalf("saved binding did not verify: %+v, %v", saved, err)
	}
	if _, err := plugin.Import(ctx, "workspace-a", "../"+fileName, strings.NewReader(contents)); err == nil {
		t.Fatal("import accepted a filename containing a path")
	}
	submitted, err := plugin.Import(ctx, "workspace-a", fileName, strings.NewReader(contents))
	if err != nil || submitted.TaskID != "new-task" || submitted.URI != sourceURI || submitted.Status != "pending" {
		t.Fatalf("unexpected submission: %+v, %v", submitted, err)
	}
	if completed, err := plugin.ImportStatus(ctx, "workspace-a", submitted.TaskID); err != nil || completed.Status != "completed" || completed.URI != sourceURI {
		t.Fatalf("unexpected completion: %+v, %v", completed, err)
	}
	if failed, err := plugin.ImportStatus(ctx, "workspace-a", "failed-task"); err != nil ||
		failed.Status != "failed" || failed.Error != "index error [redacted]" {
		t.Fatalf("unexpected failure: %+v, %v", failed, err)
	}
	if _, err := plugin.Bind(ctx, "workspace-b", root+"-other"); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.ImportStatus(ctx, "workspace-b", submitted.TaskID); err == nil {
		t.Fatal("another workspace could inspect the import")
	}
}
