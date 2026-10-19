package infisicalsession

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
)

func TestDeploymentIdentityOverridesDatabaseWithoutPersistence(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{SQLite: filepath.Join(t.TempDir(), "one.db"), Key: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	oldMachine := []byte(`{"siteUrl":"https://old.example","clientId":"old","clientSecret":"old-credential"}`)
	oldSession, _ := json.Marshal(Session{Info: Info{LoggedIn: true, SiteURL: "https://old.example", ExpiresAt: time.Now().Add(time.Hour)}, Token: "old-access-token"})
	for key, value := range map[string][]byte{"infisical-machine": oldMachine, "infisical-session": oldSession} {
		if err := db.PutSecret(ctx, key, value); err != nil {
			t.Fatal(err)
		}
	}
	var calls atomic.Int32
	var reject atomic.Bool
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var input map[string]string
		if r.URL.Path != "/api/v1/auth/universal-auth/login" || json.NewDecoder(r.Body).Decode(&input) != nil || input["clientId"] != "deployed" || input["clientSecret"] != "deployment-credential" {
			t.Error("incorrect machine authentication request")
			w.WriteHeader(400)
			return
		}
		if reject.Load() {
			w.WriteHeader(401)
			_, _ = w.Write([]byte("deployment-credential must never be reflected"))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"accessToken": "deployment-access-token", "expiresIn": 3600})
	}))
	t.Cleanup(identity.Close)
	m := &Manager{Store: db, Deployment: &Deployment{Source: "secret-file", Machine: Machine{SiteURL: identity.URL, ClientID: "deployed", ClientSecret: "deployment-credential", OrganizationID: "org"}}}
	ctx = WithManager(ctx, m)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			s, err := Require(ctx)
			if err != nil || s.UserID != "deployed" || s.Token != "deployment-access-token" {
				t.Error("deployment identity was not used")
			}
		})
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("concurrent requests did not share the cached token")
	}
	cfg, err := ConfigurationFor(ctx)
	if err != nil || !cfg.Managed || cfg.Source != "secret-file" || cfg.Mode != "remote" || cfg.ClientID != "deployed" {
		t.Fatal("deployment metadata is incorrect")
	}
	data, _ := json.Marshal(cfg)
	if strings.Contains(string(data), "deployment-credential") || strings.Contains(string(data), "deployment-access-token") {
		t.Fatal("configuration metadata exposed credentials")
	}
	m.deploymentMu.Lock()
	m.deploymentSession.ExpiresAt = time.Now().Add(-time.Minute)
	m.deploymentMu.Unlock()
	if _, err := Require(ctx); err != nil || calls.Load() != 2 {
		t.Fatalf("expired deployment token did not renew: %v", err)
	}
	m.deploymentMu.Lock()
	m.deploymentSession.ExpiresAt = time.Now().Add(-time.Minute)
	m.deploymentMu.Unlock()
	reject.Store(true)
	if _, err := Require(ctx); err == nil || strings.Contains(err.Error(), "deployment-credential") {
		t.Fatal("failed deployment authentication fell back or exposed credentials")
	}
	if _, err := ConnectMachine(ctx, Machine{}); err == nil {
		t.Fatal("deployment identity was writable")
	}
	if err := Logout(ctx); err == nil {
		t.Fatal("deployment identity was logged out")
	}
	for key, expected := range map[string][]byte{"infisical-machine": oldMachine, "infisical-session": oldSession} {
		actual, err := db.Secret(ctx, key)
		if err != nil || string(actual) != string(expected) {
			t.Fatal("deployment credentials changed database credentials")
		}
	}
	m.Deployment = nil // Simulate a new process without deployment inputs.
	if s, err := Require(ctx); err != nil || s.Token != "old-access-token" {
		t.Fatal("stored identity was not preserved")
	}
}

func TestDeploymentSessionDoesNotPersistCredentials(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{SQLite: filepath.Join(t.TempDir(), "one.db"), Key: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"accessToken": "memory-only", "expiresIn": 3600})
	}))
	t.Cleanup(identity.Close)
	m := &Manager{Store: db, Deployment: &Deployment{Source: "environment", Machine: Machine{SiteURL: identity.URL, ClientID: "deployed", ClientSecret: "deployment-credential"}}}
	ctx = WithManager(ctx, m)
	if _, err := Require(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Secret(ctx, "infisical-session"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("deployment token was persisted")
	}
	if _, err := db.Secret(ctx, "infisical-machine"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("deployment identity was persisted")
	}
}
