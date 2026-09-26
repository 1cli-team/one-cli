package mise

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func serveArchive(t *testing.T, archive []byte) (downloader, *httptest.Server, *atomic.Int32) {
	t.Helper()
	requests := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write(archive)
	}))
	t.Cleanup(server.Close)
	return downloader{client: server.Client(), baseURL: server.URL, backoff: time.Millisecond}, server, requests
}

func TestDownloadRetriesOnlyTransientFailures(t *testing.T) {
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusOK, http.StatusNotFound, http.StatusForbidden} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			t.Parallel()
			archive := []byte("verified archive")
			a := releaseAsset{Platform: "test", Format: "zip", ArchiveSHA256: fmt.Sprintf("%x", sha256.Sum256(archive))}
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/"+a.Filename() || r.Header.Get("User-Agent") != "one-cli/mise-bootstrap" {
					t.Errorf("unexpected download request: %s %s", r.URL.Path, r.Header.Get("User-Agent"))
				}
				if requests.Add(1) == 1 {
					if status == http.StatusOK {
						w.Header().Set("Content-Length", "100") // Interrupted response.
					}
					w.WriteHeader(status)
					_, _ = w.Write([]byte("partial"))
					return
				}
				_, _ = w.Write(archive)
			}))
			defer server.Close()
			d := downloader{client: server.Client(), baseURL: server.URL, backoff: time.Millisecond}
			dir := t.TempDir()
			path, err := d.fetch(context.Background(), dir, a)
			if status == http.StatusNotFound || status == http.StatusForbidden {
				if err == nil || requests.Load() != 1 || !strings.Contains(err.Error(), fmt.Sprint(status)) {
					t.Fatalf("permanent failure: requests=%d err=%v", requests.Load(), err)
				}
			} else {
				if err != nil || requests.Load() != 2 {
					t.Fatalf("retry: requests=%d err=%v", requests.Load(), err)
				}
				if err := verifyFile(path, a.ArchiveSHA256, maxArchiveSize); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			assertNoTemporaryFiles(t, dir)
		})
	}
}

func TestDownloadRejectsDigestAndOversizedResponseWithoutRetry(t *testing.T) {
	for _, oversized := range []bool{false, true} {
		t.Run(fmt.Sprint(oversized), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if oversized {
					w.Header().Set("Content-Length", fmt.Sprint(maxArchiveSize+1))
				}
				_, _ = w.Write([]byte("bad digest"))
			}))
			defer server.Close()
			d := downloader{client: server.Client(), baseURL: server.URL}
			dir := t.TempDir()
			_, err := d.fetch(context.Background(), dir, releaseAsset{Format: "zip"})
			if err == nil || requests.Load() != 1 {
				t.Fatalf("unverified response: requests=%d err=%v", requests.Load(), err)
			}
			assertNoTemporaryFiles(t, dir)
		})
	}
}

func TestDownloadRetryLimitAndCancellation(t *testing.T) {
	for _, cancelWait := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelWait), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer server.Close()
			d := downloader{client: server.Client(), baseURL: server.URL, backoff: time.Millisecond}
			ctx := context.Background()
			if cancelWait {
				d.backoff = time.Second
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
			}
			dir := t.TempDir()
			_, err := d.fetch(ctx, dir, releaseAsset{Format: "zip"})
			if cancelWait {
				if !errors.Is(err, context.DeadlineExceeded) || requests.Load() != 1 {
					t.Fatalf("retry cancellation: %d %v", requests.Load(), err)
				}
			} else if err == nil || requests.Load() != maxDownloadAttempts {
				t.Fatalf("retry limit: %d %v", requests.Load(), err)
			}
			assertNoTemporaryFiles(t, dir)
		})
	}
}

func TestDownloadCancelsInFlightResponse(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { <-started; cancel() }()
	dir := t.TempDir()
	d := downloader{client: server.Client(), baseURL: server.URL}
	if _, err := d.fetch(ctx, dir, releaseAsset{Format: "zip"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("in-flight cancellation: %v", err)
	}
	assertNoTemporaryFiles(t, dir)
}

func TestOfficialDownloadRejectsHTTPSDowngrade(t *testing.T) {
	var targetRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetRequests.Add(1) }))
	defer target.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer server.Close()
	d := defaultDownloader()
	d.client.Transport = server.Client().Transport
	d.baseURL, d.backoff = server.URL, 0
	if _, err := d.fetch(context.Background(), t.TempDir(), releaseAsset{Format: "zip"}); err == nil || !strings.Contains(err.Error(), "non-HTTPS") {
		t.Fatalf("downgrade: %v", err)
	}
	if targetRequests.Load() != 0 {
		t.Fatal("followed insecure redirect")
	}
}
