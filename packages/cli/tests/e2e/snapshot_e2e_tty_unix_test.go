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

func TestE2E_EnvSetHidesValueBeforeScopeCancellation(t *testing.T) {
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
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("cancel scope selection: %v", err)
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
	for _, name := range []string{".env", ".env.dev"} {
		if _, err := os.Stat(filepath.Join(ws, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected local variable file %s: %v", name, err)
		}
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
