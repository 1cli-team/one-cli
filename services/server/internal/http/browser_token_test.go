package serve

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	provider "github.com/torchstellar-team/one-cli/packages/one-client/provider"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/desktopcontrol"
	session "github.com/torchstellar-team/one-cli/services/desktop-api/internal/providers/infisicalsession"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
)

func TestBrowserTokenSignInThroughManagementAPI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	root := t.TempDir()
	db, err := store.Open(ctx, store.Config{SQLite: filepath.Join(root, "one.db"), Key: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	claims, _ := json.Marshal(map[string]any{"userId": "test-user", "organizationId": "test-org", "exp": time.Now().Add(time.Hour).Unix()})
	providerToken := "header." + base64.RawURLEncoding.EncodeToString(claims) + ".signature"
	var verified atomic.Int32
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/auth/checkAuth" || r.Header.Get("Authorization") != "Bearer "+providerToken {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		verified.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(identity.Close)
	admin := strings.Repeat("a", 64)
	h := BuildMux(MuxOpts{Lifecycle: ctx, Store: db, Local: true, Manager: &session.Manager{Store: db, Root: root, Local: true}, Session: &desktopcontrol.Session{Bootstrap: desktopcontrol.Bootstrap{Token: admin}}})
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	t.Setenv("ONE_HOME", t.TempDir())
	t.Setenv("ONE_SERVER_URL", server.URL)
	t.Setenv("ONE_SERVER_TOKEN", admin)
	if r := call(h, "POST", "/api/v1/session/login/token", `{"token":"invalid"}`, "", "", ""); r.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated token submission: %d", r.Code)
	}
	attempt, err := provider.Start(ctx, identity.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(attempt.Cancel)
	if _, err, done := attempt.Result(); err != nil || done {
		t.Fatalf("login should be pending: done=%t err=%v", done, err)
	}
	for _, invalid := range []string{"!", strings.Repeat("x", 96<<10+1)} {
		if err := attempt.SubmitBrowserToken(invalid); err == nil {
			t.Fatal("invalid browser token accepted")
		}
	}
	if verified.Load() != 0 {
		t.Fatal("malformed tokens reached the identity provider")
	}
	payload, _ := json.Marshal(map[string]string{"email": "user@example.com", "JTWToken": providerToken})
	if err := attempt.SubmitBrowserToken(base64.StdEncoding.EncodeToString(payload)); err != nil {
		t.Fatal(err)
	}
	info, err := attempt.Wait()
	if err != nil || !info.LoggedIn || info.UserID != "test-user" || info.OrganizationID != "test-org" {
		t.Fatalf("browser sign-in failed: info=%+v err=%v", info, err)
	}
	if verified.Load() != 1 {
		t.Fatal("browser token was not verified exactly once")
	}
	if r := call(h, "GET", "/api/v1/session", "", admin, "", ""); strings.Contains(r.Body.String(), providerToken) {
		t.Fatal("provider token leaked in session metadata")
	}
	var state struct {
		Configuration session.Configuration `json:"configuration"`
	}
	metadata := call(h, "GET", "/api/v1/session", "", admin, "", "")
	if json.Unmarshal(metadata.Body.Bytes(), &state) != nil || state.Configuration.Mode != "local" || state.Configuration.Source != "database" || state.Configuration.Method != "browser" || state.Configuration.Managed {
		t.Fatal("local browser session metadata is incorrect")
	}
	if err := provider.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if info, err := provider.Status(ctx); err != nil || info.LoggedIn {
		t.Fatalf("logout failed: info=%+v err=%v", info, err)
	}
}
