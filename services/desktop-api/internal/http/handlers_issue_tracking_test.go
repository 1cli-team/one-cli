package serve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
)

type linearRouteBackend struct {
	tracking.Backend
	configured bool
}

func (*linearRouteBackend) Status(context.Context) (tracking.ConnectionState, error) {
	return tracking.ConnectionState{Connections: []tracking.Connection{}}, nil
}
func (b *linearRouteBackend) Configure(_ context.Context, input tracking.OAuthConfig) error {
	if input.ClientID == "invalid" {
		return tracking.Fail("config")
	}
	b.configured = true
	return nil
}

func TestLinearRoutesValidationAndAvailability(t *testing.T) {
	backend := &linearRouteBackend{}
	mux := http.NewServeMux()
	registerIssueTrackingRoutes(mux, MuxOpts{IssueTracking: tracking.New(backend, nil)})
	for _, test := range []struct {
		method, path, body string
		want               int
	}{
		{http.MethodGet, "/linear/connections", "", 200},
		{http.MethodPut, "/linear/config", `{"clientId":"public","redirectUri":"http://127.0.0.1:39127/oauth/linear/callback"}`, 200},
		{http.MethodPut, "/linear/config", `{"clientId":"invalid"}`, 400},
		{http.MethodPut, "/linear/config", `not-json`, 400},
		{http.MethodPost, "/issue-tracking/projects?kind=workspace&scope=test", `not-json`, 400},
	} {
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Errorf("%s %s: %d %s", test.method, test.path, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Error("Linear response can be cached")
		}
	}
	if !backend.configured {
		t.Fatal("configuration route was not called")
	}
	mux = http.NewServeMux()
	registerIssueTrackingRoutes(mux, MuxOpts{})
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/linear/connections", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing plugin returned %d", response.Code)
	}
}
