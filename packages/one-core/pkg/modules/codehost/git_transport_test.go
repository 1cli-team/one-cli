package codehost

import (
	"encoding/base64"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func TestGitEnvironmentUsesScopedTokenWithoutAccount(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	saved, ok := parseInput(Input{URL: "https://git.example.invalid", Token: "fixture-token", Repository: "team/repo"})
	if !ok {
		t.Fatal("invalid fixture")
	}
	if err := update(t.Context(), func(values map[string]credential) (bool, error) {
		values[saved.Host] = saved
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	service := Service{}
	env, err := service.GitEnvironment(nil, saved.URL+"/team/repo.git")
	auth := "GIT_CONFIG_VALUE_0=Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(saved.Token+":"))
	if err != nil || !slices.Contains(env, auth) {
		t.Fatalf("expected Git HTTPS authentication for the target repository, got error %v", err)
	}
	if env, err := service.GitEnvironment(nil, saved.URL+"/team/other.git"); err != nil || len(env) != 0 {
		t.Fatalf("unexpected Git HTTPS authentication for a different repository, got error %v", err)
	}
}

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
