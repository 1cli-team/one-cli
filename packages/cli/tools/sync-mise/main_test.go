package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBuildResourceVerificationAndCache(t *testing.T) {
	archive := []byte("pinned archive")
	digest := fmt.Sprintf("%x", sha256.Sum256(archive))
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "assets", "mise.tar.gz")
	for range 2 {
		if err := syncArchive(context.Background(), server.Client(), server.URL, path, digest); err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 1 {
		t.Fatal("valid asset was downloaded again")
	}
	if err := os.WriteFile(path, []byte("previous file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncArchive(context.Background(), server.Client(), server.URL, path, "wrong-digest"); err == nil {
		t.Fatal("accepted mismatched archive")
	}
	if requests.Load() != 2 {
		t.Fatal("checksum mismatch was retried")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "previous file" {
		t.Fatalf("previous resource replaced: %q %v", got, err)
	}
	if err := syncArchive(context.Background(), server.Client(), server.URL, path, digest); err != nil {
		t.Fatal(err)
	}
	server.Close()
	if err := syncArchive(context.Background(), server.Client(), server.URL, path, digest); err != nil {
		t.Fatalf("offline build cache: %v", err)
	}
	leaked, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".fetch-*"))
	if err != nil || len(leaked) != 0 {
		t.Fatalf("temporary resources: %v %v", leaked, err)
	}
}

func TestBuildResourceHTTPErrorAndCancellation(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "mise.zip")
	if err := syncArchive(context.Background(), server.Client(), server.URL, path, "digest"); err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("HTTP error lost: %v", err)
	}
	if requests.Load() != maxDownloadAttempts {
		t.Fatalf("requests = %d, want %d", requests.Load(), maxDownloadAttempts)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := syncArchive(ctx, server.Client(), server.URL, path, "digest"); err == nil {
		t.Fatal("cancellation ignored")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed fetch published an asset")
	}
}

func TestBuildResourceRetriesTransientFailures(t *testing.T) {
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusOK} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			t.Parallel()
			archive := []byte("verified archive")
			digest := fmt.Sprintf("%x", sha256.Sum256(archive))
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) == 1 {
					if status == http.StatusOK {
						// A dropped connection must discard the partial download.
						w.Header().Set("Content-Length", "100")
					}
					w.WriteHeader(status)
					_, _ = w.Write([]byte("partial archive"))
					return
				}
				_, _ = w.Write(archive)
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "mise.tar.gz")
			if err := syncArchive(context.Background(), server.Client(), server.URL, path, digest); err != nil {
				t.Fatal(err)
			}
			if requests.Load() != 2 || !validArchive(path, digest) {
				t.Fatalf("retry did not publish verified archive: requests=%d", requests.Load())
			}
			leaked, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".fetch-*"))
			if err != nil || len(leaked) != 0 {
				t.Fatalf("temporary resources: %v %v", leaked, err)
			}
		})
	}
}

func TestBuildResourcePermanentHTTPError(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "mise.zip")
	if err := os.WriteFile(path, []byte("previous file"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := syncArchive(context.Background(), server.Client(), server.URL, path, "digest")
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") || requests.Load() != 1 {
		t.Fatalf("permanent error: requests=%d, err=%v", requests.Load(), err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "previous file" {
		t.Fatalf("previous resource replaced: %q %v", got, err)
	}
}

func TestBuildResourceCancelsRetryWait(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := syncArchive(ctx, server.Client(), server.URL, filepath.Join(t.TempDir(), "mise.zip"), "digest")
	if !errors.Is(err, context.DeadlineExceeded) || requests.Load() != 1 {
		t.Fatalf("cancellation: requests=%d, err=%v", requests.Load(), err)
	}
}
