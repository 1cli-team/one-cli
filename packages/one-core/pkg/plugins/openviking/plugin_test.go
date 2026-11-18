package openviking

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
)

func TestBindCreatesMissingDirectory(t *testing.T) {
	const uri = "viking://resources/drama"
	for _, test := range []struct {
		name        string
		statStatus  int
		statCode    string
		mkdirStatus int
		isFile      bool
		wantError   cliErrors.Code
	}{
		{name: "existing directory", statStatus: http.StatusOK},
		{name: "existing file", statStatus: http.StatusOK, isFile: true, wantError: cliErrors.OPENVIKING_INVALID_INPUT},
		{name: "missing directory", statStatus: http.StatusNotFound, statCode: "NOT_FOUND", mkdirStatus: http.StatusOK},
		{name: "concurrent creation", statStatus: http.StatusNotFound, statCode: "NOT_FOUND", mkdirStatus: http.StatusConflict},
		{name: "missing API endpoint", statStatus: http.StatusNotFound, wantError: cliErrors.OPENVIKING_NOT_FOUND},
		{name: "stat forbidden", statStatus: http.StatusForbidden, wantError: cliErrors.OPENVIKING_ACCESS_DENIED},
		{name: "mkdir forbidden", statStatus: http.StatusNotFound, statCode: "NOT_FOUND", mkdirStatus: http.StatusForbidden, wantError: cliErrors.OPENVIKING_ACCESS_DENIED},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ONE_HOME", t.TempDir())
			var mkdirCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var result any
				switch r.Method + " " + r.URL.Path {
				case "GET /health":
					http.NotFound(w, r)
					return
				case "GET /api/v1/fs/ls":
					result = []any{}
				case "GET /api/v1/fs/stat":
					if r.URL.Query().Get("uri") != uri {
						t.Errorf("unexpected stat URI: %q", r.URL.Query().Get("uri"))
					}
					if test.statStatus != http.StatusOK && mkdirCalls.Load() == 0 {
						w.WriteHeader(test.statStatus)
						if test.statCode != "" {
							_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": test.statCode}})
						}
						return
					}
					result = resourceStat{URI: uri, IsDir: !test.isFile}
				case "POST /api/v1/fs/mkdir":
					mkdirCalls.Add(1)
					var input struct{ URI string }
					if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.URI != uri {
						t.Errorf("unexpected mkdir request: %+v, %v", input, err)
					}
					if test.mkdirStatus != http.StatusOK {
						w.WriteHeader(test.mkdirStatus)
						return
					}
				default:
					t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "result": result})
			}))
			defer server.Close()

			plugin := New()
			ctx := t.Context()
			if _, err := plugin.Configure(ctx, knowledge.ConnectionInput{URL: server.URL}); err != nil {
				t.Fatal(err)
			}
			binding, err := plugin.Bind(ctx, "workspace", uri)
			if test.wantError == "" {
				if err != nil || binding.URI != uri {
					t.Fatalf("Bind = %+v, %v", binding, err)
				}
			} else {
				var problem *output.Error
				if !errors.As(err, &problem) || problem.Code != string(test.wantError) {
					t.Fatalf("Bind error = %v, want %s", err, test.wantError)
				}
			}
			status, err := plugin.Status(ctx, "workspace")
			if err != nil || (status.Binding != nil) != (test.wantError == "") {
				t.Fatalf("Status = %+v, %v", status, err)
			}
			var wantCalls int32
			if test.mkdirStatus != 0 {
				wantCalls = 1
			}
			if calls := mkdirCalls.Load(); calls != wantCalls {
				t.Fatalf("Mkdir called %d times, want %d", calls, wantCalls)
			}
		})
	}
}

func TestWorkspaceBindingScopesSearchAndRead(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	const (
		root    = "viking://resources/demo"
		file    = root + "/readme.md"
		sibling = "viking://resources/demo-other/secret.md"
		token   = "test-api-key"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unexpected credentials", http.StatusUnauthorized)
			return
		}
		var result any
		switch r.URL.Path {
		case "/openviking/api/v1/fs/ls":
			result = []any{}
		case "/openviking/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"account_id": "team", "user_id": "alice"})
			return
		case "/openviking/api/v1/fs/stat":
			uri := r.URL.Query().Get("uri")
			switch uri {
			case root, file:
				result = resourceStat{URI: uri, IsDir: uri == root}
			default:
				t.Errorf("unexpected stat URI %q", uri)
				http.NotFound(w, r)
				return
			}
		case "/openviking/api/v1/search/find":
			var input struct {
				Query     string `json:"query"`
				TargetURI string `json:"target_uri"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Query != "auth" || input.TargetURI != root {
				t.Errorf("wrong search scope or query: %+v, %v", input, err)
			}
			result = map[string]any{"resources": []knowledge.Match{{URI: file, Abstract: "Auth"}, {URI: sibling}}}
		case "/openviking/api/v1/content/read":
			if uri := r.URL.Query().Get("uri"); uri != file {
				t.Errorf("unexpected read URI %q", uri)
			}
			result = "Authentication uses an API key."
		case "/openviking/api/v1/content/overview":
			if uri := r.URL.Query().Get("uri"); uri != root {
				t.Errorf("unexpected overview URI %q", uri)
			}
			result = "Project docs."
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "result": result})
	}))
	defer server.Close()

	ctx := t.Context()
	plugin := New()
	input := knowledge.ConnectionInput{URL: server.URL + "/openviking/", APIKey: new(token)}
	if _, err := plugin.Configure(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.Bind(ctx, "workspace-a", root); err != nil {
		t.Fatal(err)
	}
	status, err := New().Status(ctx, "workspace-a")
	if err != nil || !status.Configured || status.Binding == nil || status.Binding.URI != root {
		t.Fatalf("unexpected saved binding: %+v, %v", status, err)
	}
	configured, err := plugin.Connection(ctx)
	if err != nil || configured == nil || configured.URL != server.URL+"/openviking" {
		t.Fatalf("unexpected global connection: %+v, %v", configured, err)
	}
	encoded, err := json.Marshal(configured)
	if err != nil || strings.Contains(string(encoded), token) {
		t.Fatal("public connection includes credentials")
	}
	if other, err := plugin.Status(ctx, "workspace-b"); err != nil || !other.Configured || other.Binding != nil {
		t.Fatalf("unbound workspace received a binding: %+v, %v", other, err)
	}
	matches, err := plugin.Find(ctx, "workspace-a", "auth")
	if err != nil || len(matches) != 1 || matches[0].URI != file {
		t.Fatalf("unexpected search matches: %+v, %v", matches, err)
	}
	for _, bad := range []string{sibling, root + "/%2e%2e/demo-other/secret.md", root + "/%2Fetc"} {
		if _, err := plugin.Read(ctx, "workspace-a", bad); err == nil {
			t.Fatalf("accepted an out-of-scope read: %s", bad)
		}
	}
	content, err := plugin.Read(ctx, "workspace-a", file)
	if err != nil || content.IsDirectory || content.Text != "Authentication uses an API key." {
		t.Fatalf("unexpected file content: %+v, %v", content, err)
	}
	directory, err := plugin.Read(ctx, "workspace-a", root)
	if err != nil || !directory.IsDirectory || directory.Text != "Project docs." {
		t.Fatalf("unexpected directory overview: %+v, %v", directory, err)
	}
	input.APIKey = nil
	if _, err := plugin.Configure(ctx, input); err != nil {
		t.Fatalf("same server should reuse its saved key: %v", err)
	}
	if err := plugin.Unbind(ctx, "workspace-a"); err != nil {
		t.Fatal(err)
	}
	if binding, err := plugin.Status(ctx, "workspace-a"); err != nil || binding.Binding != nil {
		t.Fatalf("binding was not removed: %+v, %v", binding, err)
	}
}
