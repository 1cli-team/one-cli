package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/desktopcontrol"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	session "github.com/torchstellar-team/one-cli/services/desktop-api/internal/providers/infisicalsession"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	root := t.TempDir()
	db, err := store.Open(context.Background(), store.Config{SQLite: filepath.Join(root, "one.db"), Key: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return BuildMux(MuxOpts{Store: db, Manager: &session.Manager{Store: db, Root: root}, ServerID: "test", PublicOrigin: "https://one.example", Session: &desktopcontrol.Session{Bootstrap: desktopcontrol.Bootstrap{Token: strings.Repeat("a", 64), Generation: "generation"}}})
}
func call(h http.Handler, method, path, body, token, origin, language string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	req.Header.Set("Accept-Language", language)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}
func TestManagementAPIHasNoFilesystemDependency(t *testing.T) {
	h := testHandler(t)
	token := strings.Repeat("a", 64)
	body := `{"manifest":{"version":2,"workspace":{"id":"detached","name":"No checkout"},"projects":[{"name":"api","relativeDir":"services/api","toolchain":"go"}]},"sourceRevision":"abc"}`
	if r := call(h, "POST", "/api/v1/workspaces", body, token, "", ""); r.Code != 201 {
		t.Fatalf("register: %d %s", r.Code, r.Body.String())
	}
	for _, path := range []string{"/workspaces", "/workspaces/detached/overview", "/workspaces/detached/environment", "/workspaces/detached/projects/api", "/workspaces/detached/snapshot", "/server/export"} {
		r := call(h, "GET", "/api/v1"+path, "", token, "", "")
		if r.Code != 200 {
			t.Fatalf("%s: %d %s", path, r.Code, r.Body.String())
		}
		if strings.Contains(r.Body.String(), t.TempDir()) {
			t.Fatal("local path leaked")
		}
	}
	if r := call(h, "POST", "/api/v1/workspaces/observe", `{"path":"/etc"}`, token, "", ""); r.Code < 400 {
		t.Fatalf("accepted filesystem operation: %d", r.Code)
	}
	if r := call(h, "POST", "/api/v1/workspaces/detached/projects", `{"name":"bad"}`, token, "", ""); r.Code < 400 {
		t.Fatalf("accepted execution: %d", r.Code)
	}
}
func TestAuthenticationAndRequestLocale(t *testing.T) {
	h := testHandler(t)
	token := strings.Repeat("a", 64)
	for _, locale := range []string{"zh-CN", "en-US"} {
		r := call(h, "GET", "/api/v1/server/info", "", "", "", locale)
		if r.Code != 401 || !strings.Contains(r.Body.String(), i18n.ForContext(i18n.WithLocale(context.Background(), locale)).T("server.authentication_failed")) {
			t.Fatalf("locale %s: %d %s", locale, r.Code, r.Body.String())
		}
	}
	if r := call(h, "GET", "/api/v1/server/info", "", token, "https://evil.example", ""); r.Code != 403 {
		t.Fatal(r.Code)
	}
	login := call(h, "POST", "/api/v1/auth/session", `{"token":"`+token+`"}`, "", "https://one.example", "")
	if login.Code != 200 {
		t.Fatalf("login: %d %s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Fatal("session cookie flags")
	}
	request := httptest.NewRequest("PUT", "/api/v1/preferences", bytes.NewBufferString(`{"locale":"zh-CN"}`))
	request.AddCookie(cookies[0])
	res := httptest.NewRecorder()
	h.ServeHTTP(res, request)
	if res.Code != 403 {
		t.Fatalf("CSRF accepted: %d", res.Code)
	}
	request = httptest.NewRequest("PUT", "/api/v1/preferences", bytes.NewBufferString(`{"locale":"zh-CN"}`))
	request.AddCookie(cookies[0])
	request.Header.Set("Origin", "https://one.example")
	res = httptest.NewRecorder()
	h.ServeHTTP(res, request)
	if res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	var p map[string]any
	json.Unmarshal(res.Body.Bytes(), &p)
	if p["stored_locale"] != "zh-CN" {
		t.Fatal(p)
	}
	request = httptest.NewRequest("DELETE", "/api/v1/auth/session", nil)
	request.AddCookie(cookies[0])
	request.Header.Set("Origin", "https://one.example")
	res = httptest.NewRecorder()
	h.ServeHTTP(res, request)
	if res.Code != 204 {
		t.Fatal(res.Code)
	}
	request = httptest.NewRequest("GET", "/api/v1/server/info", nil)
	request.AddCookie(cookies[0])
	res = httptest.NewRecorder()
	h.ServeHTTP(res, request)
	if res.Code != 401 {
		t.Fatal("revoked session accepted")
	}
}
