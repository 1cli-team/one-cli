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
	const repository = "team/local-one"
	local := t.TempDir()
	t.Setenv("ONE_HOME", local)
	gitea := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/repos/"+repository || r.Header.Get("Authorization") != "token "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"full_name": repository})
	}))
	defer gitea.Close()
	mux := http.NewServeMux()
	registerCodeHostRoutes(mux)
	request := func(method, path string, body any) []byte {
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
		return rec.Body.Bytes()
	}
	request(http.MethodGet, "/code-hosts", nil)
	request(http.MethodPut, "/code-hosts", codehost.Input{URL: gitea.URL, Token: token, Repository: repository})
	var verified codehost.Connection
	if err := json.Unmarshal(request(http.MethodPost, "/code-hosts/127.0.0.1/test", nil), &verified); err != nil ||
		verified.Repository != repository || verified.Account != "" {
		t.Fatalf("verified repository connection: %+v, %v", verified, err)
	}

	hosts, err := (codehost.Service{}).List()
	if err != nil || len(hosts) != 2 || hosts[1] != verified {
		t.Fatalf("local One host configuration: %+v, %v", hosts, err)
	}
	if _, err := os.Stat(filepath.Join(local, "config", "gateway.json")); !os.IsNotExist(err) {
		t.Fatal("test unexpectedly created a Gateway connection")
	}
}
