package serve

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
)

func TestCodeHostSettingsUseLocalOneWithNoGateway(t *testing.T) {
	const token = "fixture-gitea-token"
	local := t.TempDir()
	t.Setenv("ONE_HOME", local)
	gitea := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user" || r.Header.Get("Authorization") != "token "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"login": "local-one"})
	}))
	defer gitea.Close()
	mux := http.NewServeMux()
	registerCodeHostRoutes(mux)
	request := func(method, path string, body any) {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(data)))
		if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" ||
			strings.Contains(rec.Body.String(), token) {
			t.Fatalf("%s %s returned %d", method, path, rec.Code)
		}
	}
	request(http.MethodGet, "/code-hosts", nil)
	request(http.MethodPut, "/code-hosts", codehost.Input{URL: gitea.URL, Token: token})
	request(http.MethodPost, "/code-hosts/127.0.0.1/test", nil)

	hosts, err := (codehost.Service{}).List()
	if err != nil || len(hosts) != 2 || hosts[1].Account != "local-one" {
		t.Fatalf("local One host configuration: %+v, %v", hosts, err)
	}
	if _, err := os.Stat(filepath.Join(local, "config", "gateway.json")); !os.IsNotExist(err) {
		t.Fatal("test unexpectedly created a Gateway connection")
	}
}
