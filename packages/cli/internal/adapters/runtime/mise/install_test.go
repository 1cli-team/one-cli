package mise

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func archiveFixture(t *testing.T, format string, binary []byte) (releaseAsset, []byte) {
	t.Helper()
	var buf bytes.Buffer
	a := releaseAsset{Platform: "test-platform", Format: format, BinarySHA256: fmt.Sprintf("%x", sha256.Sum256(binary))}
	if format == "zip" {
		z := zip.NewWriter(&buf)
		w, err := z.Create("mise/bin/" + a.BinaryName())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(binary); err != nil {
			t.Fatal(err)
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
	} else {
		gz := gzip.NewWriter(&buf)
		tr := tar.NewWriter(gz)
		// This unrelated entry must never be extracted to the filesystem.
		if err := tr.WriteHeader(&tar.Header{Name: "../../escape", Mode: 0o644, Size: 1}); err != nil {
			t.Fatal(err)
		}
		if _, err := tr.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
		if err := tr.WriteHeader(&tar.Header{Name: "mise/bin/" + a.BinaryName(), Mode: 0o755, Size: int64(len(binary))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tr.Write(binary); err != nil {
			t.Fatal(err)
		}
		if err := tr.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
	}
	a.ArchiveSHA256 = fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))
	return a, buf.Bytes()
}

func TestReleaseAssetsCoverSupportedOnePlatforms(t *testing.T) {
	for _, target := range [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}, {"windows", "amd64"}} {
		a, err := assetFor(target[0], target[1])
		if err != nil {
			t.Fatal(err)
		}
		for _, digest := range []string{a.BinarySHA256, a.ArchiveSHA256} {
			b, err := hex.DecodeString(digest)
			if err != nil || len(b) != sha256.Size {
				t.Fatalf("bad digest for %v", target)
			}
		}
		if !strings.Contains(a.Filename(), managedVersion) {
			t.Fatal("unversioned asset")
		}
	}
	if _, err := assetFor("plan9", "amd64"); err == nil {
		t.Fatal("unsupported platform accepted")
	}
}

func TestManagedInstallConcurrentOfflineReuseAndRepair(t *testing.T) {
	for _, format := range []string{"tar.gz", "zip"} {
		t.Run(format, func(t *testing.T) {
			a, archive := archiveFixture(t, format, []byte("verified executable fixture"))
			d, server, requests := serveArchive(t, archive)
			root := t.TempDir()
			i := installer{root: root, downloader: d}
			var wg sync.WaitGroup
			for range 8 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if _, err := i.ensure(context.Background(), a); err != nil {
						t.Error(err)
					}
				}()
			}
			wg.Wait()
			if requests.Load() != 1 {
				t.Fatalf("downloaded %d times", requests.Load())
			}
			path, err := i.ensure(context.Background(), a)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "escape")); !os.IsNotExist(err) {
				t.Fatal("archive escaped destination")
			}
			if err := os.WriteFile(path, []byte("damaged"), 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := i.ensure(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if _, err := i.ensure(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			if requests.Load() != 3 {
				t.Fatalf("repair requests = %d, want 3", requests.Load())
			}
			server.Close()
			if _, err := i.ensure(context.Background(), a); err != nil {
				t.Fatalf("offline cache: %v", err)
			}
			assertNoTemporaryFiles(t, filepath.Dir(path))
		})
	}
}

func TestManagedInstallFailureNeverPublishesOrReplacesBinary(t *testing.T) {
	for _, failure := range []string{"archive-digest", "binary-digest", "cancelled", "invalid-archive"} {
		t.Run(failure, func(t *testing.T) {
			a, archive := archiveFixture(t, "tar.gz", []byte("expected binary"))
			if failure == "archive-digest" {
				a.ArchiveSHA256 = strings.Repeat("0", 64)
			}
			if failure == "binary-digest" {
				a.BinarySHA256 = strings.Repeat("0", 64)
			}
			if failure == "invalid-archive" {
				archive = []byte("not a tarball")
				a.ArchiveSHA256 = fmt.Sprintf("%x", sha256.Sum256(archive))
			}
			d, _, requests := serveArchive(t, archive)
			i := installer{root: t.TempDir(), downloader: d}
			dir := filepath.Join(i.root, managedVersion, a.Platform)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, a.BinaryName())
			if err := os.WriteFile(path, []byte("previous file"), 0o755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure == "cancelled" {
				cancel()
			}
			if _, err := i.ensure(ctx, a); err == nil {
				t.Fatal("bad install succeeded")
			}
			raw, err := os.ReadFile(path)
			if err != nil || string(raw) != "previous file" {
				t.Fatalf("previous binary modified: %v %q", err, raw)
			}
			if failure == "cancelled" && requests.Load() != 0 {
				t.Fatal("cancelled install downloaded")
			}
			assertNoTemporaryFiles(t, dir)
		})
	}
}

func TestManagedInstallMigratesOnlyVerifiedLegacyBinary(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(fmt.Sprint(valid), func(t *testing.T) {
			binary := []byte("verified legacy binary")
			a, archive := archiveFixture(t, "tar.gz", binary)
			d, server, requests := serveArchive(t, archive)
			i := installer{root: filepath.Join(t.TempDir(), "new"), legacyRoot: filepath.Join(t.TempDir(), "old"), downloader: d}
			legacy := filepath.Join(i.legacyRoot, managedVersion, a.Platform, a.BinaryName())
			if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
				t.Fatal(err)
			}
			old := binary
			if !valid {
				old = []byte("corrupt legacy")
			} else {
				server.Close()
			}
			if err := os.WriteFile(legacy, old, 0o755); err != nil {
				t.Fatal(err)
			}
			target, err := i.ensure(context.Background(), a)
			if err != nil {
				t.Fatal(err)
			}
			if err := verifyExecutable(target, a.BinarySHA256); err != nil {
				t.Fatal(err)
			}
			preserved, err := os.ReadFile(legacy)
			if err != nil || !bytes.Equal(preserved, old) {
				t.Fatal("legacy cache changed")
			}
			want := int32(1)
			if valid {
				want = 0
			}
			if requests.Load() != want {
				t.Fatalf("requests = %d, want %d", requests.Load(), want)
			}
		})
	}
}

func TestManagedInstallCancellationWhileWaitingForLock(t *testing.T) {
	a, archive := archiveFixture(t, "tar.gz", []byte("binary"))
	d, _, requests := serveArchive(t, archive)
	i := installer{root: t.TempDir(), downloader: d}
	dir := filepath.Join(i.root, managedVersion, a.Platform)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	lock := flock.New(filepath.Join(dir, ".install.lock"))
	if err := lock.Lock(); err != nil {
		t.Fatal(err)
	}
	defer lock.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := i.ensure(ctx, a); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lock cancellation: %v", err)
	}
	if requests.Load() != 0 {
		t.Fatal("downloaded before obtaining lock")
	}
}

func TestPublishFailureAndCancellationPreserveDestination(t *testing.T) {
	for _, cancelWrite := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelWrite), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mise")
			if err := os.WriteFile(path, []byte("previous"), 0o755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			data := []byte("new")
			digest := fmt.Sprintf("%x", sha256.Sum256(data))
			err := publishBinary(ctx, path, digest, func(w io.Writer) error {
				if _, err := w.Write(data); err != nil {
					return err
				}
				if cancelWrite {
					cancel()
					return nil
				}
				return errors.New("local write failed")
			})
			if err == nil {
				t.Fatal("failed write published")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "previous" {
				t.Fatalf("destination: %q %v", got, err)
			}
			assertNoTemporaryFiles(t, filepath.Dir(path))
		})
	}
}

func assertNoTemporaryFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".extract-") || strings.HasPrefix(entry.Name(), ".fetch-") {
			t.Errorf("temporary file leaked: %s", entry.Name())
		}
	}
}
