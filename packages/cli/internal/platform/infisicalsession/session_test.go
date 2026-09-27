package infisicalsession

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func isolate(t *testing.T) { t.Helper(); keyring.MockInit(); t.Setenv("XDG_CONFIG_HOME", t.TempDir()) }
func token(t *testing.T) string {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"userId": "user-1", "organizationId": "org-1", "exp": time.Now().Add(time.Hour).Unix()})
	return "header." + base64.RawURLEncoding.EncodeToString(b) + ".signature"
}
func TestBrowserLoginChecksOriginAndStoresOnlyInKeyring(t *testing.T) {
	isolate(t)
	tok := token(t)
	instance := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/checkAuth" || r.Header.Get("Authorization") != "Bearer "+tok {
			http.Error(w, "invalid", 401)
			return
		}
		w.WriteHeader(200)
	}))
	defer instance.Close()
	a, e := Start(context.Background(), instance.URL)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Cancel()
	u, _ := url.Parse(a.URL)
	callback := "http://127.0.0.1:" + u.Query().Get("callback_port")
	body, _ := json.Marshal(map[string]string{"email": "user@example.com", "JTWToken": tok})
	post := func(origin string) int {
		t.Helper()
		r, _ := http.NewRequest(http.MethodPost, callback, strings.NewReader(string(body)))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		resp, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	if got := post("https://attacker.example"); got != 403 {
		t.Fatalf("wrong origin: %d", got)
	}
	if _, e := Require(); e == nil {
		t.Fatal("invalid origin saved a session")
	}
	if _, e := Start(context.Background(), instance.URL); e == nil {
		t.Fatal("allowed concurrent login")
	}
	if got := post(instance.URL); got != 200 {
		t.Fatalf("callback: %d", got)
	}
	info, e := a.Wait()
	if e != nil {
		t.Fatal(e)
	}
	if !info.LoggedIn || info.UserID != "user-1" || info.OrganizationID != "org-1" {
		t.Fatalf("info: %#v", info)
	}
	raw, _ := json.Marshal(info)
	if strings.Contains(string(raw), tok) {
		t.Fatal("public status leaked token")
	}
	s, e := Require()
	if e != nil || s.Token != tok {
		t.Fatalf("session missing: %v", e)
	}
	if _, e := Start(context.Background(), instance.URL); e == nil {
		t.Fatal("logged in account replaced without logout")
	}
	if e := Logout(); e != nil {
		t.Fatal(e)
	}
	if _, e := Require(); e == nil {
		t.Fatal("logout left usable token")
	}
}
func TestCancelledLoginCannotSaveSession(t *testing.T) {
	isolate(t)
	a, e := Start(context.Background(), DefaultSiteURL)
	if e != nil {
		t.Fatal(e)
	}
	a.Cancel()
	if _, e := a.Wait(); e == nil {
		t.Fatal("cancel succeeded")
	}
	if _, e := Require(); e == nil {
		t.Fatal("cancel created session")
	}
}
func TestExpiredSessionFailsClosed(t *testing.T) {
	isolate(t)
	if e := save(&Session{Info: Info{ExpiresAt: time.Now().Add(-time.Minute)}, Token: "old"}); e != nil {
		t.Fatal(e)
	}
	if _, e := Require(); e == nil {
		t.Fatal("expired session accepted")
	}
	info, e := Status()
	if e != nil || !info.Expired || info.LoggedIn {
		t.Fatalf("status: %#v %v", info, e)
	}
}
func TestRequestDoesNotLeakUpstreamBodiesOrFollowRedirects(t *testing.T) {
	var called bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer target.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, target.URL, 302)
			return
		}
		http.Error(w, "sensitive-value", 500)
	}))
	defer upstream.Close()
	s := &Session{Info: Info{SiteURL: upstream.URL}, Token: "token"}
	for _, path := range []string{"/redirect", "/error"} {
		e := Request(context.Background(), s, "GET", path, nil, nil)
		if e == nil || strings.Contains(e.Error(), "sensitive-value") {
			t.Fatalf("unsafe error %v", e)
		}
	}
	if called {
		t.Fatal("followed redirect")
	}
}
func TestNormalizeSite(t *testing.T) {
	for _, s := range []string{"http://example.com", "https://u:p@example.com", "https://example.com/path", "https://example.com?token=x", "file:///tmp/login"} {
		if _, e := NormalizeSite(s); e == nil {
			t.Errorf("accepted %s", s)
		}
	}
	if _, e := NormalizeSite("http://127.0.0.1:8000"); e != nil {
		t.Fatal(e)
	}
}

func TestLogoutInvalidatesCallbackBeingVerified(t *testing.T) {
	isolate(t)
	entered, release := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(200)
	}))
	defer upstream.Close()
	attempt, err := Start(context.Background(), upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer attempt.Cancel()
	u, _ := url.Parse(attempt.URL)
	body, _ := json.Marshal(map[string]string{"JTWToken": token(t), "email": "user@example.com"})
	callbackDone := make(chan struct{})
	go func() {
		defer close(callbackDone)
		req, _ := http.NewRequest("POST", "http://127.0.0.1:"+u.Query().Get("callback_port"), strings.NewReader(string(body)))
		req.Header.Set("Origin", upstream.URL)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()
	<-entered
	err = Logout()
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	<-callbackDone
	if _, err = attempt.Wait(); err == nil {
		t.Fatal("logout did not invalidate pending login")
	}
	if _, err = Require(); err == nil {
		t.Fatal("late callback recreated the session")
	}
}
