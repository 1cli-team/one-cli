package issueworkflow

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
)

func TestGiteaCodeHostAndPRLifecycle(t *testing.T) {
	const credential = "fixture-token"
	const branch = "one/linear/one-1"
	const slug = "team/sample"
	const pulls = "/api/v1/repos/" + slug + "/pulls"
	var created atomic.Int32
	var merged atomic.Bool
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token "+credential {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("private upstream error"))
			return
		}
		switch {
		case r.URL.Path == "/api/v1/user":
			writeJSON(w, http.StatusOK, map[string]string{"login": "test-user"})
		case r.URL.Path == pulls && r.Method == http.MethodGet:
			if r.URL.Query().Get("state") != "all" || r.URL.Query().Get("base_branch") != "main" {
				t.Error("missing pull request filters")
			}
			page := r.URL.Query().Get("page")
			if page == "1" {
				writeJSON(w, http.StatusOK, []any{giteaPullFixture(10, "other/sample", branch)})
			} else if page == "2" && created.Load() > 0 {
				writeJSON(w, http.StatusOK, []any{giteaPullFixture(42, slug, branch)})
			} else {
				writeJSON(w, http.StatusOK, []any{})
			}
		case r.URL.Path == pulls && r.Method == http.MethodPost:
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
				body["head"] != branch || body["base"] != "main" || body["title"] != "ONE-1 Implement sample" {
				t.Errorf("unexpected PR creation: %v %v", body, err)
			}
			created.Add(1)
			// Simulate a PR created by Gitea even though Gateway lost the response.
			w.WriteHeader(http.StatusServiceUnavailable)
		case r.URL.Path == pulls+"/42":
			result := PullRequest{Number: 42, State: "open"}
			if merged.Load() {
				result.Merged, result.State = true, "closed"
			}
			writeJSON(w, http.StatusOK, result)
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer remote.Close()

	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	t.Setenv("ONE_HOME", t.TempDir())
	service, linear, runner, git, _ := fixture(store)
	linear.issue.Title = "Implement sample"
	hosts := codehost.Service{}
	service.pr = hosts
	if _, err := hosts.Save(t.Context(), codehost.Input{URL: remote.URL, Token: "rejected"}); err != codehost.ErrAuth {
		t.Fatalf("bad token = %v", err)
	}
	local, err := hosts.Save(t.Context(), codehost.Input{URL: remote.URL, Token: credential})
	if err != nil || local.Account != "test-user" {
		t.Fatalf("save connection = %+v, %v", local, err)
	}
	if _, err := hosts.Save(t.Context(), codehost.Input{URL: remote.URL, Token: "rejected-rotation"}); err != codehost.ErrAuth {
		t.Fatalf("bad rotation = %v", err)
	}
	if _, err := hosts.Test(t.Context(), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	// The public view can be loaded independently from the workflow.
	views, err := (codehost.Service{}).List()
	if err != nil || len(views) != 2 || views[0].Provider != "github" || views[1] != local {
		t.Fatalf("local connections: %+v, %v", views, err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("ONE_HOME"), "config", "code-hosts.json")); err != nil {
		t.Fatal(err)
	}
	project := service.projects[projectID]
	project.Repository = gitmodule.ParseRepositoryRef(remote.URL + "/" + slug + ".git")
	project.Base = "main"
	service.projects[projectID] = project
	if !hosts.MatchesOrigin(project.ref(), "ssh://git@127.0.0.1:2222/"+slug+".git") ||
		hosts.MatchesOrigin(project.ref(), "https://127.0.0.1:44/"+slug+".git") ||
		hosts.MatchesOrigin(project.ref(), "https://127.0.0.2/"+slug+".git") {
		t.Fatal("repository host or port did not match correctly")
	}
	verifyWorkRequest(t, service, api.WorkRequest{Operation: api.WorkDispatch, ProjectID: projectID, IssueID: issueID}, http.StatusAccepted)
	job, _ := store.get(projectID, issueID)
	ctx := t.Context()
	if err := service.run(ctx, &job); err != nil || job.Stage != publishing {
		t.Fatalf("execute = %s, %v", job.Stage, err)
	}
	if err := service.deliver(ctx, &job); err == nil || job.Stage != opening {
		t.Fatalf("publish = %s, %v", job.Stage, err)
	}
	for _, want := range []stage{toReview, review} {
		if err := service.deliver(ctx, &job); err != nil || job.Stage != want {
			t.Fatalf("deliver = %s, %v, want %s", job.Stage, err, want)
		}
	}
	merged.Store(true)
	for _, want := range []stage{toDone, done} {
		if err := service.deliver(ctx, &job); err != nil || job.Stage != want {
			t.Fatalf("finish = %s, %v, want %s", job.Stage, err, want)
		}
	}
	if created.Load() != 1 || git.pushes != 1 || runner.turns != 1 ||
		!slices.Equal(linear.transitions, []string{"running", "review", "done"}) {
		t.Fatalf("unexpected side effects: created=%d pushed=%d turns=%d Linear=%v",
			created.Load(), git.pushes, runner.turns, linear.transitions)
	}
}

func giteaPullFixture(number int, slug, branch string) map[string]any {
	return map[string]any{"number": number, "state": "open", "head": map[string]any{"ref": branch, "repo": map[string]string{"full_name": slug}}, "base": map[string]string{"ref": "main"}}
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
