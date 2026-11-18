package serve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
)

func TestSyncOpenVikingUsesSavedConnection(t *testing.T) {
	const worker = "w-aaaaaaaaaaaaaaaaaaaaaaaaaa"
	const secret = "fixture-openviking-key"
	const url = "https://openviking.example"
	var gatewayRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gatewayRequests.Add(1)
		var input gateway.SetupInput
		if r.Method != http.MethodPost || r.URL.Path != "/v1/workers/"+worker+"/setup" ||
			json.NewDecoder(r.Body).Decode(&input) != nil || input.Operation != gateway.SetupOpenViking ||
			input.OpenViking == nil || input.OpenViking.URL != url || input.OpenViking.APIKey == nil || *input.OpenViking.APIKey != secret {
			t.Error("Gateway did not receive the saved connection on the selected worker")
		}
		_ = json.NewEncoder(w).Encode(knowledge.Connection{URL: url, HasAPIKey: true})
	}))
	defer server.Close()
	mux := http.NewServeMux()
	registerGatewayOpenVikingSyncRoute(mux, func(context.Context) (knowledge.ConnectionInput, error) {
		return knowledge.ConnectionInput{URL: url, APIKey: new(secret)}, nil
	}, func() (gateway.Connection, error) {
		return gateway.Connection{URL: server.URL, Token: strings.Repeat("a", 32)}, nil
	})
	call := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/gateway/workers/"+worker+"/openviking", strings.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	body := `{"gatewayUrl":"` + server.URL + `"`
	if got := call(body + `,"apiKey":"injected"}`); got.Code != http.StatusBadRequest || gatewayRequests.Load() != 0 {
		t.Fatal("renderer-supplied credentials were accepted")
	}
	if got := call(`{"gatewayUrl":"https://another-gateway.example"}`); got.Code != http.StatusConflict || gatewayRequests.Load() != 0 {
		t.Fatal("sync used a different Gateway from the one the user selected")
	}
	got := call(body + `}`)
	if got.Code != http.StatusOK || gatewayRequests.Load() != 1 || strings.Contains(got.Body.String(), secret) ||
		!strings.Contains(got.Body.String(), url) || got.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected sync response: HTTP %d", got.Code)
	}
}
