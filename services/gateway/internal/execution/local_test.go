package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
)

func TestLocalExecutionSurvivesRestart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses an executable shell fixture")
	}
	t.Setenv("ONE_HOME", t.TempDir())
	t.Setenv("ONE_GATEWAY_TOKEN", "server-token-not-for-the-child")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("remote", "add", "origin", "https://example.test/owner/repo.git")
	if err := os.WriteFile(filepath.Join(repo, "one.manifest.toml"), []byte("version = 2\n[workspace]\nid = 'test'\nname = 'test'\n[projects.app]\npath = 'apps/app'\ntoolchain = 'node'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "apps", "app"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "apps", "app", "tracked"), []byte("tracked"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("-c", "user.name=Test", "-c", "user.email=test@example.test", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	sha := git("rev-parse", "HEAD")

	root := t.TempDir()
	one := filepath.Join(root, "one")
	if err := os.WriteFile(one, []byte("#!/bin/sh\nset -eu\ntest -z \"${ONE_GATEWAY_TOKEN:-}\"\ntest \"${ONE_CHINA_MIRROR:-}\" = 1\ntest -e one.manifest.toml\ntest \"$1 $2 $3 $4\" = 'run check -p app'\nprintf 'checking\\n' >&2\nprintf '{\"status\":\"succeeded\"}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	runner, err := NewRunner(filepath.Join(root, "local"), one, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.git.Associate(ctx, repo); err != nil {
		t.Fatal(err)
	}

	store, err := Open(root, "this machine")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		store.RunLocal(ctx, runner)
	}()
	defer func() { cancel(); <-stopped }()

	token := strings.Repeat("t", 32)
	handler := Control(token, store, http.NotFoundHandler(), "")
	call := func(handler http.Handler, method, route string, input any, wantStatus int, result any) {
		t.Helper()
		body, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(method, route, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != wantStatus {
			t.Fatalf("%s %s returned %d, want %d: %s", method, route, w.Code, wantStatus, w.Body)
		}
		if result != nil && json.Unmarshal(w.Body.Bytes(), result) != nil {
			t.Fatalf("invalid JSON: %s", w.Body)
		}
	}
	input := api.ExecutionSubmission{WorkerID: LocalWorkerID, RequestID: "smoke/1", Source: "cli", Input: api.ExecutionInput{
		Kind: api.OneTask, Repository: gitmodule.RepositoryRef{Host: "example.test", Slug: "owner/repo"},
		CommitSHA: sha, Project: "app", Task: "check",
	}}
	var job, duplicate api.Execution
	const downloadsPath = "/v1/settings/downloads"
	var downloads api.DownloadSettings
	call(handler, "GET", downloadsPath, nil, http.StatusOK, &downloads)
	if downloads.ChinaMirror {
		t.Fatal("China mirrors must be opt-in")
	}
	call(handler, "PUT", downloadsPath, struct{}{}, http.StatusBadRequest, nil)
	call(handler, "PUT", downloadsPath, api.DownloadSettings{ChinaMirror: true}, http.StatusOK, &downloads)
	if !downloads.ChinaMirror {
		t.Fatal("PUT did not return the saved download settings")
	}
	call(handler, "POST", "/v1/executions", input, http.StatusAccepted, &job)
	call(handler, "PUT", downloadsPath, api.DownloadSettings{}, http.StatusOK, nil)
	call(handler, "POST", "/v1/executions", input, http.StatusAccepted, &duplicate)
	if job.ID != duplicate.ID || !job.ChinaMirror || !duplicate.ChinaMirror {
		t.Fatalf("repeated dispatch lost the original settings: %+v, %+v", job, duplicate)
	}
	conflict := input
	conflict.Input.Task = "another"
	call(handler, "POST", "/v1/executions", conflict, http.StatusConflict, nil)
	path := "/v1/executions/" + job.ID
	for deadline := time.Now().Add(10 * time.Second); job.Result == nil && time.Now().Before(deadline); {
		time.Sleep(30 * time.Millisecond)
		job = api.Execution{}
		call(handler, "GET", path, nil, http.StatusOK, &job)
	}
	if job.State != api.ExecutionSucceeded || job.Result == nil || job.Result.CommitSHA != sha || job.Result.ExitCode == nil || *job.Result.ExitCode != 0 {
		t.Fatalf("unexpected result: %+v / %+v", job, job.Result)
	}
	cancel()
	<-stopped
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root, "this machine")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	handler = Control(token, reopened, http.NotFoundHandler(), "")
	call(handler, "GET", downloadsPath, nil, http.StatusOK, &downloads)
	if downloads.ChinaMirror {
		t.Fatal("disabled setting did not persist after restart")
	}
	var persisted api.Execution
	call(handler, "GET", path, nil, http.StatusOK, &persisted)
	if persisted.State != api.ExecutionSucceeded || persisted.Result == nil || persisted.Result.CommitSHA != sha || !persisted.ChinaMirror {
		t.Fatalf("unexpected persisted result: %+v", persisted)
	}
	var logs api.ExecutionLogs
	call(handler, "GET", path+"/logs", nil, http.StatusOK, &logs)
	if string(logs.Data) != "checking\n" {
		t.Fatalf("unexpected logs: %q", logs.Data)
	}
}
