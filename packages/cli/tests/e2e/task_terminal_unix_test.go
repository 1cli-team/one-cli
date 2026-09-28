//go:build unix

package cli_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

type taskTerminal struct {
	cmd      *exec.Cmd
	pty      *os.File
	out      *lockedBuffer
	done     chan error
	readDone chan struct{}
}

func startTaskTerminal(t *testing.T, root string, args ...string) *taskTerminal {
	t.Helper()
	cmd := exec.Command(binaryPath(t), args...)
	cmd.Dir = root
	cmd.Env = replaceEnvValues(prependPath(os.Environ(), filepath.Dir(binaryPath(t))), map[string]string{"TERM": "xterm-256color", "CI": "", "NO_COLOR": ""})
	pt, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 110})
	if err != nil {
		t.Fatal(err)
	}
	tt := &taskTerminal{cmd: cmd, pty: pt, out: &lockedBuffer{}, done: make(chan error, 1), readDone: make(chan struct{})}
	go func() {
		defer close(tt.readDone)
		// Lip Gloss stops reading once it gets the device-attributes reply.
		// Send the background reply first so it cannot leak into child stdin.
		replies := []struct{ query, response string }{
			{"\x1b]11;?", "\x1b]11;rgb:0000/0000/0000\x1b\\"},
			{"\x1b[c", "\x1b[?1;2c"},
			{"\x1b[6n", "\x1b[1;1R"},
		}
		counts := map[string]int{}
		buf := make([]byte, 8192)
		for {
			n, err := pt.Read(buf)
			if n > 0 {
				_, _ = tt.out.Write(buf[:n])
				all := tt.out.String()
				for _, reply := range replies {
					count := strings.Count(all, reply.query)
					for counts[reply.query] < count {
						_, _ = io.WriteString(pt, reply.response)
						counts[reply.query]++
					}
				}
			}
			if err != nil {
				return
			}
		}
	}()
	go func() { tt.done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = pt.Close() })
	return tt
}
func (tt *taskTerminal) wait(t *testing.T, want int) {
	t.Helper()
	select {
	case err := <-tt.done:
		code := 0
		if ex, ok := err.(*exec.ExitError); ok {
			code = ex.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		if code != want {
			t.Fatalf("exit=%d want=%d\n%s", code, want, tt.out.String())
		}
	case <-time.After(8 * time.Second):
		t.Fatalf("terminal did not finish\n%s", tt.out.String())
	}
	select {
	case <-tt.readDone:
	case <-time.After(time.Second):
		_ = tt.pty.Close()
	}
}
func devTerminalFixture(t *testing.T, mise bool) string {
	root := buildFixture(t, mise)
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "one.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err = json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, v := range m["projects"].([]any) {
		p := v.(map[string]any)
		if p["name"] != "mobile" {
			p["dev"] = map[string]any{"command": "sh dev.sh"}
		}
	}
	raw, _ = json.Marshal(m)
	buildWrite(t, root, "one.manifest.json", string(raw))
	return root
}
func TestE2E_SingleDevAndBuildKeepNativeTTY(t *testing.T) {
	for _, command := range []string{"dev", "build"} {
		t.Run(command, func(t *testing.T) {
			root := devTerminalFixture(t, false)
			script := "test -t 0 && test -t 1 && test -t 2 || exit 91\nprintf '\\033[35mNATIVE_READY\\033[0m'\nread value\nprintf '\\rNATIVE_REPLY:%s\\n' \"$value\"\n"
			buildWrite(t, root, "packages/lib/"+command+".sh", script)
			args := []string{command, "-p", "lib", "-o", "text", "--ui", "raw"}
			tt := startTaskTerminal(t, root, args...)
			waitForTTYOutput(t, tt.out, "NATIVE_READY", 8*time.Second)
			_, _ = tt.pty.Write([]byte("hello\r"))
			tt.wait(t, 0)
			got := tt.out.String()
			if !strings.Contains(got, "\x1b[35mNATIVE_READY\x1b[0m") || !strings.Contains(got, "NATIVE_REPLY:hello") || strings.Contains(got, "lib | NATIVE") || strings.Contains(got, "[lib] NATIVE") {
				t.Fatal(got)
			}
		})
	}
}

func TestE2E_SingleDevAndBuildExitWithIdleTerminalInput(t *testing.T) {
	for _, command := range []string{"dev", "build"} {
		for _, tc := range []struct {
			name string
			code int
		}{{"success", 0}, {"failure", 42}} {
			t.Run(command+"/"+tc.name, func(t *testing.T) {
				root := devTerminalFixture(t, false)
				// The input relay remains blocked while the child finishes without
				// reading stdin. Exiting must cancel that read and keep the exit code.
				script := fmt.Sprintf("test -t 0 && test -t 1 && test -t 2 || exit 91\nprintf 'NATIVE_DONE\\n'\nexit %d\n", tc.code)
				buildWrite(t, root, "packages/lib/"+command+".sh", script)
				args := []string{command, "-p", "lib", "-o", "text", "--ui", "raw"}
				tt := startTaskTerminal(t, root, args...)
				tt.wait(t, tc.code)
				if !strings.Contains(tt.out.String(), "NATIVE_DONE") {
					t.Fatal(tt.out.String())
				}
			})
		}
	}
}

func TestE2E_BuildRawKeepsFailure(t *testing.T) {
	root := devTerminalFixture(t, false)
	buildWrite(t, root, "packages/lib/build.sh", "test -t 1 || exit 91\nprintf '\\033[31mBUILD_FAILURE\\033[0m\\n'\nexit 42\n")
	tt := startTaskTerminal(t, root, "build", "--ui=raw", "-o", "text")
	tt.wait(t, 42)
	got := tt.out.String()
	if !strings.Contains(got, "BUILD_FAILURE") {
		t.Fatal(got)
	}
}

func TestE2E_DevHonorsMiseProjectOverrideAndRawTerminal(t *testing.T) {
	root := devTerminalFixture(t, true)
	buildWrite(t, root, "packages/lib/mise.toml", `[tasks.dev]
run = "sh override.sh"
raw = true
`)
	buildWrite(t, root, "packages/lib/override.sh", "test -t 0 && test -t 1 && test -t 2 || exit 91\necho USER_DEV_READY\nread value\necho USER_DEV_REPLY:$value\n")
	buildWrite(t, root, "packages/lib/dev.sh", "echo WRONG_MANIFEST_COMMAND\nexit 92\n")
	tt := startTaskTerminal(t, root, "dev", "-p", "lib", "-o", "text")
	waitForTTYOutput(t, tt.out, "USER_DEV_READY", 8*time.Second)
	_, _ = tt.pty.Write([]byte("hello\r"))
	tt.wait(t, 0)
	if !strings.Contains(tt.out.String(), "USER_DEV_REPLY:hello") || strings.Contains(tt.out.String(), "WRONG_MANIFEST_COMMAND") {
		t.Fatal(tt.out.String())
	}
}
