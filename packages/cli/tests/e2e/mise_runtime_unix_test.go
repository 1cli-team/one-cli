//go:build unix

package cli_test

import (
	"bytes"
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
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node required for process integration")
	}
	for _, mode := range []string{"interrupt", "child-failure"} {
		t.Run(mode, func(t *testing.T) {
			root := runtimeFixture(t)
			// Both long-running fixtures execute Node. Give them valid Node
			// workspace metadata so preparation can be exercised without Go.
			manifest := filepath.Join(root, "one.manifest.json")
			b, err := os.ReadFile(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifest, []byte(strings.ReplaceAll(string(b), `"toolchain":"go"`, `"toolchain":"node"`)), 0o644); err != nil {
				t.Fatal(err)
			}
			for path, body := range map[string]string{
				"package.json":              `{"private":true,"packageManager":"npm@11.0.0","workspaces":["apps/*","services/*"]}`,
				"services/api/package.json": `{"name":"api"}`,
			} {
				if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if mise := os.Getenv("ONE_TEST_MISE_BINARY"); mise != "" {
				useRealMise(t, root, mise)
			} else {
				installFakeMise(t)
			}
			if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
				t.Fatal(err)
			}
			script := `const fs = require('node:fs');
const net = require('node:net');
const server = net.createServer();
server.listen(0, '127.0.0.1', () => fs.writeFileSync('ready', String(server.address().port)));
setInterval(() => { if (fs.existsSync('fail')) process.exit(7); }, 20);
`
			for _, rel := range []string{"apps/web", "services/api"} {
				if err := os.WriteFile(filepath.Join(root, rel, "dev.cjs"), []byte(script), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(binaryPath(t), "dev")
			cmd.Dir = root
			cmd.Env = os.Environ()
			var log bytes.Buffer
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
			for _, rel := range []string{"apps/web", "services/api"} {
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
			} else {
				if err := os.WriteFile(filepath.Join(root, "apps/web/fail"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				finished = true
				if err == nil {
					t.Fatalf("expected interrupted or failed exit\n%s", log.String())
				}
			case <-time.After(8 * time.Second):
				t.Fatal("dev did not stop")
			}
			for _, port := range ports {
				conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 250*time.Millisecond)
				if err == nil {
					conn.Close()
					t.Errorf("project still listening on %s after dev stopped", port)
				}
			}
		})
	}
}
