//go:build unix

package cli_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/devservice"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
)

func TestDevHTTPHelper(t *testing.T) {
	if os.Getenv("ONE_TEST_DEV_HTTP") != "1" {
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "\x1b[32mReady http://"+listener.Addr().String()+"/\x1b[0m")
	_ = http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "service-ready") }))
	os.Exit(0)
}
func TestE2E_DashboardServiceLifecycle(t *testing.T) {
	root := devTerminalFixture(t, true)
	contextDir := t.TempDir()
	t.Setenv("TMPDIR", contextDir)
	script := "exec env ONE_TEST_DEV_HTTP=1 '" + strings.ReplaceAll(os.Args[0], "'", "'\\''") + "' -test.run=^TestDevHTTPHelper$\n"
	buildWrite(t, root, "packages/lib/dev.sh", script)
	buildWrite(t, root, "apps/web/dev.sh", script)
	cmd := exec.Command(binaryPath(t), "serve", "--port", "0", "--open=false", "-o", "json")
	cmd.Dir = root
	cmd.Env = prependPath(os.Environ(), strings.TrimSuffix(binaryPath(t), "/one"))
	var out, errOut safeBuf
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = process.StopTree(cmd.Process) })
	envelope := waitForEnvelope(t, &out, 5*time.Second, errOut.String)
	base := envelope["url"].(string) + "api/workspace"
	client := &http.Client{Timeout: 2 * time.Second}
	request := func(method, project, action string) devservice.Snapshot {
		t.Helper()
		path := base + "/projects/" + project + "/service"
		if action != "" {
			path += "/" + action
		}
		req, _ := http.NewRequest(method, path, strings.NewReader(`{"environment":"dev"}`))
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, _ := io.ReadAll(response.Body)
		if response.StatusCode >= 300 {
			t.Fatalf("%d: %s", response.StatusCode, raw)
		}
		var state devservice.Snapshot
		if err = json.Unmarshal(raw, &state); err != nil {
			t.Fatal(err)
		}
		return state
	}
	wait := func(project string, predicate func(devservice.Snapshot) bool) devservice.Snapshot {
		t.Helper()
		var state devservice.Snapshot
		for deadline := time.Now().Add(12 * time.Second); time.Now().Before(deadline); {
			state = request("GET", project, "")
			if predicate(state) {
				return state
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("service never became ready: %+v\n%s", state, errOut.String())
		return state
	}
	ready := func(s devservice.Snapshot) bool {
		return s.Status == "running" && len(s.Endpoints) > 0 && s.Endpoints[0].Reachable
	}
	first := request("POST", "lib", "start")
	if again := request("POST", "lib", "start"); again.ID != first.ID {
		t.Fatal("duplicate process")
	}
	first = wait("lib", ready)
	// A second project can prepare and run while the first is still alive.
	request("POST", "web", "start")
	web := wait("web", ready)
	res, err := client.Get(first.Endpoints[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	request("POST", "lib", "restart")
	second := wait("lib", func(s devservice.Snapshot) bool { return s.ID != first.ID && ready(s) })
	request("POST", "lib", "stop")
	wait("lib", func(s devservice.Snapshot) bool { return s.Status == "stopped" })
	if err = cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(9 * time.Second):
		t.Fatal("Dashboard shutdown did not finish")
	}
	remaining, err := filepath.Glob(filepath.Join(contextDir, "one-task-context-*"))
	if err != nil || len(remaining) != 0 {
		t.Fatalf("task context survived shutdown: %v %v", remaining, err)
	}
	for _, s := range []devservice.Snapshot{first, second, web} {
		conn, err := net.DialTimeout("tcp", strings.TrimSuffix(strings.TrimPrefix(s.Endpoints[0].URL, "http://"), "/"), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			t.Fatal("orphaned service port")
		}
	}
}
