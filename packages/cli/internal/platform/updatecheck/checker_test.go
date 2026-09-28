package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Exercise the production fetch path against a local server.
func fetchAt(ctx context.Context, url, currentVersion string) (string, error) {
	return fetchLatestUsing(ctx, http.DefaultClient, url, currentVersion)
}

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

func TestFetch_GitHubLatestRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			http.Redirect(w, r, "/releases/tag/v0.9.0", http.StatusFound)
		case "/releases/tag/v0.9.0":
			_, _ = w.Write([]byte("<html>release page</html>"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	got, err := fetchAt(context.Background(), srv.URL+"/latest", "v0.8.0")
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
	t.Skip("opt-in network probe; run manually with `go test -run RealURL ./internal/platform/updatecheck`")
	v, err := fetchLatest(context.Background(), "v0.0.0-test")
	if err != nil {
		t.Fatalf("real fetch: %v", err)
	}
	t.Logf("live latest release = %s", v)
}
