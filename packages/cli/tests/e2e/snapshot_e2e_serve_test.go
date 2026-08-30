package cli_test

// E2E coverage of `one serve`. Locks the JSON envelope shape, validates the
// session token gates /api/*, and confirms graceful SIGINT shutdown.
//
// `serve` is the only top-level command that blocks indefinitely — every
// other snapshot test exec's the binary and reads stdout. We adapt by
// streaming stdout into a buffer until the envelope appears, then probing
// the live HTTP server, then SIGINT-ing the process.

import (
	"bytes"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestSnapshot_E2E_Serve_StartupEnvelope(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	cmd := exec.Command(binaryPath(t), "serve", "--port", "0", "--open=false", "-o", "json")
	cmd.Env = append(os.Environ(), "HOME="+tmp, "XDG_CONFIG_HOME=")

	var stdout, stderr safeBuf
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		// Belt-and-suspenders: if a sub-test fails before sending SIGINT,
		// kill the process so the test binary doesn't hang.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	// Wait up to 5s for the envelope to land on stdout. The server prints
	// it immediately after binding.
	envelope := waitForEnvelope(t, &stdout, 5*time.Second, stderr.String)
	if envelope["schema"] != "one-cli/serve/v1" {
		t.Errorf("schema: got %v", envelope["schema"])
	}
	if envelope["status"] != "listening" {
		t.Errorf("status: got %v", envelope["status"])
	}
	if envelope["host"] != "127.0.0.1" {
		t.Errorf("host: got %v", envelope["host"])
	}
	rawPort, ok := envelope["port"].(float64)
	if !ok || rawPort < 1 {
		t.Errorf("port: got %v (%T)", envelope["port"], envelope["port"])
	}
	rawURL, _ := envelope["url"].(string)
	if rawURL == "" {
		t.Fatalf("url empty: %v", envelope)
	}
	rawToken, _ := envelope["token"].(string)
	if rawToken == "" {
		t.Fatalf("token empty")
	}

	// Probe /api/configure with the token. Empty config should yield a 200
	// with the schema-shaped payload.
	port := int(rawPort)
	probe := "http://127.0.0.1:" + strconv.Itoa(port) + "/api/configure?token=" + url.QueryEscape(rawToken)
	res, err := http.Get(probe)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Errorf("probe: want 200, got %d", res.StatusCode)
	}

	// Shut down. SIGINT triggers the signal-aware ctx in cmd.go's RunE.
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("sigint: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("clean shutdown: got %v\n  stderr: %s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("server did not exit within 5s after SIGINT")
	}

	assertSnapshot(t, "serve.json", envelope)
}

// safeBuf is a bytes.Buffer with a mutex so the goroutine running the
// child process can write while the test goroutine reads. Without the
// guard the race detector flags the concurrent access.
type safeBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// waitForEnvelope polls stdout until firstJSONLine returns a complete
// envelope or the timeout expires. dumpStderr produces the stderr buffer
// for diagnostics on timeout.
func waitForEnvelope(t *testing.T, stdout *safeBuf, timeout time.Duration, dumpStderr func() string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if line := firstJSONLine(stdout.String()); line != "" {
			return mustParseJSON(t, line)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("envelope never appeared\n  stdout: %s\n  stderr: %s", stdout.String(), dumpStderr())
	return nil
}
