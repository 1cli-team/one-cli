package updatecheck

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testArchive(t *testing.T, goos string, names []string, body []byte, symlink bool) []byte {
	t.Helper()
	var out bytes.Buffer
	if goos == "windows" {
		writer := zip.NewWriter(&out)
		for _, name := range names {
			header := &zip.FileHeader{Name: name, Method: zip.Deflate}
			mode := os.FileMode(0o755)
			if symlink {
				mode |= os.ModeSymlink
			}
			header.SetMode(mode)
			entry, err := writer.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write(body); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	} else {
		compressed := gzip.NewWriter(&out)
		writer := tar.NewWriter(compressed)
		for _, name := range names {
			header := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}
			if symlink {
				header.Typeflag, header.Linkname, header.Size = tar.TypeSymlink, "elsewhere", 0
			}
			if err := writer.WriteHeader(header); err != nil {
				t.Fatal(err)
			}
			if !symlink {
				if _, err := writer.Write(body); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := compressed.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return out.Bytes()
}

func TestAutomaticUpdateInstallsOnlyVerifiedReleases(t *testing.T) {
	for _, tc := range []struct{ name, goos, latest, failure string }{
		{"tar success", "linux", "v1.2.3", ""},
		{"zip success", "windows", "v1.2.3", ""},
		{"same version", "linux", "v1.0.0", ""},
		{"older version", "linux", "v0.9.0", ""},
		{"prerelease feed", "linux", "v1.2.3-rc.1", "prerelease"},
		{"checksum mismatch", "linux", "v1.2.3", "checksum"},
		{"missing checksum", "linux", "v1.2.3", "missing-checksum"},
		{"duplicate checksum", "linux", "v1.2.3", "duplicate-checksum"},
		{"download unavailable", "linux", "v1.2.3", "download"},
		{"wrong executable version", "linux", "v1.2.3", "probe"},
		{"concurrent developer build", "linux", "v1.2.3", "changed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "one")
			old, candidate := []byte("installed release"), []byte("verified new release")
			if err := os.WriteFile(target, old, 0o700); err != nil {
				t.Fatal(err)
			}
			expected, err := executableDigest(target)
			if err != nil {
				t.Fatal(err)
			}
			binaryName, ext := "one", ".tar.gz"
			if tc.goos == "windows" {
				binaryName, ext = "one.exe", ".zip"
			}
			name := "one-cli_" + tc.goos + "_amd64" + ext
			archive := testArchive(t, tc.goos, []string{binaryName}, candidate, false)
			digest := fmt.Sprintf("%x", sha256.Sum256(archive))
			probed, waited, downloaded := false, false, false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/latest":
					io.WriteString(w, tc.latest)
				case "/v1.2.3/checksums.txt":
					entry := digest + "  " + name + "\n"
					switch tc.failure {
					case "checksum":
						entry = strings.Repeat("0", 64) + "  " + name + "\n"
					case "missing-checksum":
						entry = digest + "  other.tar.gz\n"
					case "duplicate-checksum":
						entry += entry
					}
					io.WriteString(w, entry)
				case "/v1.2.3/" + name:
					downloaded = true
					if tc.failure == "download" {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					w.Write(archive)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			u := updater{client: server.Client(), latestURL: server.URL + "/latest", releaseURL: server.URL, goos: tc.goos, goarch: "amd64",
				probe: func(_ context.Context, path, version string) error {
					probed = true
					raw, err := os.ReadFile(path)
					if err != nil || !bytes.Equal(raw, candidate) || version != tc.latest {
						t.Fatalf("candidate = %q, version = %s, err = %v", raw, version, err)
					}
					if tc.failure == "probe" {
						return fmt.Errorf("unexpected version")
					}
					if tc.failure == "changed" {
						return os.WriteFile(target, []byte("new developer build"), 0o700)
					}
					return nil
				}, beforeInstall: func(string) error { waited = true; return nil },
			}
			_, installed, err := u.update(context.Background(), target, "1.0.0", expected)
			if (err != nil) != (tc.failure != "") {
				t.Fatalf("error = %v", err)
			}
			raw, readErr := os.ReadFile(target)
			if readErr != nil {
				t.Fatal(readErr)
			}
			want := old
			success := tc.latest == "v1.2.3" && tc.failure == ""
			if success {
				want = candidate
				if installed != tc.latest || !probed || !waited {
					t.Fatalf("update not completed: %q %v %v", installed, probed, waited)
				}
			} else if installed != "" {
				t.Fatalf("failed/skipped update reported installed %s", installed)
			}
			if tc.failure == "changed" {
				want = []byte("new developer build")
			}
			if !bytes.Equal(raw, want) {
				t.Fatalf("installed content = %q, want %q", raw, want)
			}
			if (tc.latest == "v1.0.0" || tc.latest == "v0.9.0") && (probed || downloaded) {
				t.Fatal("same/older release was downloaded")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary downloads leaked: %v %v", entries, err)
			}
		})
	}
}

func TestUpdateArchiveRejectsUnsafeOrAmbiguousExecutables(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		name := "one"
		if goos == "windows" {
			name = "one.exe"
		}
		for _, tc := range []struct {
			name    string
			names   []string
			symlink bool
		}{
			{"path traversal", []string{"../" + name}, false},
			{"duplicate", []string{name, name}, false},
			{"symlink", []string{name}, true},
			{"missing", []string{"README.md"}, false},
		} {
			t.Run(goos+"/"+tc.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "archive")
				if err := os.WriteFile(path, testArchive(t, goos, tc.names, []byte("candidate"), tc.symlink), 0o600); err != nil {
					t.Fatal(err)
				}
				var out bytes.Buffer
				if err := extractExecutable(path, goos, &out); err == nil {
					t.Fatal("invalid archive accepted")
				}
			})
		}
	}
}

func TestUpdateSkipsDevelopmentVersionsBeforeNetwork(t *testing.T) {
	u := updater{} // accessing the client or probe would panic
	for _, version := range developmentVersions {
		if _, installed, err := u.update(context.Background(), "unused", version, "unused"); err == nil || installed != "" {
			t.Fatalf("%q: %q %v", version, installed, err)
		}
	}
}
