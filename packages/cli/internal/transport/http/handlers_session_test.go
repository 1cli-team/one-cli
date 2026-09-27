package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	"github.com/zalando/go-keyring"
)

func TestSessionStatusAndLogoutNeverExposeToken(t *testing.T) {
	srv, _ := newTestServer(t)
	raw, _ := json.Marshal(session.Session{Info: session.Info{UserID: "user", Email: "test@example.com", SiteURL: session.DefaultSiteURL, ExpiresAt: time.Now().Add(time.Hour)}, Token: "private-session-token"})
	if err := keyring.Set("one-cli.infisical", "session", string(raw)); err != nil {
		t.Fatal(err)
	}
	res, body := apiRequest(t, srv, "GET", "/api/session", nil)
	var status struct {
		Session session.Info `json:"session"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || !status.Session.LoggedIn {
		t.Fatalf("status: %d %s", res.StatusCode, body)
	}
	if strings.Contains(string(body), "private-session-token") || strings.Contains(string(body), `"token"`) {
		t.Fatal("public status leaked token")
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("session metadata was cacheable")
	}
	req, _ := http.NewRequest("DELETE", srv.URL+"/api/session", nil)
	req.Header.Set("Origin", "https://untrusted.example")
	rejected, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	rejected.Body.Close()
	if rejected.StatusCode != 403 {
		t.Fatalf("cross-origin logout: %d", rejected.StatusCode)
	}
	if _, err = session.Require(); err != nil {
		t.Fatal("rejected logout changed session")
	}
	res, _ = apiRequest(t, srv, "DELETE", "/api/session", strings.NewReader(""))
	if res.StatusCode != 200 {
		t.Fatalf("logout status: %d", res.StatusCode)
	}
	if _, err = session.Require(); err == nil {
		t.Fatal("logout retained session")
	}
}
func TestRemovedConfigureRoutesUnavailable(t *testing.T) {
	srv, _ := newTestServer(t)
	res, _ := apiRequest(t, srv, "GET", "/api/configure", nil)
	if res.StatusCode != 404 {
		t.Fatalf("removed route status: %d", res.StatusCode)
	}
}

func TestSharedProjectCreationAndDefaultLocationRoutes(t *testing.T) {
	srv, _ := newTestServer(t)
	creates := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/workspace":
			w.Write([]byte(`{"workspaces":[{"id":"shared","name":"shared-credentials","type":"secret-manager","orgId":"org"}]}`))
		case "/api/v2/workspace":
			creates++
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["projectName"] != "Team" || body["type"] != "secret-manager" {
				t.Errorf("unexpected creation: %v", body)
			}
			w.Write([]byte(`{"project":{"id":"shared"}}`))
		case "/api/v1/workspace/shared":
			w.Write([]byte(`{"workspace":{"id":"shared","name":"shared-credentials","type":"secret-manager","orgId":"org","environments":[{"slug":"dev"}]}}`))
		default:
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	raw, _ := json.Marshal(session.Session{Info: session.Info{UserID: "user", OrganizationID: "org", SiteURL: upstream.URL, ExpiresAt: time.Now().Add(time.Hour)}, Token: "private-session-token"})
	if err := keyring.Set("one-cli.infisical", "session", string(raw)); err != nil {
		t.Fatal(err)
	}
	res, body := apiRequest(t, srv, "GET", "/api/global-env/location", nil)
	if res.StatusCode != 200 || !strings.Contains(string(body), `"location": null`) || creates != 0 {
		t.Fatalf("GET changed state: %s", body)
	}
	for _, path := range []string{"/api/infisical/projects", "/api/global-env/location/default"} {
		req, _ := http.NewRequest("POST", srv.URL+path, strings.NewReader(`{"name":"Team"}`))
		req.Header.Set("Origin", "https://untrusted.example")
		rejected, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		rejected.Body.Close()
		if rejected.StatusCode != 403 {
			t.Fatalf("cross-origin mutation: %d", rejected.StatusCode)
		}
	}
	res, body = apiRequest(t, srv, "POST", "/api/infisical/projects", strings.NewReader(`{"name":"Team"}`))
	if res.StatusCode != 201 || creates != 1 || strings.Contains(string(body), "private-session-token") {
		t.Fatalf("create: %d %s", res.StatusCode, body)
	}
	res, body = apiRequest(t, srv, "POST", "/api/global-env/location/default", strings.NewReader(`{}`))
	if res.StatusCode != 200 || creates != 1 || !strings.Contains(string(body), `"defaultEnvironment": "dev"`) {
		t.Fatalf("default: %d %s", res.StatusCode, body)
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("location metadata was cacheable")
	}
}
