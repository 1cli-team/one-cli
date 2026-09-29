//go:build unix

package cli_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
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
	// The fixture owns explicit native mise tasks, including the aggregate.
	raw, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatal(err)
	}
	extra := `
[tasks.dev]
depends=["web:dev","lib:dev"]
[tasks."web:dev"]
dir="apps/web"
run="sh dev.sh"
raw_args=true
depends=["lib:build"]
[tasks."lib:dev"]
dir="packages/lib"
run="sh dev.sh"
raw_args=true
`
	buildWrite(t, root, "mise.toml", string(raw)+extra)
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
	appendRootTaskConfig(t, root, `[tasks."lib:dev"]
dir = "packages/lib"
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

func TestE2E_TaskTUIRestoresTerminalAndCancelsTree(t *testing.T) {
	for _, mode := range []string{"auto", "tui"} {
		t.Run(mode, func(t *testing.T) {
			root := devTerminalFixture(t, false)
			buildWrite(t, root, "packages/lib/dev.sh", "echo TUI_CHILD_READY; sleep 60\n")
			buildWrite(t, root, "apps/web/dev.sh", "echo TUI_WEB_READY; sleep 60\n")
			tt := startTaskTerminal(t, root, "run", "dev", "--ui", mode, "-o", "text")
			waitForTTYOutput(t, tt.out, "TUI_CHILD_READY", 8*time.Second)
			if !strings.Contains(tt.out.String(), "\x1b[?1049h") {
				t.Fatalf("TUI did not enter alternate screen: %s", tt.out.String())
			}
			if err := pty.Setsize(tt.pty, &pty.Winsize{Rows: 12, Cols: 45}); err != nil {
				t.Fatal(err)
			}
			_, _ = tt.pty.Write([]byte("\x1b[B/TUI\r\x1b[5~f"))
			_, _ = tt.pty.Write([]byte{3})
			tt.wait(t, 130)
			if !strings.Contains(tt.out.String(), "\x1b[?1049l") {
				t.Fatalf("TUI did not restore terminal: %s", tt.out.String())
			}
		})
	}
}
func TestE2E_TaskTUIKeepsChildFailure(t *testing.T) {
	root := devTerminalFixture(t, false)
	buildWrite(t, root, "packages/lib/build.sh", "echo TUI_FAILURE; exit 42\n")
	tt := startTaskTerminal(t, root, "run", "build", "-p", "lib", "--ui", "tui", "-o", "text")
	tt.wait(t, 42)
	if !strings.Contains(tt.out.String(), "TUI_FAILURE") || !strings.Contains(tt.out.String(), "\x1b[?1049l") {
		t.Fatal(tt.out.String())
	}
}

func TestE2E_TaskTUIFullHistoryStylesAndResize(t *testing.T) {
	root := devTerminalFixture(t, false)
	buildWrite(t, root, "packages/lib/build.sh", `printf '\033[1;35mFIRST_HISTORY_MARKER\033[0m\n'
 i=0
 while [ "$i" -lt 6000 ]; do
  printf 'line %s 中文 long log content that must wrap and stay complete........................................................\n' "$i"
  i=$((i+1))
 done
 printf 'LAST_HISTORY_MARKER\n'
 sleep 60
 `)
	tt := startTaskTerminal(t, root, "run", "build", "-p", "lib", "--ui", "tui", "-o", "text")
	waitForTTYOutput(t, tt.out, "LAST_HISTORY_MARKER", 15*time.Second)
	// Browse all history while the task is still running, including logs that
	// exceed the former 4,000-line buffer.
	mark := len(tt.out.String())
	_, _ = tt.pty.Write([]byte("\x1b[H"))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(tt.out.String()[mark:], "FIRST_HISTORY_MARKER") {
		time.Sleep(20 * time.Millisecond)
	}
	recent := tt.out.String()[mark:]
	if !strings.Contains(recent, "FIRST_HISTORY_MARKER") || !strings.Contains(ansi.Strip(recent), "[//:lib:build]") || !strings.Contains(recent, "35") {
		t.Fatalf("history/prefix/style lost: %q", recent)
	}
	for _, size := range []pty.Winsize{{Rows: 12, Cols: 45}, {Rows: 30, Cols: 140}} {
		if err := pty.Setsize(tt.pty, &size); err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Mouse scrolling, keyboard paging and returning to the live tail.
	_, _ = tt.pty.Write([]byte("\x1b[<65;35;5M\x1b[6~\x1b[F"))
	waitForTTYOutput(t, tt.out, "LAST_HISTORY_MARKER", 5*time.Second)
	_, _ = tt.pty.Write([]byte{3})
	tt.wait(t, 130)
	if !strings.Contains(tt.out.String(), "\x1b[?1049l") {
		t.Fatal("terminal was not restored")
	}
}

func TestE2E_InstallTaskTUIExitsWithoutKeyboardInput(t *testing.T) {
	for _, mode := range []string{"auto", "tui"} {
		t.Run(mode, func(t *testing.T) {
			root := devTerminalFixture(t, false)
			appendRootTaskConfig(t, root, `[tasks.install]
 depends=["web:build"]
 run="echo INSTALL_FINISHED"
 `)
			tt := startTaskTerminal(t, root, "install", "--ui", mode, "-o", "text")
			// Do not send q, Enter, Ctrl+C, or close stdin: completion alone must exit.
			tt.wait(t, 0)
			got := tt.out.String()
			for _, want := range []string{"INSTALL_FINISHED", "\x1b[?1049h", "\x1b[?1049l"} {
				if !strings.Contains(got, want) {
					t.Fatalf("missing %q in %s", want, got)
				}
			}
		})
	}
}

func TestE2E_TaskTUIDependencyTree(t *testing.T) {
	root := devTerminalFixture(t, false)
	appendRootTaskConfig(t, root, `[tasks.tree-demo]
 depends=["tree-api","tree-web"]
 [tasks.tree-api]
 depends=["tree-prepare"]
 run="echo TREE_API_READY; sleep 60"
 [tasks.tree-web]
 depends=["tree-prepare"]
 run="echo TREE_WEB_READY; sleep 60"
 [tasks.tree-prepare]
 run="echo TREE_SHARED_OUTPUT"
 `)
	tt := startTaskTerminal(t, root, "run", "tree-demo", "--ui", "tui", "-o", "text")
	for _, want := range []string{"TREE_API_READY", "TREE_WEB_READY", "├─ ▾ tree-api", "└─ ↪ tree-prepare"} {
		waitForTTYOutput(t, tt.out, want, 8*time.Second)
	}
	_, _ = tt.pty.Write([]byte("\t\x1b[B\x1b[D")) // focus tree and collapse entry
	waitForTTYOutput(t, tt.out, "▸ tree-demo", 5*time.Second)
	_, _ = tt.pty.Write([]byte("\x1b[C\x1b[F")) // expand and select shared reference
	waitForTTYOutput(t, tt.out, "Shared dependency", 5*time.Second)
	if err := pty.Setsize(tt.pty, &pty.Winsize{Rows: 18, Cols: 45}); err != nil {
		t.Fatal(err)
	}
	_, _ = tt.pty.Write([]byte("\r\t")) // jump to primary occurrence and show its logs
	waitForTTYOutput(t, tt.out, "[//:tree-prepare]", 5*time.Second)
	_, _ = tt.pty.Write([]byte{3})
	tt.wait(t, 130)
	if !strings.Contains(tt.out.String(), "\x1b[?1049l") {
		t.Fatal("tree view did not restore terminal")
	}
}

func TestE2E_TaskListTerminalStyles(t *testing.T) {
	for _, color := range []bool{true, false} {
		t.Run(fmt.Sprint(color), func(t *testing.T) {
			root := buildFixture(t, true)
			t.Setenv("FORCE_COLOR", "")
			if color {
				t.Setenv("CLICOLOR", "")
			} else {
				t.Setenv("CLICOLOR", "0")
			}
			tt := startTaskTerminal(t, root, "run")
			tt.wait(t, 0)
			got := tt.out.String()
			// Terminal capability probes are not visible SGR color/style sequences.
			if regexp.MustCompile(`\x1b\[[0-9;]*m`).MatchString(got) != color || strings.Contains(got, "\x1b[?1049h") {
				t.Fatalf("unexpected task list styling: %q", got)
			}
			for _, want := range []string{"Workspace tasks", "Project: web", "web:build", "one run --verbose"} {
				if !strings.Contains(ansi.Strip(got), want) {
					t.Fatalf("missing %q: %s", want, got)
				}
			}
		})
	}
}
