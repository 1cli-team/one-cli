//go:build unix

package cli_test

import (
	"fmt"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
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
	finished atomic.Bool
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
	go func() { err := cmd.Wait(); tt.finished.Store(true); tt.done <- err }()
	t.Cleanup(func() {
		if !tt.finished.Load() {
			_ = process.StopTree(cmd.Process)
		}
		_ = pt.Close()
	})
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

func TestE2E_BuildRawRejectsMultipleCommandsBeforeStarting(t *testing.T) {
	root := devTerminalFixture(t, false)
	buildWrite(t, root, "packages/lib/build.sh", "touch SHOULD_NOT_RUN\n")
	tt := startTaskTerminal(t, root, "build", "--ui=raw", "-o", "text")
	tt.wait(t, 1)
	if _, err := os.Stat(filepath.Join(root, "packages/lib/SHOULD_NOT_RUN")); !os.IsNotExist(err) {
		t.Fatal("raw rejection launched a command")
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
			buildWrite(t, root, "packages/lib/dev.sh", "echo $$ > ../../lib-dev.pid; sleep 60 & echo $! > ../../lib-child.pid; wait\n")
			buildWrite(t, root, "apps/web/dev.sh", "echo TUI_WEB_READY; sleep 60\n")
			tt := startTaskTerminal(t, root, "run", "dev", "--ui", mode, "-o", "text")
			deadline := time.Now().Add(8 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Stat(filepath.Join(root, "lib-child.pid")); err == nil {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if _, err := os.Stat(filepath.Join(root, "lib-child.pid")); err != nil {
				t.Fatal("task did not start", err)
			}
			waitForTTYOutput(t, tt.out, "\x1b[?1049h", 8*time.Second)
			if !strings.Contains(tt.out.String(), "\x1b[?1049h") {
				t.Fatalf("TUI did not enter alternate screen: %s", tt.out.String())
			}
			if err := pty.Setsize(tt.pty, &pty.Winsize{Rows: 12, Cols: 45}); err != nil {
				t.Fatal(err)
			}
			_, _ = tt.pty.Write([]byte("\t\x1b[5~"))
			time.Sleep(200 * time.Millisecond)
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
	if !strings.Contains(tt.out.String(), "\x1b[?1049l") {
		t.Fatal(tt.out.String())
	}
}

func TestE2E_TaskTUIFullHistoryAndResize(t *testing.T) {
	root := devTerminalFixture(t, false)
	buildWrite(t, root, "packages/lib/build.sh", `printf '\033[1;35mFIRST_HISTORY_MARKER\033[0m\n'
 i=0
 while [ "$i" -lt 12000 ]; do
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
	if !strings.Contains(recent, "FIRST_HISTORY_MARKER") {
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
			for _, want := range []string{"\x1b[?1049h", "\x1b[?1049l"} {
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
	waitForTaskText(t, tt.out, "● tree-api", 8*time.Second)
	waitForTaskText(t, tt.out, "● tree-web", 8*time.Second)
	waitForTTYOutput(t, tt.out, "Task dependency tree", 5*time.Second)
	// Reveal shared dependencies in the tree with sidebar arrow navigation.
	_, _ = tt.pty.Write([]byte("\t\x1b[F\x1b[C"))
	waitForTTYOutput(t, tt.out, "tree-prepare", 5*time.Second)
	waitForTTYOutput(t, tt.out, "↪", 5*time.Second)
	time.Sleep(200 * time.Millisecond)
	if err := pty.Setsize(tt.pty, &pty.Winsize{Rows: 18, Cols: 45}); err != nil {
		t.Fatal(err)
	}

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

func TestE2E_TaskTUISelectionFreezesLogsWhileProcessesContinue(t *testing.T) {
	root := devTerminalFixture(t, false)
	appendRootTaskConfig(t, root, `[tasks.copy-demo]
 run="sh copy.sh"
 `)
	buildWrite(t, root, "copy.sh", "printf 'COPY_BEFORE\\n'\nwhile [ ! -f next ]; do sleep .05; done\nprintf 'COPY_AFTER\\n'\ntouch produced\nsleep 60\n")
	tt := startTaskTerminal(t, root, "run", "copy-demo", "--ui", "tui", "-o", "text")
	waitForTTYOutput(t, tt.out, "COPY_BEFORE", 8*time.Second)
	_, _ = tt.pty.Write([]byte("c"))
	waitForTTYOutput(t, tt.out, "Native copy", 5*time.Second)
	waitForTTYOutput(t, tt.out, "\x1b[?1002l", 5*time.Second)
	start := len(tt.out.String())
	os.WriteFile(filepath.Join(root, "next"), nil, 0600)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "produced")); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(root, "produced")); err != nil {
		t.Fatal("copy mode stopped child")
	}
	if strings.Contains(tt.out.String()[start:], "COPY_AFTER") {
		t.Fatal("native snapshot followed new logs")
	}
	// Returning from the frozen screen restores live logs and mouse reporting.
	_, _ = tt.pty.Write([]byte("\x1b"))
	time.Sleep(300 * time.Millisecond)
	_, _ = tt.pty.Write([]byte("f"))

	if err := pty.Setsize(tt.pty, &pty.Winsize{Rows: 28, Cols: 100}); err != nil {
		t.Fatal(err)
	}
	waitForTTYOutput(t, tt.out, "COPY_AFTER", 5*time.Second)

	_, _ = tt.pty.Write([]byte{3})
	tt.wait(t, 130)
}

func TestE2E_TaskTUISilentRunningAndSelectedCompletion(t *testing.T) {
	root := devTerminalFixture(t, false)
	appendRootTaskConfig(t, root, `[tasks.silent-demo]
 depends=["silent"]
 run="echo STILL_RUNNING; sleep 60"
 [tasks.silent]
 run="sh silent.sh"
 `)
	buildWrite(t, root, "silent.sh", "touch started\nwhile [ ! -f finish ]; do sleep .05; done\n")
	tt := startTaskTerminal(t, root, "run", "silent-demo", "--ui", "tui", "-o", "text")
	waitForTaskText(t, tt.out, "● silent", 8*time.Second)
	_, _ = tt.pty.Write([]byte("]"))
	waitForTTYOutput(t, tt.out, "//:silent ·", 5*time.Second)
	if err := os.WriteFile(filepath.Join(root, "finish"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	waitForTaskText(t, tt.out, "completed", 5*time.Second)
	if tt.finished.Load() {
		t.Fatal("selected task completion stopped whole invocation")
	}
	_, _ = tt.pty.Write([]byte{3})
	tt.wait(t, 130)
}

// Bubble Tea redraws only changed cells; don't expect complete updated lines
// to appear contiguously in the raw PTY byte stream.
func waitForTaskText(t *testing.T, output *lockedBuffer, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(ansi.Strip(output.String()), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("missing task text %q", want)
}

func TestE2E_TaskTUICopiesThroughLocalClipboardBackend(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS clipboard backend fixture")
	}
	root := devTerminalFixture(t, false)
	for _, name := range []string{"SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY"} {
		t.Setenv(name, "")
	}
	buildWrite(t, root, "tools/pbcopy", "#!/bin/sh\ncat > "+"\""+filepath.Join(root, "copied")+"\""+"\n")
	buildWrite(t, root, "copy-backend.sh", "printf '\033[32mCOPY_中文🧑‍💻é\033[0m\n'\nsleep 60\n")
	appendRootTaskConfig(t, root, `[tasks.copy-backend]
run="sh copy-backend.sh"
`)

	tt := startTaskTerminal(t, root, "run", "copy-backend", "--ui", "tui", "-o", "text")
	waitForTTYOutput(t, tt.out, "COPY_", 8*time.Second)
	_, _ = tt.pty.Write([]byte("yl"))
	waitForTTYOutput(t, tt.out, "Copied to clipboard", 5*time.Second)
	copied, err := os.ReadFile(filepath.Join(root, "copied"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(copied), "COPY_中文🧑‍💻é") || strings.Contains(string(copied), "\x1b") || strings.Contains(string(copied), "Task dependency") {
		t.Fatalf("clipboard content mismatch: %q", copied)
	}
	_, _ = tt.pty.Write([]byte{3})
	tt.wait(t, 130)
}
