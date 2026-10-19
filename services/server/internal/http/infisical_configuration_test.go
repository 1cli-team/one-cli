package serve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	oneclient "github.com/torchstellar-team/one-cli/packages/one-client"
	"github.com/torchstellar-team/one-cli/packages/one-client/provider"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/desktopcontrol"
	session "github.com/torchstellar-team/one-cli/services/desktop-api/internal/providers/infisicalsession"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
)

func TestDeploymentConfigurationIsReadOnlyThroughAPI(t *testing.T) {
	for _, source := range []string{"environment", "secret-file"} {
		for _, local := range []bool{false, true} {
			t.Run(source+map[bool]string{true: "/local", false: "/remote"}[local], func(t *testing.T) {
				ctx := context.Background()
				root := t.TempDir()
				db, err := store.Open(ctx, store.Config{SQLite: filepath.Join(root, "one.db"), Key: make([]byte, 32)})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = db.Close() })
				identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_ = json.NewEncoder(w).Encode(map[string]any{"accessToken": "private-provider-token", "expiresIn": 3600})
				}))
				t.Cleanup(identity.Close)
				manager := &session.Manager{Store: db, Root: root, Local: local, Deployment: &session.Deployment{Source: source, Machine: session.Machine{SiteURL: identity.URL, ClientID: "identity", ClientSecret: "private-client-secret"}}}
				admin := strings.Repeat("a", 64)
				h := BuildMux(MuxOpts{Lifecycle: ctx, Store: db, Local: local, Manager: manager, Session: &desktopcontrol.Session{Bootstrap: desktopcontrol.Bootstrap{Token: admin}}})
				response := call(h, "GET", "/api/v1/session", "", admin, "", "")
				var state struct {
					Configuration session.Configuration `json:"configuration"`
					Session       session.Info          `json:"session"`
				}
				if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &state) != nil || !state.Configuration.Managed || state.Configuration.Source != source || !state.Session.LoggedIn {
					t.Fatal("managed session metadata missing")
				}
				if (state.Configuration.Mode == "local") != local {
					t.Fatal("incorrect server mode")
				}
				for _, secret := range []string{"private-provider-token", "private-client-secret"} {
					if strings.Contains(response.Body.String(), secret) {
						t.Fatal("session metadata exposed a credential")
					}
				}
				for _, locale := range []string{"zh-CN", "en-US"} {
					for _, route := range []struct{ method, path string }{
						{"POST", "/session/machine"}, {"POST", "/session/login"}, {"POST", "/session/login/token"}, {"DELETE", "/session/login"}, {"DELETE", "/session"},
					} {
						r := call(h, route.method, "/api/v1"+route.path, `{}`, admin, "", locale)
						if r.Code != 403 || !strings.Contains(r.Body.String(), "SERVE_CONFIG_MANAGED") {
							t.Fatalf("managed mutation accepted: %s %s (%d)", route.method, route.path, r.Code)
						}
					}
				}
				info := call(h, "GET", "/api/v1/server/info", "", admin, "", "")
				var metadata oneclient.Info
				if json.Unmarshal(info.Body.Bytes(), &metadata) != nil || metadata.Capabilities["browserLogin"] {
					t.Fatal("managed server advertises mutable browser login")
				}
			})
		}
	}
}

func TestUnconfiguredServerReportsConfigurationSource(t *testing.T) {
	h := testHandler(t)
	r := call(h, "GET", "/api/v1/session", "", strings.Repeat("a", 64), "", "")
	var state struct {
		Configuration session.Configuration `json:"configuration"`
	}
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &state) != nil || state.Configuration.Source != "none" || state.Configuration.Managed || state.Configuration.Mode != "remote" {
		t.Fatal("unconfigured server did not expose an editable configuration")
	}
}

func TestDeploymentAuthenticationFailureKeepsReadOnlyMetadata(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := store.Open(ctx, store.Config{SQLite: filepath.Join(root, "one.db"), Key: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("private-client-secret"))
	}))
	t.Cleanup(identity.Close)
	manager := &session.Manager{Store: db, Root: root, Deployment: &session.Deployment{Source: "environment", Machine: session.Machine{SiteURL: identity.URL, ClientID: "identity", ClientSecret: "private-client-secret"}}}
	admin := strings.Repeat("a", 64)
	h := BuildMux(MuxOpts{Store: db, Manager: manager, Session: &desktopcontrol.Session{Bootstrap: desktopcontrol.Bootstrap{Token: admin}}})
	r := call(h, "GET", "/api/v1/session", "", admin, "", "zh-CN")
	var state provider.State
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &state) != nil || state.Configuration == nil || !state.Configuration.Managed || state.Error == "" || state.Session.LoggedIn {
		t.Fatal("failed authentication lost the deployment configuration metadata")
	}
	if strings.Contains(r.Body.String(), "private-client-secret") {
		t.Fatal("provider error body exposed a credential")
	}
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	t.Setenv("ONE_SERVER_URL", server.URL)
	t.Setenv("ONE_SERVER_TOKEN", admin)
	t.Setenv("ONE_HOME", t.TempDir())
	if _, err := provider.Status(ctx); err == nil {
		t.Fatal("CLI suppressed the deployment authentication failure")
	}
}

func TestDatabaseIdentityRemainsEditable(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := store.Open(ctx, store.Config{SQLite: filepath.Join(root, "one.db"), Key: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"accessToken": "private-provider-token", "expiresIn": 3600})
	}))
	t.Cleanup(identity.Close)
	admin := strings.Repeat("a", 64)
	h := BuildMux(MuxOpts{Store: db, Manager: &session.Manager{Store: db, Root: root}, Session: &desktopcontrol.Session{Bootstrap: desktopcontrol.Bootstrap{Token: admin}}})
	input, _ := json.Marshal(session.Machine{SiteURL: identity.URL, ClientID: "identity", ClientSecret: "private-client-secret"})
	if r := call(h, "POST", "/api/v1/session/machine", string(input), admin, "", ""); r.Code != 200 {
		t.Fatal("manual machine identity configuration failed")
	}
	r := call(h, "GET", "/api/v1/session", "", admin, "", "")
	var state provider.State
	if json.Unmarshal(r.Body.Bytes(), &state) != nil || state.Configuration == nil || state.Configuration.Managed || state.Configuration.Source != "database" || state.Configuration.Method != "machine" {
		t.Fatal("manual identity did not report database configuration")
	}
	if r := call(h, "DELETE", "/api/v1/session", "", admin, "", ""); r.Code != 200 {
		t.Fatal("manual identity could not be disconnected")
	}
}
