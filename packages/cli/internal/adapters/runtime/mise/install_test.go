package mise

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
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
			binary := []byte("verified executable fixture")
			a, archive := archiveFixture(t, format, binary)
			var preparations countWriter
			root := t.TempDir()
			i := installer{root: root, archive: archive, out: &preparations}
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
			if preparations.Load() != 1 {
				t.Fatalf("extracted %d times", preparations.Load())
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
			if preparations.Load() != 2 {
				t.Fatal("cache was not repaired")
			}
			if _, err := i.ensure(context.Background(), a); err != nil {
				t.Fatalf("offline cache: %v", err)
			}
		})
	}
}

func TestManagedInstallFailureNeverPublishesOrReplacesBinary(t *testing.T) {
	for _, failure := range []string{"missing-archive", "archive-digest", "binary-digest", "cancelled"} {
		t.Run(failure, func(t *testing.T) {
			a, archive := archiveFixture(t, "tar.gz", []byte("expected binary"))
			if failure == "archive-digest" {
				a.ArchiveSHA256 = strings.Repeat("0", 64)
			}
			if failure == "binary-digest" {
				a.BinarySHA256 = strings.Repeat("0", 64)
			}
			if failure == "missing-archive" {
				archive = nil
			}
			i := installer{root: t.TempDir(), archive: archive}
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
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".extract-") {
					t.Errorf("temporary file leaked: %s", entry.Name())
				}
			}
		})
	}
}

func TestMissingBundleHasNoSideEffects(t *testing.T) {
	a, _ := archiveFixture(t, "tar.gz", []byte("binary"))
	root := filepath.Join(t.TempDir(), "not-created")
	i := installer{root: root}
	if _, err := i.ensure(context.Background(), a); err == nil {
		t.Fatal("missing runtime accepted")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("missing bundle wrote files")
	}
}

// The real per-platform resource must unpack to the separately pinned digest.
func TestBundledOfficialArchive(t *testing.T) {
	a, err := currentAsset()
	if err != nil {
		t.Skip(err)
	}
	i := installer{root: t.TempDir(), archive: bundledArchive}
	if _, err := i.ensure(context.Background(), a); err != nil {
		t.Fatal(err)
	}
}

// sync-mise-all makes foreign release resources available for cross-builds.
// Validate their executable digests too, without executing foreign binaries.
func TestPreparedCrossPlatformArchives(t *testing.T) {
	for _, target := range [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}, {"windows", "amd64"}} {
		t.Run(target[0]+"-"+target[1], func(t *testing.T) {
			a, err := assetFor(target[0], target[1])
			if err != nil {
				t.Fatal(err)
			}
			archive, err := os.ReadFile(filepath.Join("assets", "mise-"+target[0]+"-"+target[1]+"."+a.Format))
			if os.IsNotExist(err) {
				t.Skip("cross-platform resources are prepared by task sync-mise-all")
			}
			if err != nil {
				t.Fatal(err)
			}
			i := installer{root: t.TempDir(), archive: archive}
			if _, err := i.ensure(context.Background(), a); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type countWriter struct{ atomic.Int32 }

func (w *countWriter) Write(p []byte) (int, error) { w.Add(1); return len(p), nil }
