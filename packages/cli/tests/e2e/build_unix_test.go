//go:build unix

package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	buildmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/tasks"
)

func TestE2E_BuildSignalStopsProcessTree(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			root := buildFixture(t, false)
			buildWrite(t, root, "packages/lib/build.sh", "#!/bin/sh\nsleep 60 &\necho $! > ../../grandchild.pid\nwait\n")
			cmd := exec.Command(binaryPath(t), "build", "-o", "json")
			cmd.Dir = root
			cmd.Env = os.Environ()
			var out, errOut bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &errOut
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer cmd.Process.Kill()
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			deadline := time.Now().Add(10 * time.Second)
			pid := 0
			for time.Now().Before(deadline) {
				raw, err := os.ReadFile(filepath.Join(root, "grandchild.pid"))
				if err == nil {
					pid, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
					if pid > 0 {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
			}
			if pid == 0 {
				t.Fatal("build process did not start")
			}
			defer syscall.Kill(pid, syscall.SIGKILL)
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				exit, ok := err.(*exec.ExitError)
				want := 128 + int(sig)
				if !ok || exit.ExitCode() != want {
					t.Fatalf("exit %v; stdout=%s stderr=%s", err, out.String(), errOut.String())
				}
			case <-time.After(10 * time.Second):
				t.Fatal("build did not stop")
			}
			var result buildmodule.Result
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err, out.String())
			}
			if result.Status != "cancelled" {
				t.Fatal(result.Tasks)
			}
			deadline = time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if syscall.Kill(pid, 0) == syscall.ESRCH {
					return
				}
				// Linux may briefly retain a killed orphan as a zombie until init reaps it.
				raw, _ := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
				if end := strings.LastIndex(string(raw), ") "); end >= 0 && strings.HasPrefix(string(raw)[end+2:], "Z ") {
					return
				}
				time.Sleep(20 * time.Millisecond)
			}
			t.Fatalf("grandchild %d remains running", pid)
		})
	}
}
