package execution

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
)

type loopTransport struct{ handler http.Handler }

func (l loopTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	l.handler.ServeHTTP(recorder, req)
	return recorder.Result(), nil
}

func TestRemoteWorkerOwnsOnlyItsJobsAndCanReportAfterRestart(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root, "host")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	invite := s.invite()
	register := func(name, code string) (remoteLink, error) {
		t.Helper()
		cfg := workerConfig{URL: "http://worker.example.test", ID: "w-" + strings.ToLower(rand.Text()), Name: name, Token: rand.Text() + rand.Text()}
		link, err := newRemote(cfg)
		if err != nil {
			t.Fatal(err)
		}
		link.http.Transport = loopTransport{WorkerAPI(s)}
		err = link.send(ctx, link.base.JoinPath("worker", "v1", "join"), joinRequest{ID: cfg.ID, Name: name, Code: code}, new(struct{}))
		return link, err
	}
	link, err := register("remote", invite.Code)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := register("other", invite.Code); err == nil {
		t.Fatal("another worker reused a consumed pairing code")
	}
	other, err := register("other", s.invite().Code)
	if err != nil {
		t.Fatal(err)
	}

	ref := gitmodule.RepositoryRef{Host: "example.test", Slug: "owner/repo"}
	inventory := api.ExecutionWorker{Repositories: []gitmodule.RepositoryRef{ref}}
	session, otherSession := rand.Text(), rand.Text()
	if err := link.heartbeat(ctx, session, inventory); err != nil {
		t.Fatal(err)
	}
	if err := other.heartbeat(ctx, otherSession, inventory); err != nil {
		t.Fatal(err)
	}
	job, err := s.submit(api.ExecutionSubmission{WorkerID: link.config.ID, RequestID: "remote/1", Source: "cli", Input: api.ExecutionInput{
		Kind: api.OneTask, Repository: ref, Ref: "refs/heads/main", Project: "app", Task: "check",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if claim, err := other.claim(ctx, otherSession); claim != nil || err != nil {
		t.Fatalf("wrong worker claim: %+v, %v", claim, err)
	}
	claim, err := link.claim(ctx, session)
	if err != nil || claim == nil || claim.ID != job.ID {
		t.Fatalf("unexpected claim: %+v, %v", claim, err)
	}
	retry, err := link.claim(ctx, session)
	if err != nil || retry == nil || retry.Attempt != claim.Attempt {
		t.Fatalf("claim retry created another attempt: %+v, %v", retry, err)
	}
	done := report{JobID: job.ID, Attempt: claim.Attempt, Logs: []byte("finished")}
	if err := other.complete(ctx, done); err == nil {
		t.Fatal("another worker reported the result")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(root, "host")
	if err != nil {
		t.Fatal(err)
	}
	link.http.Transport = loopTransport{WorkerAPI(s)}
	state, err := s.job(job.ID)
	if err != nil || state.State != api.ExecutionUnknown {
		t.Fatalf("after restart: %+v, %v", state, err)
	}
	if err := link.heartbeat(ctx, rand.Text(), inventory); err != nil {
		t.Fatal(err)
	}
	if err := link.complete(ctx, done); err != nil {
		t.Fatal(err)
	}
	if err := link.complete(ctx, done); err != nil {
		t.Fatal(err)
	}
	state, err = s.job(job.ID)
	if err != nil || state.State != api.ExecutionSucceeded {
		t.Fatalf("replayed report: %+v, %v", state, err)
	}

	workerReq := httptest.NewRequest(http.MethodGet, "/v1/executions", bytes.NewReader(nil))
	workerReq.Header.Set("Authorization", "Bearer "+link.config.Token)
	recorder := httptest.NewRecorder()
	Control(strings.Repeat("a", 32), s, http.NotFoundHandler(), "").ServeHTTP(recorder, workerReq)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("worker credential reached control API: %d", recorder.Code)
	}
	logs, err := s.logs(job.ID, 0)
	if err != nil || string(logs.Data) != "finished" {
		t.Fatalf("replayed logs: %+v, %v", logs, err)
	}
	data, err := json.Marshal(s.workersList())
	if err != nil || bytes.Contains(data, []byte(link.config.Token)) {
		t.Fatalf("worker list exposed credentials: %s, %v", data, err)
	}
}
