package serve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

func TestCreateBranchRoutes(t *testing.T) {
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
	registerGitRoutes(mux, MuxOpts{GitService: gitmodule.NewService(plugin, registry), RegistryService: registry})
	workspace := "/workspaces/" + entry.EntryID + "/git/branches"
	repository := "/git/repositories/" + entry.EntryID + "/branches"
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
