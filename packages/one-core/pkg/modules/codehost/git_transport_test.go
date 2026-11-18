package codehost

import (
	"encoding/base64"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGitEnvironmentScopesAuthorizationToRepoOrigin(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	saved, ok := parseInput(Input{URL: "https://git.example.invalid:4433", Token: "fixture-private-token"})
	if !ok {
		t.Fatal("invalid fixture")
	}
	saved.Account = "gateway-worker"
	if err := update(t.Context(), func(values map[string]credential) (bool, error) {
		values[saved.Host] = saved
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	remote := saved.URL + "/team/repo.git"
	env, err := (Service{}).GitEnvironment(append(os.Environ(), "GIT_TRACE_CURL=1"), remote)
	if err != nil {
		t.Fatal(err)
	}
	match := func(remote string) string {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", "config", "--get-urlmatch", "http.extraHeader", remote)
		cmd.Env = env
		out, _ := cmd.Output()
		return strings.TrimSpace(string(out))
	}
	auth := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(saved.Account+":"+saved.Token))
	if match(remote+"/info/refs") != auth {
		t.Fatal("Git did not apply authorization to the repository")
	}
	for _, url := range []string{saved.URL + "/team/repo.git-extra", saved.URL + "/team/other.git", "https://git.example.invalid/team/repo.git", "https://other.invalid:4433/team/repo.git"} {
		if match(url) == auth {
			t.Fatalf("Git sent authorization outside the repository: %s", url)
		}
	}
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_TRACE") {
			t.Fatal("the authenticated subprocess inherited Git tracing")
		}
	}
}
