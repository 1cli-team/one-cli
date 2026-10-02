package http

import (
	"encoding/json"
	"github.com/example/one-template-go-api/internal/config"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStarterRoutes(t *testing.T) {
	router := NewRouter(config.Config{AppName: "starter", AppEnv: "production", AllowedOrigins: []string{"http://localhost:5173"}}, zap.NewNop())
	for _, tt := range []struct {
		method string
		path   string
		status int
	}{{http.MethodGet, "/", 200}, {http.MethodGet, "/health", 200}, {http.MethodPost, "/auth/login", 404}, {http.MethodGet, "/api/users", 404}} {
		t.Run(tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))
			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.status)
			}
			if tt.path == "/health" {
				var body map[string]any
				if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body["status"] != "ok" {
					t.Fatalf("health = %v", body)
				}
			}
		})
	}
	for _, tt := range []struct{ origin, allowed string }{{"http://localhost:5173", "http://localhost:5173"}, {"https://localhost.example.com", ""}} {
		t.Run(tt.origin, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodOptions, "/health", nil)
			request.Header.Set("Origin", tt.origin)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != tt.allowed {
				t.Fatalf("origin = %q, want %q", got, tt.allowed)
			}
		})
	}
}
