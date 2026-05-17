package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fetchAt is a test-only seam: same code path as fetchLatest but lets us
// point at an httptest server. We can't override the const URL otherwise.
func fetchAt(ctx context.Context, url, currentVersion string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "one-cli/"+currentVersion)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", &httpError{status: resp.StatusCode}
	}
	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	v := normalizeTag(strings.TrimSpace(string(buf[:n])))
	if v == "" {
		return "", &httpError{status: resp.StatusCode, body: string(buf[:n])}
	}
	return v, nil
}

type httpError struct {
	status int
	body   string
}

func (e *httpError) Error() string { return "fetch failed" }

func TestFetch_Happy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "one-cli/") {
			t.Errorf("missing/bad User-Agent: %q", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("v0.9.0\n"))
	}))
	defer srv.Close()
	got, err := fetchAt(context.Background(), srv.URL, "v0.8.0")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got != "v0.9.0" {
		t.Errorf("got %q, want v0.9.0", got)
	}
}

func TestFetch_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()
	_, err := fetchAt(context.Background(), srv.URL, "v0.8.0")
	if err == nil {
		t.Errorf("expected error for 503, got nil")
	}
}

func TestFetch_GarbageBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("not a version"))
	}))
	defer srv.Close()
	_, err := fetchAt(context.Background(), srv.URL, "v0.8.0")
	if err == nil {
		t.Errorf("expected error for garbage body, got nil")
	}
}

// TestFetchLatest_LiveURL is a sanity check that the production endpoint
// is well-formed when reachable. Skipped by default (`go test -tags=net`)
// — we don't want pre-push to fail when a developer is offline.
func TestFetchLatest_RealURL_Skipped(t *testing.T) {
	t.Skip("opt-in network probe; run manually with `go test -run RealURL ./internal/updatecheck`")
	v, err := fetchLatest(context.Background(), "v0.0.0-test")
	if err != nil {
		t.Fatalf("real fetch: %v", err)
	}
	t.Logf("live /dl/latest = %s", v)
}
