package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

func TestGatewaySettingsAuthenticateAndKeepSavedTokenPrivate(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	token := strings.Repeat("a", 40)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/status" {
			t.Errorf("unexpected Gateway path %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, gateway.Snapshot{
			Schema: gateway.SnapshotSchema, StartedAt: time.Now().UTC(),
			Jobs: gateway.QueueCounts{Queued: 3},
		})
	}))
	defer server.Close()

	mux := http.NewServeMux()
	registerGatewaySettingsRoutes(mux)
	call := func(method, path, url, auth string, wantCode int) gatewayStatus {
		t.Helper()
		payload, err := json.Marshal(gatewayConnection{URL: url, Token: auth})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(method, path, strings.NewReader(string(payload)))
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != wantCode {
			t.Fatalf("%s %s = %d, want %d", method, path, response.Code, wantCode)
		}
		if strings.Contains(response.Body.String(), token) {
			t.Fatalf("%s %s exposed the saved token", method, path)
		}
		var result gatewayStatus
		if wantCode == http.StatusOK {
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
		}
		return result
	}

	get := func() gatewayStatus { return call(http.MethodGet, "/gateway/status", "", "", 200) }
	if got := get(); got.State != "unconfigured" {
		t.Fatalf("initial connection: %+v", got)
	}
	testPath := "/gateway/connection/test"
	if got := call(http.MethodPost, testPath, server.URL, strings.Repeat("x", 40), 200); got.State != "unauthorized" {
		t.Fatalf("incorrect token result: %+v", got)
	}
	if got := call(http.MethodPost, testPath, server.URL, token, 200); got.State != "connected" {
		t.Fatalf("connection test: %+v", got)
	}
	if got := get(); got.State != "unconfigured" {
		t.Fatalf("testing changed the saved connection: %+v", got)
	}

	connectionPath := "/gateway/connection"
	got := call(http.MethodPut, connectionPath, server.URL, token, 200)
	if got.State != "connected" || !got.HasToken || got.Snapshot == nil || got.Snapshot.Jobs.Queued != 3 {
		t.Fatalf("save result: %+v", got)
	}
	path, err := gatewayConnectionPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("connection file permissions = %#o", info.Mode().Perm())
	}
	if got := get(); got.State != "connected" || got.URL != server.URL {
		t.Fatalf("saved connection: %+v", got)
	}
	if got := call(http.MethodPost, testPath, server.URL, "", 200); got.State != "connected" {
		t.Fatalf("existing token did not authenticate the saved endpoint: %+v", got)
	}
	call(http.MethodPut, connectionPath, server.URL+"/changed", "", 400)
	if got := get(); got.State != "connected" || got.URL != server.URL {
		t.Fatalf("invalid update changed the saved connection: %+v", got)
	}
	server.Close()
	if got := get(); got.State != "unreachable" || got.Snapshot != nil {
		t.Fatalf("disconnected Gateway retained a connected snapshot: %+v", got)
	}
}
