//go:build unix

package cli_test

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestE2E_MiseDevStopsBothProjects(t *testing.T) {
	// Reproduce invocation from an outer mise task, including Git hook gates.
	t.Setenv("MISE_TASK_PGID_MANAGED", "1")
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node required for process integration")
	}
	for _, mode := range []string{"interrupt", "terminate", "child-failure"} {
		t.Run(mode, func(t *testing.T) {
			root := devTerminalFixture(t, true)
			for _, project := range []string{"web", "lib"} {
				overrideDevCommand(t, root, project, "'"+strings.ReplaceAll(node, "'", "'\"'\"'")+"' dev.cjs")
			}
			script := `const fs = require('node:fs');
const net = require('node:net');
const server = net.createServer();
server.listen(0, '127.0.0.1', () => fs.writeFileSync('ready', String(server.address().port)));
setInterval(() => { if (fs.existsSync('fail')) process.exit(7); }, 20);
`
			for _, rel := range []string{"apps/web", "packages/lib"} {
				if err := os.WriteFile(filepath.Join(root, rel, "dev.cjs"), []byte(script), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(binaryPath(t), "dev")
			cmd.Dir = root
			cmd.Env = os.Environ()
			var log lockedBuffer
			cmd.Stdout, cmd.Stderr = &log, &log
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			finished := false
			defer func() {
				if !finished {
					_ = cmd.Process.Signal(syscall.SIGTERM)
					select {
					case <-done:
					case <-time.After(8 * time.Second):
						_ = cmd.Process.Kill()
						<-done
					}
				}
			}()
			ports := []string{}
			deadline := time.Now().Add(10 * time.Second)
			for _, rel := range []string{"apps/web", "packages/lib"} {
				for {
					if raw, err := os.ReadFile(filepath.Join(root, rel, "ready")); err == nil {
						ports = append(ports, strings.TrimSpace(string(raw)))
						break
					}
					select {
					case err := <-done:
						finished = true
						t.Fatalf("dev stopped before ready: %v\n%s", err, log.String())
					default:
					}
					if time.Now().After(deadline) {
						t.Fatal("dev did not start both projects")
					}
					time.Sleep(20 * time.Millisecond)
				}
			}
			if mode == "interrupt" {
				if err := cmd.Process.Signal(os.Interrupt); err != nil {
					t.Fatal(err)
				}
			} else if mode == "terminate" {
				if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(filepath.Join(root, "apps/web/fail"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				finished = true
				want := 7
				if mode == "interrupt" {
					want = 130
				}
				if mode == "terminate" {
					want = 143
				}
				if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != want {
					t.Fatalf("exit=%v want=%d\n%s", err, want, log.String())
				}
			case <-time.After(8 * time.Second):
				t.Fatalf("dev did not stop\n%s", log.String())
			}
			for _, port := range ports {
				// SIGKILL delivery and the kernel closing descendant sockets are
				// asynchronous. Require closure within a bounded interval.
				deadline := time.Now().Add(time.Second)
				for {
					conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 100*time.Millisecond)
					if err != nil {
						break
					}
					conn.Close()
					if time.Now().After(deadline) {
						t.Errorf("project still listening on %s after dev stopped", port)
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
		})
	}
}
