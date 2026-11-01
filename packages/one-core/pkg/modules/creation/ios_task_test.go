package creation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/tasks"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
)

// Run the real task worker in a subprocess so signals follow One's cancellation
// path. Only the scheduler's invocation broker and simulator are test doubles.
func iosTaskCommand(t *testing.T, script *exec.Cmd) *exec.Cmd {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ios-test" || r.URL.Path != "/task-0" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"task":     tasks.Task{Name: "ios:dev", Directory: script.Dir},
				"commands": []string{"bash scripts/dev.sh"}, "arguments": script.Args[2:],
				"environment": script.Env, "force": true,
			})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	cmd := exec.Command(os.Args[0], "-test.run=^TestIOSTaskLeaf$")
	cmd.Env = append(append([]string{}, script.Env...), "ONE_TEST_IOS_LEAF=1",
		"ONE_PROCESS_ENDPOINT="+server.URL, "ONE_PROCESS_TOKEN=ios-test")
	return cmd
}

func TestIOSTaskLeaf(t *testing.T) {
	if os.Getenv("ONE_TEST_IOS_LEAF") != "1" {
		return
	}
	err := tasks.RunProcessLeaf(context.Background(), "task-0", os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(process.ExitCode(err))
}
