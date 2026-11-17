package serve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	core "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	gitplugin "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/git"
)

type branchTestRegistry struct{ state core.Registry }

func (r *branchTestRegistry) Load(context.Context) (core.Registry, error) { return r.state, nil }
func (r *branchTestRegistry) Update(_ context.Context, update func(*core.Registry) error) error {
	return update(&r.state)
}

func gitRoutesFixture(t *testing.T) (string, string, *http.ServeMux) {
	t.Helper()
	t.Setenv("ONE_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "initial"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if data, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, data)
		}
	}
	plugin := gitplugin.New()
	registry, err := workspaceapp.NewRegistryService(&branchTestRegistry{}, workspaceapp.RegistryOptions{WorktreeIdentity: plugin.WorkspaceIdentity, Git: plugin.InspectWorkspaceRepository})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := registry.Observe(context.Background(), root, "test")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	opts := MuxOpts{GitService: gitmodule.NewService(plugin, registry), RegistryService: registry}
	registerGitRoutes(mux, opts)
	registerGatewayProjectRoutes(mux, opts)
	return root, entry.EntryID, mux
}

func TestGatewayProjectOriginsUsesWorkspaceRemote(t *testing.T) {
	for _, test := range []struct {
		name, url, github string
		repository        gitmodule.RepositoryRef
	}{
		{name: "no origin"},
		{name: "GitHub HTTPS", url: "https://github.com/Example/One.git", github: "example/one"},
		{name: "GitHub SSH", url: "git@github.com:Example/One.git", github: "example/one"},
		{name: "Gitea SSH", url: "ssh://git@git.example.com:2222/team/one.git",
			repository: gitmodule.RepositoryRef{Host: "git.example.com", Slug: "team/one"}},
		{name: "local Git", url: "/repos/one.git"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, id, handler := gitRoutesFixture(t)
			canonicalRoot, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			path := "/gateway/project-origins?kind=workspace&scope=" + id
			if test.url != "" {
				command := exec.Command("git", "config", "remote.origin.url", test.url)
				command.Dir = root
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("git config: %v %s", err, output)
				}
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("GET %s: %d %s", path, response.Code, response.Body)
			}
			var origins []gatewayProjectOrigin
			if err := json.Unmarshal(response.Body.Bytes(), &origins); err != nil {
				t.Fatal(err)
			}
			if len(origins) != 1 || origins[0] != (gatewayProjectOrigin{
				Path: canonicalRoot, GitHub: test.github, HasOrigin: test.url != "", Repository: test.repository,
			}) {
				t.Fatalf("unexpected origin: %+v", origins)
			}
		})
	}
}

func TestCreateBranchRoutes(t *testing.T) {
	root, id, mux := gitRoutesFixture(t)
	workspace := "/workspaces/" + id + "/git/branches"
	repository := "/git/repositories/" + id + "/branches"
	for _, test := range []struct {
		path, body string
		status     int
	}{
		{workspace, `{"branch":"mg-1","startPoint":"main","checkout":false}`, 201},
		{workspace, `{"branch":"mg-1"}`, 409},
		{workspace, `{"branch":"bad..name"}`, 400},
		{workspace, `{"branch":"--orphan"}`, 400},
		{workspace, `{"branch":"mg-2","path":"/tmp/unregistered"}`, 400},
		{workspace, `{"branch":"mg-2","checkout":"yes"}`, 400},
		{repository, `{"branch":"mg-2","checkout":true}`, 201},
		{"/workspaces/missing/git/branches", `{"branch":"mg-3"}`, 404},
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body)))
		if response.Code != test.status {
			t.Errorf("%s %s: %d %s", test.path, test.body, response.Code, response.Body.String())
		}
	}
	command := exec.Command("git", "branch", "--show-current")
	command.Dir = root
	if data, err := command.Output(); err != nil || strings.TrimSpace(string(data)) != "mg-2" {
		t.Fatalf("checkout was not applied: %s %v", data, err)
	}
}

func TestWorkspaceGitOriginBinding(t *testing.T) {
	root, id, handler := gitRoutesFixture(t)
	path := "/workspaces/" + id + "/git/origin"
	const initial = "https://github.com/example/one.git"
	const changed = "git@git.example.com:team/one.git"
	const push = "ssh://git@git.example.com/team/push-one.git"
	call := func(path string, input gitmodule.SetOriginInput, want int) gitmodule.Remote {
		t.Helper()
		body, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, path, strings.NewReader(string(body))))
		if rec.Code != want {
			t.Fatalf("PUT %s returned %d, want %d: %s", path, rec.Code, want, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "fixture-secret") {
			t.Fatal("Git error included the rejected credential")
		}
		var result gitmodule.Remote
		if want == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
		}
		return result
	}
	git := func(dir string, args ...string) string {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = dir
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v %s", args[0], err, output)
		}
		return strings.TrimSpace(string(output))
	}

	call("/workspaces/missing/git/origin", gitmodule.SetOriginInput{URL: initial}, http.StatusNotFound)
	call(path, gitmodule.SetOriginInput{URL: "https://user:fixture-secret@github.com/example/one.git"}, http.StatusBadRequest)
	got := call(path, gitmodule.SetOriginInput{URL: initial}, http.StatusOK)
	if got.Name != "origin" || got.FetchURL != initial || got.PushURL != initial {
		t.Fatalf("created origin: %+v", got)
	}
	checkout := t.TempDir() + "/linked"
	git(root, "worktree", "add", "--detach", checkout)
	git(root, "remote", "set-url", "--push", "origin", push)

	stale := gitmodule.SetOriginInput{URL: changed, ExpectedFetchURL: initial, ExpectedPushURL: initial}
	call(path, stale, http.StatusConflict)
	if url := git(checkout, "remote", "get-url", "origin"); url != initial {
		t.Fatalf("stale save changed the fetch URL: %s", url)
	}
	update := gitmodule.SetOriginInput{URL: changed, ExpectedFetchURL: initial, ExpectedPushURL: push}
	got = call(path, update, http.StatusOK)
	if got.FetchURL != changed || got.PushURL != push ||
		git(checkout, "remote", "get-url", "origin") != changed ||
		git(root, "remote", "get-url", "--push", "origin") != push {
		t.Fatalf("linked worktree and origin don't reflect the expected configuration: %+v", got)
	}
	call(path, update, http.StatusOK)
}
