package serve

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func withIsolatedConfig(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)
	keyring.MockInit()
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
