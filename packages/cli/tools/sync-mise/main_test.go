package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "mise.zip")
	if err := syncArchive(context.Background(), server.Client(), server.URL, path, "digest"); err == nil {
		t.Fatal("HTTP error accepted")
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
