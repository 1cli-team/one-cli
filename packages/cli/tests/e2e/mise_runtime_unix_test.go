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

func TestE2E_ExecWithoutInternalWorkerCancelsDescendants(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node required")
	}
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			root := buildFixture(t, true)
			script := `const fs=require('node:fs');const net=require('node:net');
if (!process.env.EXEC_GRANDCHILD) {require('node:child_process').spawn(process.execPath,[__filename],{env:{...process.env,EXEC_GRANDCHILD:'1'},stdio:'inherit'});setInterval(()=>{},1000)}
else {net.createServer().listen(0,'127.0.0.1',function(){fs.writeFileSync('exec-ready',String(this.address().port))})}
`
			buildWrite(t, root, "apps/web/exec.cjs", script)
			cmd := exec.Command(binaryPath(t), "exec", "web", "--", node, "exec.cjs")
			cmd.Dir = root
			cmd.Env = os.Environ()
			var log lockedBuffer
			cmd.Stdout, cmd.Stderr = &log, &log
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			var port string
			deadline := time.Now().Add(8 * time.Second)
			for port == "" && time.Now().Before(deadline) {
				if raw, err := os.ReadFile(filepath.Join(root, "apps/web/exec-ready")); err == nil {
					port = string(raw)
					break
				}
				select {
				case err := <-done:
					t.Fatalf("exec stopped: %v %s", err, log.String())
				default:
				}
				time.Sleep(10 * time.Millisecond)
			}
			if port == "" {
				t.Fatalf("exec never ready: %s", log.String())
			}
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				want := 130
				if sig == syscall.SIGTERM {
					want = 143
				}
				if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != want {
					t.Fatalf("exit %v, want %d: %s", err, want, log.String())
				}
			case <-time.After(8 * time.Second):
				t.Fatalf("exec did not stop: %s", log.String())
			}
			deadline = time.Now().Add(time.Second)
			for {
				conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 100*time.Millisecond)
				if err != nil {
					break
				}
				conn.Close()
				if time.Now().After(deadline) {
					t.Fatal("exec grandchild still listening")
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}
