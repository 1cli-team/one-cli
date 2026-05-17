package serve

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/preferences"
)

// newPrefsMux constructs an HTTP handler wired the same way `one
// serve` does at startup. Tests issue requests through this without
// binding a port. Reuses withIsolatedConfig from handlers_configure_test.go.
func newPrefsMux(t *testing.T) http.Handler {
	t.Helper()
	return BuildMux(MuxOpts{
		Token:         testToken,
		ExpectedHosts: map[string]struct{}{"127.0.0.1": {}, "localhost": {}},
		SelfOrigin:    "http://127.0.0.1",
	})
}

func TestPreferencesGET_DefaultsToAuto(t *testing.T) {
	withIsolatedConfig(t)
	mux := newPrefsMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/preferences?token="+testToken, nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if got["schema"] != schemaPreferences {
		t.Errorf("schema: want %q, got %v", schemaPreferences, got["schema"])
	}
	if got["stored_locale"] != preferences.LocaleAuto {
		t.Errorf("stored_locale: want %q, got %v", preferences.LocaleAuto, got["stored_locale"])
	}
}

func TestPreferencesPUT_PersistsToDisk(t *testing.T) {
	withIsolatedConfig(t)
	mux := newPrefsMux(t)

	body := bytes.NewBufferString(`{"locale":"zh-CN"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/preferences?token="+testToken, body)
	req.Host = "127.0.0.1"
	req.Header.Set("Origin", "http://127.0.0.1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var got preferencesPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if got.StoredLocale != "zh-CN" {
		t.Errorf("stored_locale: want zh-CN, got %q", got.StoredLocale)
	}

	// Reading back from disk confirms persistence. preferences.Load
	// honours XDG_CONFIG_HOME (set by withIsolatedConfig) so the
	// tempdir is implicit.
	prefs, err := preferences.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if prefs.Locale != "zh-CN" {
		t.Errorf("disk locale: want zh-CN, got %q", prefs.Locale)
	}
}

func TestPreferencesPUT_RejectsBadLocale(t *testing.T) {
	withIsolatedConfig(t)
	mux := newPrefsMux(t)

	for _, bad := range []string{`{"locale":"klingon"}`, `{"locale":""}`, `{"locale":"zh_CN"}`} {
		t.Run(bad, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/preferences?token="+testToken,
				strings.NewReader(bad))
			req.Host = "127.0.0.1"
			req.Header.Set("Origin", "http://127.0.0.1")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status: want 400, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestPreferencesPUT_RejectsUnknownField(t *testing.T) {
	withIsolatedConfig(t)
	mux := newPrefsMux(t)

	req := httptest.NewRequest(http.MethodPut, "/api/preferences?token="+testToken,
		strings.NewReader(`{"locale":"zh-CN","colour":"red"}`))
	req.Host = "127.0.0.1"
	req.Header.Set("Origin", "http://127.0.0.1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400 on unknown field, got %d", rec.Code)
	}
}

func TestPreferences_RequiresToken(t *testing.T) {
	withIsolatedConfig(t)
	mux := newPrefsMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/preferences", nil) // no ?token=
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauth status: want 401, got %d", rec.Code)
	}
}
