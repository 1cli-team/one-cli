package updatecheck

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func helperArchive(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	gz := gzip.NewWriter(&body)
	tw := tar.NewWriter(gz)
	value := []byte("exact-release-server")
	if err := tw.WriteHeader(&tar.Header{Name: "one-api", Mode: 0755, Size: int64(len(value))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(value); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}
func TestHistoricalUpdaterRepairUsesExactVerifiedRelease(t *testing.T) {
	archive := helperArchive(t)
	hash := sha256.Sum256(archive)
	for _, valid := range []bool{true, false} {
		t.Run(fmt.Sprint(valid), func(t *testing.T) {
			requests := 0
			sum := fmt.Sprintf("%x", hash)
			if !valid {
				sum = strings.Repeat("0", 64)
			}
			u := updater{releaseURL: "https://releases.example/download", goos: "linux", goarch: "amd64", probe: func(_ context.Context, path, version string) error {
				value, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if string(value) != "exact-release-server" || version != "v1.2.3" {
					t.Fatal("wrong release candidate")
				}
				return nil
			}}
			u.client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				requests++
				var data []byte
				switch r.URL.Path {
				case "/download/v1.2.3/checksums.txt":
					data = []byte(sum + "  one-cli_linux_amd64.tar.gz\n")
				case "/download/v1.2.3/one-cli_linux_amd64.tar.gz":
					data = archive
				default:
					t.Fatalf("unexpected download: %s", r.URL.Path)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
			})}
			root := t.TempDir()
			helper, err := u.ensureCompanion(context.Background(), root, "v1.2.3")
			if !valid {
				if err == nil {
					t.Fatal("accepted modified archive")
				}
				matches, _ := filepath.Glob(filepath.Join(root, "data/companions/1.2.3/linux-amd64/one-api"))
				if len(matches) != 0 {
					t.Fatal("published unverified helper")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if requests != 2 {
				t.Fatal(requests)
			}
			again, err := u.ensureCompanion(context.Background(), root, "v1.2.3")
			if err != nil || again != helper || requests != 2 {
				t.Fatalf("cached repair: %s %v requests=%d", again, err, requests)
			}
		})
	}
}
