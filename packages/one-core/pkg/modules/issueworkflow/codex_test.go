package issueworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

func TestCodexImplementationOverridesOnlySpecifiedOptions(t *testing.T) {
	const replies = `{"id":1,"result":{}}
{"id":2,"result":{"exitCode":0}}
{"id":3,"result":{"thread":{"id":"thread"}}}
{"id":4,"result":{"turn":{"id":"turn"}}}
{"method":"turn/completed","params":{"threadId":"thread","turn":{"id":"turn","status":"completed"}}}
`
	for _, tc := range []struct {
		name    string
		options api.CodexOptions
	}{
		{"defaults", api.CodexOptions{}},
		{"model", api.CodexOptions{Model: "fixture-model"}},
		{"effort", api.CodexOptions{Effort: "xhigh"}},
		{"both", api.CodexOptions{Model: "fixture-model", Effort: "xhigh"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests bytes.Buffer
			directory := t.TempDir()
			codex := Codex{Executable: os.Args[0], StateDir: directory,
				Launch: func(context.Context, api.CodexStart) (api.CodexStream, error) {
					return api.CodexStream{Input: &requests, Output: strings.NewReader(replies), Close: func() {}}, nil
				}}
			job := Job{Worktree: directory, Codex: tc.options}
			if err := codex.Implement(t.Context(), &job, directory, func(Job) error { return nil }); err != nil {
				t.Fatal(err)
			}
			decoder := json.NewDecoder(&requests)
			for _, method := range []string{"initialize", "initialized", "command/exec", "thread/start", "turn/start"} {
				var request struct {
					Method string
					Params map[string]any
				}
				if err := decoder.Decode(&request); err != nil || request.Method != method {
					t.Fatalf("request %s: %+v, %v", method, request, err)
				}
				field, want := "model", tc.options.Model
				if method == "turn/start" {
					field, want = "effort", tc.options.Effort
				} else if method != "thread/start" {
					continue
				}
				got, present := request.Params[field]
				if present != (want != "") || (present && got != want) {
					t.Fatalf("%s %s = %v, present = %v, want %q", method, field, got, present, want)
				}
			}
		})
	}
}

func TestInstalledCodexAppServer(t *testing.T) {
	if os.Getenv("ONE_GATEWAY_CODEX_SMOKE") != "1" {
		t.Skip("set ONE_GATEWAY_CODEX_SMOKE=1 to use the installed Codex App Server")
	}
	executable, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	stateDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(stateDir, "checkout")
	if err := os.Mkdir(worktree, 0700); err != nil {
		t.Fatal(err)
	}
	c := Codex{Executable: executable, StateDir: stateDir}
	// The isolated command exercises the real stdio transport and permissions
	// without requesting a model turn or using GitHub or Linear.
	job := Job{Worktree: worktree, Project: Project{Check: []string{
		"/bin/sh", "-c", "printf ok > verification-result",
	}}}
	if err := c.Verify(t.Context(), job, ""); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(worktree, "verification-result"))
	if err != nil || string(got) != "ok" {
		t.Fatalf("verification output = %q, %v", got, err)
	}
}
