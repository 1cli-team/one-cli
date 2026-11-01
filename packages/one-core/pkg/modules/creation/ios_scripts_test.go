package creation

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// Exercise the rendered scripts without Xcode, a simulator or signing secrets.
func checkIOSTemplateScripts(t *testing.T, rendered string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	raw, err := os.ReadFile(filepath.Join(rendered, "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var taskfile struct {
		Tasks map[string]struct{ Cmds []string }
	}
	if err := yaml.Unmarshal(raw, &taskfile); err != nil {
		t.Fatal(err)
	}
	var syntax string
	for _, command := range taskfile.Tasks["check"].Cmds {
		if strings.Contains(command, "bash -n") {
			syntax = command
			break
		}
	}
	if syntax == "" {
		t.Fatal("missing syntax check")
	}
	scripts, err := filepath.Glob(filepath.Join(rendered, "scripts", "*.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, broken := range scripts {
		t.Run("syntax-"+filepath.Base(broken), func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "scripts"), 0755); err != nil {
				t.Fatal(err)
			}
			for _, script := range scripts {
				content, err := os.ReadFile(script)
				if err != nil {
					t.Fatal(err)
				}
				if script == broken {
					content = []byte("if then broken\n")
				}
				if err := os.WriteFile(filepath.Join(dir, "scripts", filepath.Base(script)), content, 0644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", "-c", syntax)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err == nil {
				t.Fatalf("syntax check accepted broken %s: %s", broken, out)
			}
		})
	}
	if runtime.GOOS != "darwin" {
		return
	} // Uses stock macOS lockf and PlistBuddy.
	t.Run("dev-session-ownership", func(t *testing.T) { checkIOSDevSessions(t, rendered) })
}

func checkIOSDevSessions(t *testing.T, rendered string) {
	t.Helper()
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
	}
	dev, err := os.ReadFile(filepath.Join(rendered, "scripts/dev.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, checkout := range []string{"first", "second"} {
		write(filepath.Join(root, checkout, "scripts/dev.sh"), string(dev))
		write(filepath.Join(root, checkout, "scripts/build.sh"), "#!/bin/bash\nexit 0\n")
		write(filepath.Join(root, checkout, "build/sample-app.app/Info.plist"), `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>test.one.ios</string></dict></plist>`)
	}
	write(filepath.Join(root, "bin/xcrun"), `#!/bin/bash
set -eu
case "$1 $2" in
 "simctl list") printf 'Review (DEVICE-A) (Booted)\nReview (DEVICE-B) (Booted)\n' ;;
 "simctl bootstatus") exit 0 ;;
 "simctl install") echo install >> "$REVIEW_LOG" ;;
 "simctl terminate") echo unexpected-bundle-termination >> "$REVIEW_LOG"; exit 1 ;;
 "simctl launch")
   printf '%s\n' "$@" > "$REVIEW_ROOT/args-$$"
   trap 'echo stopped-$$ >> "$REVIEW_LOG"; exit 0' TERM INT
   touch "$REVIEW_ROOT/ready-$$"
   while [[ ! -e "$REVIEW_ROOT/finish-$$" ]]; do sleep 0.05; done ;;
 *) exit 2 ;;
esac
`)
	log := filepath.Join(root, "log")
	env := append(os.Environ(), "PATH="+filepath.Join(root, "bin")+":"+os.Getenv("PATH"), "TMPDIR="+root, "REVIEW_ROOT="+root, "REVIEW_LOG="+log)
	command := func(checkout, device string) *exec.Cmd {
		cmd := exec.Command("bash", "scripts/dev.sh", "argument with spaces")
		cmd.Dir = filepath.Join(root, checkout)
		cmd.Env = append(append([]string{}, env...), "IOS_SIMULATOR_UDID="+device)
		return cmd
	}
	start := func(checkout, device string) (*exec.Cmd, chan error) {
		t.Helper()
		cmd := command(checkout, device)
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		t.Cleanup(func() { _ = cmd.Process.Signal(syscall.SIGTERM) })
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err := os.Stat(filepath.Join(root, fmt.Sprintf("ready-%d", cmd.Process.Pid))); err == nil {
				break
			}
			select {
			case err := <-done:
				t.Fatalf("dev %s/%s exited before readiness: %v %s", checkout, device, err, output.String())
			default:
			}
			if time.Now().After(deadline) {
				t.Fatal("dev readiness timeout")
			}
			time.Sleep(10 * time.Millisecond)
		}
		args, err := os.ReadFile(filepath.Join(root, fmt.Sprintf("args-%d", cmd.Process.Pid)))
		if err != nil || !strings.Contains(string(args), "argument with spaces\n") {
			t.Fatalf("argument/PID forwarding: %v %s", err, args)
		}
		return cmd, done
	}
	stop := func(cmd *exec.Cmd, done chan error) {
		t.Helper()
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("stop did not reach console process")
		}
	}
	first, firstDone := start("first", "DEVICE-A")
	duplicate := command("second", "DEVICE-A")
	if out, err := duplicate.CombinedOutput(); err == nil || !strings.Contains(string(out), "already has a dev session") {
		t.Fatalf("duplicate launch was not rejected: %v %s", err, out)
	}
	select {
	case err := <-firstDone:
		t.Fatalf("duplicate killed first session: %v", err)
	default:
	}
	other, otherDone := start("second", "DEVICE-B") // Different devices remain independent.
	stop(first, firstDone)
	select {
	case err := <-otherDone:
		t.Fatalf("stopping A killed B: %v", err)
	default:
	}
	restarted, restartedDone := start("second", "DEVICE-A") // Dead lock owner is reclaimed.
	write(filepath.Join(root, fmt.Sprintf("finish-%d", restarted.Process.Pid)), "")
	select {
	case err := <-restartedDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("normal exit not propagated")
	}
	stop(other, otherDone)
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "install\n") != 3 || strings.Contains(string(raw), "unexpected-bundle-termination") {
		t.Fatalf("unexpected simulator side effects: %s", raw)
	}
	for _, cmd := range []*exec.Cmd{first, other} {
		if !strings.Contains(string(raw), fmt.Sprintf("stopped-%d", cmd.Process.Pid)) {
			t.Fatal("stop signal was not forwarded")
		}
	}
}
