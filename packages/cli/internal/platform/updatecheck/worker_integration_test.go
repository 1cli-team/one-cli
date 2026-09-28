package updatecheck

import (
	"context"
	"crypto/sha256"
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
		if runWorker("1.0.0", os.Args[1:], func() updater {
			u := defaultUpdater()
			u.client = http.DefaultClient
			u.latestURL = os.Getenv("ONE_UPDATE_TEST_SERVER") + "/latest"
			u.releaseURL = os.Getenv("ONE_UPDATE_TEST_SERVER")
			return u
		}) {
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
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
	t.Fatal("update did not complete after the initiating command exited")
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
