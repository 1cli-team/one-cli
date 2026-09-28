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
			p["domains"] = map[string]any{"dev": map[string]any{"command": "sh dev.sh"}}
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
			tt := startTaskTerminal(t, root, command, "lib", "-o", "text")
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
				tt := startTaskTerminal(t, root, command, "lib", "-o", "text")
				tt.wait(t, tc.code)
				if !strings.Contains(tt.out.String(), "NATIVE_DONE") {
					t.Fatal(tt.out.String())
				}
			})
		}
	}
}

func TestE2E_DevTUIInputResizeAndStop(t *testing.T) {
	root := devTerminalFixture(t, true)
	buildWrite(t, root, "apps/web/dev.sh", `test -t 0 && test -t 1 || exit 91
echo run >> runs
i=0
while [ "$i" -lt 60 ]; do
  printf 'LOG_%02d original project output\n' "$i"
  i=$((i + 1))
done
echo WEB_READY
while read value; do
  if [ "$value" = size ]; then stty size > terminal-size; else printf 'web:%s\n' "$value"; fi
done
`)
	buildWrite(t, root, "packages/lib/dev.sh", "echo LIB_READY\nsleep 60\n")
	tt := startTaskTerminal(t, root, "dev", "web", "lib", "--keep-going", "-o", "text")
	waitForTTYOutput(t, tt.out, "WEB_READY", 8*time.Second)
	if !strings.Contains(tt.out.String(), "\x1b[?1049h") {
		t.Fatal("TUI did not enter alternate screen")
	}
	_, _ = tt.pty.Write([]byte("\r"))
	waitForTTYOutput(t, tt.out, "INPUT", 5*time.Second)
	// Real SGR mouse input must browse history even after entering child input.
	if !strings.Contains(tt.out.String(), "\x1b[?1006h") {
		t.Fatal("TUI did not enable mouse reporting")
	}
	_, _ = tt.pty.Write([]byte("\x1b[<64;55;5M"))
	waitForTTYOutput(t, tt.out, "Shift+End", 5*time.Second)
	_, _ = tt.pty.Write([]byte("hello\r"))
	waitForTTYOutput(t, tt.out, "web:hello", 5*time.Second)
	assertChildSize := func(cols, rows, logCols int) {
		t.Helper()
		if err := pty.Setsize(tt.pty, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}); err != nil {
			t.Fatal(err)
		}
		want := fmt.Sprintf("%d %d", rows-4, logCols)
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			_, _ = tt.pty.Write([]byte("size\r"))
			raw, _ := os.ReadFile(filepath.Join(root, "apps/web/terminal-size"))
			if strings.TrimSpace(string(raw)) == want {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("child did not resize to %s", want)
	}
	assertChildSize(80, 24, 51)
	assertChildSize(60, 18, 60) // Automatically hidden sidebar uses the full width.
	assertChildSize(110, 30, 79)
	_, _ = tt.pty.Write([]byte{29})    // Ctrl+] leaves input mode.
	_, _ = tt.pty.Write([]byte("h\r")) // Hide the list, then return to child input.
	assertChildSize(110, 30, 110)
	_, _ = tt.pty.Write([]byte{29})

	_, _ = tt.pty.Write([]byte("r"))
	deadline := time.Now().Add(5 * time.Second)
	restarted := false
	for time.Now().Before(deadline) {
		raw, _ := os.ReadFile(filepath.Join(root, "apps/web/runs"))
		if strings.Count(string(raw), "run") == 2 {
			restarted = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !restarted {
		t.Fatalf("selected project did not restart: %s", tt.out.String())
	}
	_, _ = tt.pty.Write([]byte{3})
	tt.wait(t, 130)
	if !strings.Contains(tt.out.String(), "\x1b[?1049l") {
		t.Fatal("terminal was not restored")
	}
}
func TestE2E_BuildTUIAutoFinishesAndKeepsFailure(t *testing.T) {
	root := devTerminalFixture(t, false)
	buildWrite(t, root, "packages/lib/build.sh", "test -t 1 || exit 91\nprintf '\\033[31mBUILD_FAILURE\\033[0m\\n'\nexit 42\n")
	tt := startTaskTerminal(t, root, "build", "--ui=tui", "-o", "text")
	tt.wait(t, 42)
	got := tt.out.String()
	if !strings.Contains(got, "BUILD_FAILURE") || !strings.Contains(got, "blocked") || !strings.Contains(got, "\x1b[?1049l") {
		t.Fatal(got)
	}
}
