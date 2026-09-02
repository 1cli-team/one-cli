package serve

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
)

func TestConfigureProfileCRUDRejectsUnsafeNamesWithBadRequest(t *testing.T) {
	srv, _ := newTestServer(t)
	outsidePath := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "sentinel.json")
	const sentinel = "do-not-touch"
	if err := os.WriteFile(outsidePath, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "upsert",
			method: http.MethodPost,
			path:   "/api/configure/env/infisical",
			body:   `{"name":"../../../../sentinel","profile":{"siteUrl":"https://x","credentials":{"clientId":"c","clientSecret":"s"}}}`,
		},
		{
			name:   "set default",
			method: http.MethodPut,
			path:   "/api/configure/env/infisical/default",
			body:   `{"name":"../../../../sentinel"}`,
		},
		{
			name:   "remove",
			method: http.MethodDelete,
			path:   "/api/configure/env/infisical/..%2F..%2F..%2F..%2Fsentinel",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var body *strings.Reader
			if test.body != "" {
				body = strings.NewReader(test.body)
			} else {
				body = strings.NewReader("")
			}
			res, raw := apiRequest(t, srv, test.method, test.path, body)
			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; response: %s", res.StatusCode, raw)
			}
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(raw, &envelope); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if envelope.Error.Code != "PROFILE_BACKEND_INVALID" {
				t.Fatalf("code = %q, want PROFILE_BACKEND_INVALID", envelope.Error.Code)
			}
		})
	}

	configPath, err := profile.ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	credentialsPath, err := profile.CredentialsPath()
	if err != nil {
		t.Fatalf("CredentialsPath: %v", err)
	}
	for _, path := range []string{configPath, credentialsPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("unsafe HTTP mutation created %s; stat error = %v", path, err)
		}
	}

	// Also pin the exact outside path a vulnerable cache filename would reach.
	got, err := os.ReadFile(outsidePath)
	if err != nil {
		t.Fatalf("outside sentinel was removed: %v", err)
	}
	if string(got) != sentinel {
		t.Errorf("outside sentinel changed: got %q, want %q", got, sentinel)
	}
}
