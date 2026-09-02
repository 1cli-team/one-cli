package serve

// Locks the HTTP wire contract: routes, status codes, security middleware,
// credential masking, and round-trips against the real on-disk profile
// store. We use httptest.Server with an isolated XDG_CONFIG_HOME so the
// test mutates a tmpdir, not the developer's actual config.json/credentials.json.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
)

// withIsolatedConfig redirects XDG_CONFIG_HOME / HOME so profile.Load /
// profile.Save hit a per-test tmpdir. Identical pattern to
// internal/core/profile/mutate_test.go's withIsolatedConfig.
func withIsolatedConfig(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)
}

// newTestServer builds a serve.Mux behind httptest.Server and returns the
// server plus its base URL. Caller is responsible for srv.Close().
func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	withIsolatedConfig(t)
	mux := BuildMux(MuxOpts{
		UIDisabled:    true,
		ExpectedHosts: nil, // populated below once we know the test addr
		SelfOrigin:    "",
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	// httptest binds 127.0.0.1:<random>. Patch the mux opts the test
	// expects: hosts allowlist + self-origin must match the live server.
	addr := strings.TrimPrefix(srv.URL, "http://")
	mux2 := BuildMux(MuxOpts{
		UIDisabled:    true,
		ExpectedHosts: map[string]struct{}{addr: {}},
		SelfOrigin:    srv.URL,
	})
	srv.Config.Handler = mux2
	return srv, srv.URL
}

// apiRequest issues r against srv and returns response + body bytes for inline
// assertions. Mutations carry the same-origin header required by production.
func apiRequest(t *testing.T, srv *httptest.Server, method, path string, body io.Reader) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", srv.URL)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return res, raw
}

func TestGetConfig_EmptyByDefault(t *testing.T) {
	srv, _ := newTestServer(t)
	res, raw := apiRequest(t, srv, http.MethodGet, "/api/configure", nil)
	if res.StatusCode != 200 {
		t.Fatalf("status: want 200, got %d (%s)", res.StatusCode, raw)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["schema"] != schemaConfig {
		t.Errorf("schema: got %v", got["schema"])
	}
	if got["reveal"] != false {
		t.Errorf("reveal: want false default, got %v", got["reveal"])
	}
}

// Upsert via POST writes through to ~/.config/one/config.json and
// credentials.json (the isolated tmpdir variant). The default flag should
// auto-set on the first profile in a section.
func TestUpsert_FirstProfile_AutoDefault(t *testing.T) {
	srv, _ := newTestServer(t)
	body := strings.NewReader(`{"name":"work","profile":{"siteUrl":"https://app.infisical.com","credentials":{"clientId":"cid","clientSecret":"sec"}}}`)
	res, raw := apiRequest(t, srv, http.MethodPost, "/api/configure/env/infisical", body)
	if res.StatusCode != 200 {
		t.Fatalf("status: want 200, got %d (%s)", res.StatusCode, raw)
	}
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	if got["status"] != "completed" {
		t.Errorf("status: want completed, got %v", got["status"])
	}
	if got["default"] != true {
		t.Errorf("default: first add should auto-default, got %v", got["default"])
	}
	// On-disk verification: profile.Load should now find it.
	cfg, _, err := profile.Load()
	if err != nil {
		t.Fatalf("profile.Load: %v", err)
	}
	if cfg.EnvInfisical.Default != "work" {
		t.Errorf("on-disk default: want work, got %q", cfg.EnvInfisical.Default)
	}
	if cfg.EnvInfisical.Profiles["work"].Credentials.ClientSecret != "sec" {
		t.Errorf("credential not persisted")
	}
}

// GET the section back: clientSecret must be masked unless reveal=1.
func TestGetSection_MasksByDefault_RevealsOnQuery(t *testing.T) {
	srv, _ := newTestServer(t)
	body := strings.NewReader(`{"name":"work","profile":{"siteUrl":"https://x","credentials":{"clientId":"cid","clientSecret":"plaintext-secret"}}}`)
	if res, raw := apiRequest(t, srv, http.MethodPost, "/api/configure/env/infisical", body); res.StatusCode != 200 {
		t.Fatalf("seed: %d (%s)", res.StatusCode, raw)
	}

	// Default: secret masked.
	_, raw := apiRequest(t, srv, http.MethodGet, "/api/configure/env/infisical", nil)
	if strings.Contains(string(raw), "plaintext-secret") {
		t.Errorf("plaintext secret leaked in default GET: %s", raw)
	}
	if !strings.Contains(string(raw), "********") {
		t.Errorf("expected masked sentinel; got %s", raw)
	}

	// reveal=1: actual secret returned.
	_, raw2 := apiRequest(t, srv, http.MethodGet, "/api/configure/env/infisical?reveal=1", nil)
	if !strings.Contains(string(raw2), "plaintext-secret") {
		t.Errorf("reveal=1 should expose plaintext; got %s", raw2)
	}
}

func TestUpsert_MaskedCredentialPreservesExistingSecret(t *testing.T) {
	srv, _ := newTestServer(t)
	body := strings.NewReader(`{"name":"work","profile":{"siteUrl":"https://x","credentials":{"clientId":"cid","clientSecret":"original-secret"}}}`)
	if res, raw := apiRequest(t, srv, http.MethodPost, "/api/configure/env/infisical", body); res.StatusCode != 200 {
		t.Fatalf("seed: %d (%s)", res.StatusCode, raw)
	}

	update := strings.NewReader(`{"name":"work","profile":{"siteUrl":"https://updated","credentials":{"clientId":"cid-rotated","clientSecret":"********"}}}`)
	if res, raw := apiRequest(t, srv, http.MethodPost, "/api/configure/env/infisical", update); res.StatusCode != 200 {
		t.Fatalf("update: %d (%s)", res.StatusCode, raw)
	}

	cfg, _, err := profile.Load()
	if err != nil {
		t.Fatalf("profile.Load: %v", err)
	}
	got := cfg.EnvInfisical.Profiles["work"]
	if got.SiteURL != "https://updated" {
		t.Errorf("siteUrl should update, got %q", got.SiteURL)
	}
	if got.Credentials == nil {
		t.Fatal("credentials missing")
	}
	if got.Credentials.ClientID != "cid-rotated" {
		t.Errorf("clientId should update, got %q", got.Credentials.ClientID)
	}
	if got.Credentials.ClientSecret != "original-secret" {
		t.Errorf("clientSecret should be preserved, got %q", got.Credentials.ClientSecret)
	}
}

func TestGetSection_MasksDeployTokensByDefault(t *testing.T) {
	srv, _ := newTestServer(t)
	cases := []struct {
		path   string
		body   string
		secret string
	}{
		{
			path:   "/api/configure/deploy/cloudflare",
			body:   `{"name":"work","profile":{"accountId":"acct","credentials":{"apiToken":"cloudflare-secret"}}}`,
			secret: "cloudflare-secret",
		},
		{
			path:   "/api/configure/deploy/edgeone",
			body:   `{"name":"work","profile":{"region":"ap-guangzhou","credentials":{"apiToken":"edgeone-secret"}}}`,
			secret: "edgeone-secret",
		},
	}

	for _, tc := range cases {
		if res, raw := apiRequest(t, srv, http.MethodPost, tc.path, strings.NewReader(tc.body)); res.StatusCode != 200 {
			t.Fatalf("seed %s: %d (%s)", tc.path, res.StatusCode, raw)
		}
		_, raw := apiRequest(t, srv, http.MethodGet, tc.path, nil)
		if strings.Contains(string(raw), tc.secret) {
			t.Errorf("%s leaked default token: %s", tc.path, raw)
		}
		if !strings.Contains(string(raw), "********") {
			t.Errorf("%s should contain masked sentinel; got %s", tc.path, raw)
		}

		_, raw = apiRequest(t, srv, http.MethodGet, tc.path+"?reveal=1", nil)
		if !strings.Contains(string(raw), tc.secret) {
			t.Errorf("%s reveal=1 should expose plaintext; got %s", tc.path, raw)
		}
	}
}

func TestUse_SwitchesDefault(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, n := range []string{"work", "personal"} {
		body := strings.NewReader(`{"name":"` + n + `","profile":{"siteUrl":"https://x","credentials":{"clientId":"c","clientSecret":"s"}}}`)
		if res, raw := apiRequest(t, srv, http.MethodPost, "/api/configure/env/infisical", body); res.StatusCode != 200 {
			t.Fatalf("seed %s: %d (%s)", n, res.StatusCode, raw)
		}
	}
	// First add becomes default; switch to personal.
	body := strings.NewReader(`{"name":"personal"}`)
	res, raw := apiRequest(t, srv, http.MethodPut, "/api/configure/env/infisical/default", body)
	if res.StatusCode != 200 {
		t.Fatalf("use: %d (%s)", res.StatusCode, raw)
	}
	cfg, _, _ := profile.Load()
	if cfg.EnvInfisical.Default != "personal" {
		t.Errorf("want default=personal, got %q", cfg.EnvInfisical.Default)
	}
}

func TestRemove_DropsProfile(t *testing.T) {
	srv, _ := newTestServer(t)
	body := strings.NewReader(`{"name":"work","profile":{"siteUrl":"https://x","credentials":{"clientId":"c","clientSecret":"s"}}}`)
	if res, _ := apiRequest(t, srv, http.MethodPost, "/api/configure/env/infisical", body); res.StatusCode != 200 {
		t.Fatalf("seed")
	}
	res, raw := apiRequest(t, srv, http.MethodDelete, "/api/configure/env/infisical/work", nil)
	if res.StatusCode != 200 {
		t.Fatalf("delete: %d (%s)", res.StatusCode, raw)
	}
	cfg, _, _ := profile.Load()
	if _, ok := cfg.EnvInfisical.Profiles["work"]; ok {
		t.Errorf("profile not removed")
	}
}

func TestAPI_DoesNotRequireSessionToken(t *testing.T) {
	srv, _ := newTestServer(t)
	res, err := http.Get(srv.URL + "/api/configure")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Errorf("want 200, got %d", res.StatusCode)
	}
}

// Security: bad Host header → 421 (DNS rebinding defense). Use raw http
// client because http.Client overwrites Host based on req.URL.
func TestHostCheck_RejectsAttackerDomain(t *testing.T) {
	srv, _ := newTestServer(t)
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/configure", nil)
	req.Host = "attacker.example.com" // overrides what's sent in Host header
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("want 421, got %d", res.StatusCode)
	}
}

// Security: bad Origin on POST → 403.
func TestOriginCheck_RejectsCrossOriginPost(t *testing.T) {
	srv, _ := newTestServer(t)
	body := bytes.NewReader([]byte(`{"name":"x","profile":{"siteUrl":"https://x","credentials":{"clientId":"c","clientSecret":"s"}}}`))
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/configure/env/infisical", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://attacker.example.com")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 403 {
		t.Errorf("want 403, got %d", res.StatusCode)
	}
}

// Unknown (domain, backend) → 404.
func TestValidPair_UnknownReturns404(t *testing.T) {
	srv, _ := newTestServer(t)
	res, _ := apiRequest(t, srv, http.MethodGet, "/api/configure/foo/bar", nil)
	if res.StatusCode != 404 {
		t.Errorf("want 404, got %d", res.StatusCode)
	}
}

// Malformed JSON body → 400.
func TestUpsert_MalformedBody_400(t *testing.T) {
	srv, _ := newTestServer(t)
	body := strings.NewReader(`{ this is not json `)
	res, _ := apiRequest(t, srv, http.MethodPost, "/api/configure/env/infisical", body)
	if res.StatusCode != 400 {
		t.Errorf("want 400, got %d", res.StatusCode)
	}
}

// Missing name → 400 (handler's own validation, before profile package).
func TestUpsert_MissingName_400(t *testing.T) {
	srv, _ := newTestServer(t)
	body := strings.NewReader(`{"profile":{}}`)
	res, _ := apiRequest(t, srv, http.MethodPost, "/api/configure/env/dotenv", body)
	if res.StatusCode != 400 {
		t.Errorf("want 400, got %d", res.StatusCode)
	}
}
