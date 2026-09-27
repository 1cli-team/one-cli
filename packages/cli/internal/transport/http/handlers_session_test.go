package serve

import (
	"encoding/json"
	"net/http"
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
