package issueworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

func TestCodexImplementationOverridesOnlySpecifiedOptions(t *testing.T) {
	const replies = `{"id":1,"result":{}}
{"id":2,"result":{"exitCode":0}}
{"id":3,"result":{"thread":{"id":"thread"},"model":"resolved-model","reasoningEffort":"medium"}}
{"id":4,"result":{"turn":{"id":"turn"}}}
{"method":"turn/completed","params":{"threadId":"thread","turn":{"id":"turn","status":"completed"}}}
`
	for _, tc := range []struct {
		name        string
		options     api.CodexOptions
		chinaMirror bool
	}{
		{"defaults", api.CodexOptions{}, false},
		{"model", api.CodexOptions{Model: "fixture-model"}, false},
		{"effort", api.CodexOptions{Effort: "xhigh"}, false},
		{"both", api.CodexOptions{Model: "fixture-model", Effort: "xhigh"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests bytes.Buffer
			directory := t.TempDir()
			codex := Codex{Executable: os.Args[0], StateDir: directory,
				Launch: func(_ context.Context, start api.CodexStart) (api.CodexStream, error) {
					if slices.Contains(start.Config, `shell_environment_policy.set.ONE_CHINA_MIRROR = "1"`) != tc.chinaMirror {
						t.Error("mirror selection was not forwarded to the task environment")
					}
					return api.CodexStream{Input: &requests, Output: strings.NewReader(replies), Close: func() {}}, nil
				}}
			job := Job{Worktree: directory, Codex: tc.options, ChinaMirror: tc.chinaMirror}
			var effective api.CodexOptions
			if err := codex.Implement(t.Context(), &job, directory, func(snapshot Job) error {
				if snapshot.TurnID != "" {
					effective = snapshot.EffectiveCodex
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			wantEffort := tc.options.Effort
			if wantEffort == "" {
				wantEffort = "medium"
			}
			if effective.Model != "resolved-model" || effective.Effort != wantEffort || job.Codex != tc.options {
				t.Fatalf("execution settings = %+v, dispatch options = %+v", effective, job.Codex)
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

func TestCodexPromptReportsReroutedModelForTheCurrentTurn(t *testing.T) {
	const replies = `{"id":1,"result":{}}
{"id":2,"result":{"exitCode":0}}
{"id":3,"result":{"thread":{"id":"thread"},"model":"original","reasoningEffort":"high"}}
{"id":4,"result":{"turn":{"id":"turn"}}}
{"method":"model/rerouted","params":{"threadId":"thread","turnId":"earlier","toModel":"unrelated"}}
{"method":"model/rerouted","params":{"threadId":"thread","turnId":"turn","toModel":"fallback"}}
{"method":"turn/completed","params":{"threadId":"thread","turn":{"id":"turn","status":"completed"}}}
`
	directory := t.TempDir()
	var reported []api.CodexOptions
	codex := Codex{Executable: os.Args[0], StateDir: directory,
		OnSettings: func(settings api.CodexOptions) { reported = append(reported, settings) },
		Launch: func(context.Context, api.CodexStart) (api.CodexStream, error) {
			return api.CodexStream{Input: new(bytes.Buffer), Output: strings.NewReader(replies), Close: func() {}}, nil
		}}
	result, err := codex.RunPrompt(t.Context(), directory, directory, "prompt", api.CodexOptions{}, nil)
	want := api.CodexOptions{Model: "fallback", Effort: "high"}
	if err != nil || result.EffectiveCodex != want || len(reported) != 2 || reported[0].Model != "original" || reported[1] != want {
		t.Fatalf("result = %+v, reported = %+v, err = %v", result, reported, err)
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
