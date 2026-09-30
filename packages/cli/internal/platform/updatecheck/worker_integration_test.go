package updatecheck

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// The copied test executable acts as the release binary in the subprocess
// test. Endpoint overrides and version replies exist only in this test entry.
func TestMain(m *testing.M) {
	if os.Getenv("ONE_UPDATE_TEST_INTEGRATION") == "1" {
		if len(os.Args) == 2 && os.Args[1] == "--version" {
			fmt.Println("1.2.3")
			os.Exit(0)
		}
		buildChannel = "release"
		makeUpdater := func() updater {
			u := defaultUpdater()
			u.client = http.DefaultClient
			u.latestURL = os.Getenv("ONE_UPDATE_TEST_SERVER") + "/latest"
			u.releaseURL = os.Getenv("ONE_UPDATE_TEST_SERVER")
			return u
		}
		if runManualWorker("1.0.0", os.Args[1:], makeUpdater) || runWorker("1.0.0", os.Args[1:], makeUpdater) {
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func TestManualWorkerChecksImmediatelyAndReportsResult(t *testing.T) {
	for _, tc := range []struct {
		name, latest string
		fail         bool
		pathAlias    bool
	}{
		{"upgrade", "v1.2.3", false, false},
		{"already current", "v1.0.0", false, false},
		{"no downgrade", "v0.9.0", false, false},
		{"checksum failure", "v1.2.3", true, false},
		{"path alias", "v1.2.3", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withIsolatedCache(t)
			target := filepath.Join(t.TempDir(), executableName())
			if err := copyExecutable(os.Args[0], target); err != nil {
				t.Fatal(err)
			}
			if tc.pathAlias {
				alias := filepath.Join(t.TempDir(), "install")
				if err := os.Symlink(filepath.Dir(target), alias); err != nil {
					t.Skipf("directory symlinks unavailable: %v", err)
				}
				target = filepath.Join(alias, executableName())
			}
			// Upgrade returns the canonical executable path. macOS /var aliases
			// and Windows short paths can differ from t.TempDir's spelling.
			canonicalTarget, err := filepath.EvalSymlinks(target)
			if err != nil {
				t.Fatal(err)
			}
			payload, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			archive := testArchive(t, runtime.GOOS, []string{executableName()}, payload, false)
			ext := ".tar.gz"
			if runtime.GOOS == "windows" {
				ext = ".zip"
			}
			name := "one-cli_" + runtime.GOOS + "_" + runtime.GOARCH + ext
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/latest":
					fmt.Fprintln(w, tc.latest)
				case "/v1.2.3/checksums.txt":
					digest := sha256.Sum256(archive)
					if tc.fail {
						digest = [32]byte{}
					}
					fmt.Fprintf(w, "%x  %s\n", digest, name)
				case "/v1.2.3/" + name:
					w.Write(archive)
				default:
					t.Errorf("unexpected download %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			// A fresh cache and CI would suppress an automatic update.
			if err := saveCache(&Cache{LastChecked: time.Now().UTC(), CurrentVersion: "1.0.0", TargetPath: canonicalTarget, Status: "current"}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			launcher := exec.CommandContext(ctx, target, "-test.run=^TestManualUpdateIntegrationLauncher$")
			launcher.Env = append(os.Environ(), "CI=true", "ONE_UPDATE_TEST_INTEGRATION=1", "ONE_UPDATE_TEST_SERVER="+server.URL)
			raw, err := launcher.CombinedOutput()
			if err != nil {
				t.Fatalf("manual command: %v %s", err, raw)
			}
			var reply manualReply
			if err := json.Unmarshal(raw, &reply); err != nil {
				t.Fatalf("manual result %q: %v", raw, err)
			}
			if (reply.Error != "") != tc.fail {
				t.Fatalf("unexpected reply: %#v", reply)
			}
			if !tc.fail {
				want := "current"
				if tc.latest == "v1.2.3" {
					want = "updated"
					if runtime.GOOS == "windows" {
						want = "pending"
					}
				}
				if reply.Result == nil || reply.Result.Status != want || reply.Result.LatestVersion != tc.latest || reply.Result.TargetPath != canonicalTarget {
					t.Fatalf("unexpected result: %#v", reply.Result)
				}
			}
			// Windows installation completes after the launcher has exited.
			deadline := time.Now().Add(10 * time.Second)
			for {
				c, err := loadCache()
				if err == nil && c != nil && c.Status != "checking" {
					want := "current"
					if tc.fail {
						want = "failed"
					} else if tc.latest == "v1.2.3" {
						want = "updated"
					}
					if c.Status != want || c.LatestVersion != tc.latest {
						t.Fatalf("completion state: %#v", c)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("manual worker did not finish: %#v %v", c, err)
				}
				time.Sleep(20 * time.Millisecond)
			}
			got, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(got, payload) {
				// The test release contains the test executable itself.
				// Every path must leave a complete, runnable program.
				t.Fatalf("installed executable differs: %v", err)
			}
		})
	}
}

func TestManualUpdateIntegrationLauncher(t *testing.T) {
	if os.Getenv("ONE_UPDATE_TEST_INTEGRATION") != "1" {
		return
	}
	result, err := Upgrade(context.Background(), "1.0.0")
	reply := manualReply{Result: result}
	if err != nil {
		reply.Error = err.Error()
	}
	_ = json.NewEncoder(os.Stdout).Encode(reply)
	os.Exit(0)
}

func TestBackgroundWorkerCompletesAfterCommandExits(t *testing.T) {
	withIsolatedCache(t)
	clearCIEnv(t)
	target := filepath.Join(t.TempDir(), executableName())
	if err := copyExecutable(os.Args[0], target); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	archive := testArchive(t, runtime.GOOS, []string{executableName()}, payload, false)
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	name := "one-cli_" + runtime.GOOS + "_" + runtime.GOARCH + ext
	// Do not let the archive request finish until the foreground command exits.
	// An in-process goroutine would die without installing this release.
	resume := make(chan struct{})
	resumed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			fmt.Fprintln(w, "v1.2.3")
		case "/v1.2.3/checksums.txt":
			fmt.Fprintf(w, "%x  %s\n", sha256.Sum256(archive), name)
		case "/v1.2.3/" + name:
			select {
			case <-resume:
			case <-r.Context().Done():
				return
			}
			w.Write(archive)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer func() {
		if !resumed {
			close(resume)
		}
		server.Close()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	launcher := exec.CommandContext(ctx, target, "-test.run=^TestUpdateIntegrationLauncher$")
	launcher.Env = append(os.Environ(), "ONE_UPDATE_TEST_INTEGRATION=1", "ONE_UPDATE_TEST_SERVER="+server.URL)
	if raw, err := launcher.CombinedOutput(); err != nil {
		t.Fatalf("foreground command: %v %s", err, raw)
	}
	close(resume)
	resumed = true
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		c, err := loadCache()
		if err == nil && c != nil {
			if c.Status == "failed" {
				t.Fatalf("background update failed: %s", c.Error)
			}
			if c.Status == "updated" {
				if c.InstalledVersion != "v1.2.3" || !c.NotificationPending {
					t.Fatalf("completion state: %#v", c)
				}
				probe := exec.Command(target, "--version")
				probe.Env = append(os.Environ(), "ONE_UPDATE_TEST_INTEGRATION=1")
				if raw, err := probe.Output(); err != nil || string(raw) != "1.2.3\n" {
					t.Fatalf("installed binary: %q %v", raw, err)
				}
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	c, err := loadCache()
	t.Fatalf("update did not complete after the initiating command exited: cache=%#v, error=%v", c, err)
}

func TestUpdateIntegrationLauncher(t *testing.T) {
	if os.Getenv("ONE_UPDATE_TEST_INTEGRATION") != "1" {
		return
	}
	target, err := os.Executable()
	if err != nil {
		os.Exit(2)
	}
	startUpdate("1.0.0", target, launchWorker)
	os.Exit(0)
}
