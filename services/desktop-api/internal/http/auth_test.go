package serve

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/desktopcontrol"
)

func TestElectronDesktopAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name, method, origin, token, generation string
		want                                    int
		called                                  bool
	}{
		{"read", "GET", "one-desktop://app", "secret", "generation", 200, true},
		{"write", "POST", "one-desktop://app", "secret", "generation", 200, true},
		{"preflight", "OPTIONS", "one-desktop://app", "", "", 204, false},
		{"missing token", "POST", "one-desktop://app", "", "generation", 401, false},
		{"stale generation", "POST", "one-desktop://app", "secret", "old", 401, false},
		{"website", "POST", "https://example.com", "secret", "generation", 403, false},
		{"opaque origin", "POST", "null", "secret", "generation", 403, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := &desktopcontrol.Session{Bootstrap: desktopcontrol.Bootstrap{
				Origin: "one-desktop://app", Token: "secret", Generation: "generation",
			}}
			called := false
			handler := desktopAuth(MuxOpts{Session: session})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(200)
			}))
			request := httptest.NewRequest(tc.method, "http://127.0.0.1:1234/api/preferences", nil)
			request.Header.Set("Origin", tc.origin)
			request.Header.Set("Authorization", "Bearer "+tc.token)
			request.Header.Set("X-One-Backend-Generation", tc.generation)
			if tc.method == "OPTIONS" {
				request.Header.Set("Access-Control-Request-Method", "POST")
				request.Header.Set("Access-Control-Request-Headers", "authorization,content-type,x-one-backend-generation")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.want || called != tc.called {
				t.Fatalf("status=%d called=%v; want status=%d called=%v", response.Code, called, tc.want, tc.called)
			}
			if tc.origin == "one-desktop://app" && response.Header().Get("Access-Control-Allow-Origin") != tc.origin {
				t.Fatal("trusted renderer cannot read the response")
			}
		})
	}
}
