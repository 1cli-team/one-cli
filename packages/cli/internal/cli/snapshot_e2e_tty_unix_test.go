//go:build !windows

package cli_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestE2E_CIDisableCtrlCIsQuietInTTY(t *testing.T) {
	for _, term := range []string{"ansi", "dumb"} {
		t.Run(term, func(t *testing.T) {
			testCIDisableCtrlCIsQuietInTTY(t, term)
		})
	}
}

func testCIDisableCtrlCIsQuietInTTY(t *testing.T, term string) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "demo")

	if _, stderr, code := runBinaryIn(t, ws, "add", "react-spa", "--name", "web", "--yes", "-o", "json"); code != 0 {
		t.Fatalf("add failed: exit=%d stderr=%s", code, stderr)
	}
	if _, stderr, code := runBinaryIn(t, ws, "ci", "enable", "web", "-o", "json"); code != 0 {
		t.Fatalf("ci enable failed: exit=%d stderr=%s", code, stderr)
	}

	workflow := filepath.Join(ws, ".github", "workflows", "ci-apps-web.yml")
	bin := binaryPath(t)
	cmd := exec.Command(bin, "ci", "disable", "web", "-o", "text")
	cmd.Dir = ws
	cmd.Env = replaceEnvValues(prependPath(os.Environ(), filepath.Dir(bin)), map[string]string{
		"LC_ALL":   "C",
		"LANG":     "C",
		"NO_COLOR": "1",
		"TERM":     term,
	})

	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("start command in PTY: %v", err)
	}
	defer ptmx.Close()
	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 120}); err != nil {
		t.Fatalf("set PTY size: %v", err)
	}

	var output lockedBuffer
	readDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&output, ptmx)
		close(readDone)
	}()

	if term != "dumb" {
		waitForTTYOutput(t, &output, "\x1b]11;?", 5*time.Second)
		if _, err := ptmx.Write([]byte("\x1b]11;rgb:0000/0000/0000\x1b\\")); err != nil {
			t.Fatalf("reply to terminal background query: %v", err)
		}
		waitForTTYOutput(t, &output, "\x1b[6n", 5*time.Second)
		if _, err := ptmx.Write([]byte("\x1b[1;1R")); err != nil {
			t.Fatalf("reply to terminal cursor query: %v", err)
		}
	}
	waitForTTYOutput(t, &output, "Disable continuous integration", 5*time.Second)
	if _, err := ptmx.Write([]byte{3}); err != nil {
		t.Fatalf("send Ctrl-C: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("Ctrl-C should exit successfully, got %v\noutput:\n%s", err, output.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("command did not exit after Ctrl-C")
	}

	_ = ptmx.Close()
	select {
	case <-readDone:
	case <-time.After(time.Second):
	}

	if got := output.String(); strings.Contains(got, "✗") {
		t.Fatalf("user cancellation must not use error styling:\n%s", got)
	}
	if !fileExists(t, workflow) {
		t.Fatal("Ctrl-C must not remove the CI workflow")
	}
}

func TestE2E_EnvSetHidesValueInAccessibleTTY(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "demo")
	if _, stderr, code := runBinaryIn(t, ws, "add", "react-spa", "--name", "web", "--yes", "-o", "json"); code != 0 {
		t.Fatalf("add failed: exit=%d stderr=%s", code, stderr)
	}

	bin := binaryPath(t)
	cmd := exec.Command(bin, "env", "set", "TEST_KEY", "-o", "text")
	cmd.Dir = ws
	cmd.Env = replaceEnvValues(prependPath(os.Environ(), filepath.Dir(bin)), map[string]string{
		"LC_ALL":   "C",
		"LANG":     "C",
		"NO_COLOR": "1",
		"TERM":     "dumb",
	})

	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("start command in PTY: %v", err)
	}
	defer ptmx.Close()

	var output lockedBuffer
	readDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&output, ptmx)
		close(readDone)
	}()

	waitForTTYOutput(t, &output, "Enter the value for TEST_KEY", 5*time.Second)
	const secret = "not-for-terminal-scrollback"
	if _, err := ptmx.Write([]byte(secret + "\r")); err != nil {
		t.Fatalf("enter secret: %v", err)
	}
	waitForTTYOutput(t, &output, "Where does this variable belong?", 5*time.Second)
	if _, err := ptmx.Write([]byte("1\r")); err != nil {
		t.Fatalf("select workspace scope: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("env set failed: %v\noutput:\n%s", err, output.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("env set did not finish after choosing a scope")
	}
	_ = ptmx.Close()
	select {
	case <-readDone:
	case <-time.After(time.Second):
		t.Fatal("PTY reader did not finish after env set exited")
	}

	if got := output.String(); strings.Contains(got, secret) {
		t.Fatalf("secret value leaked into terminal output:\n%s", got)
	}
	body, err := os.ReadFile(filepath.Join(ws, ".env.dev"))
	if err != nil {
		t.Fatalf("read workspace environment file: %v", err)
	}
	if !strings.Contains(string(body), "TEST_KEY="+secret) {
		t.Fatalf("workspace environment file did not receive TEST_KEY: %q", body)
	}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func waitForTTYOutput(t *testing.T, output *lockedBuffer, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(output.String(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in PTY output:\n%s", want, output.String())
}

func replaceEnvValues(env []string, values map[string]string) []string {
	result := make([]string, 0, len(env)+len(values))
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		if _, replace := values[key]; !replace {
			result = append(result, kv)
		}
	}
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}
