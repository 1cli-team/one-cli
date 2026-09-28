package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	cli "github.com/torchstellar-team/one-cli/packages/cli/internal/bootstrap/cli"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	"github.com/zalando/go-keyring"
)

// Run the production command tree in a fresh test process with an in-memory
// keyring. No production auth override or developer keyring is used.
func TestAuthenticatedCLIHelper(t *testing.T) {
	site := os.Getenv("ONE_TEST_SECRET_SERVER")
	if site == "" {
		return
	}
	keyring.MockInit()
	raw, _ := json.Marshal(session.Session{Info: session.Info{SiteURL: site, ExpiresAt: time.Now().Add(time.Hour)}, Token: "test-session-token"})
	if err := keyring.Set("one-cli.infisical", "session", string(raw)); err != nil {
		t.Fatal(err)
	}
	for i, arg := range os.Args {
		if arg == "--" {
			if err := cli.Execute("0.0.0-dev", os.Args[i+1:]); err != nil {
				os.Exit(process.ExitCode(err))
			}
			os.Exit(0)
		}
	}
	t.Fatal("missing command arguments")
}
func runAuthenticatedCLI(t *testing.T, root, site string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestAuthenticatedCLIHelper$", "--"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "ONE_TEST_SECRET_SERVER="+site)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	code := 0
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return out.String(), errOut.String(), code
}
func TestE2E_SecretManagementNeverPrintsValues(t *testing.T) {
	const secret = "test-private-credential-never-print"
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/workspace/test-project" {
			_ = json.NewEncoder(w).Encode(map[string]any{"workspace": map[string]any{"id": "test-project", "type": "secret-manager", "environments": []any{map[string]string{"slug": "dev"}}}})
			return
		}
		if strings.Contains(r.URL.Path, "folders") {
			_, _ = w.Write([]byte(`{"folders":[]}`))
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v3/secrets/raw") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if fail.Load() {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "rejected " + secret + " test-session-token"})
			return
		}
		value := map[string]string{"secretKey": "TEST_KEY", "secretValue": secret}
		if r.URL.Path == "/api/v3/secrets/raw" {
			_ = json.NewEncoder(w).Encode(map[string]any{"secrets": []any{value}})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{"secret": value})
		}
	}))
	defer server.Close()
	root := t.TempDir()
	isolateHome(t, t.TempDir())
	manifest := map[string]any{"version": 1, "workspace": map[string]string{"id": "test", "name": "test"}, "env": map[string]string{"siteUrl": server.URL, "projectId": "test-project"}, "environments": map[string]any{"names": []string{"dev"}, "default": "dev"}, "projects": []any{}}
	raw, _ := json.Marshal(manifest)
	buildWrite(t, root, "one.manifest.json", string(raw))
	if out, errOut, code := runAuthenticatedCLI(t, root, server.URL, "env", "bind", "--global", "--project-id", "test-project", "--env", "dev", "-o", "json"); code != 0 {
		t.Fatalf("bind: %s %s", out, errOut)
	}
	for _, locale := range []string{"en_US.UTF-8", "zh_CN.UTF-8"} {
		t.Setenv("LC_ALL", locale)
		for _, format := range []string{"text", "json", "yaml"} {
			for _, command := range [][]string{{"env", "set", "TEST_KEY", secret, "--yes"}, {"env", "list"}, {"env", "set", "TEST_KEY", secret, "--global", "--path", "/", "--env", "dev", "--yes"}, {"env", "list", "--global", "--path", "/", "--env", "dev"}} {
				for _, failing := range []bool{false, true} {
					fail.Store(failing)
					out, errOut, code := runAuthenticatedCLI(t, root, server.URL, append(command, "-o", format)...)
					if (code != 0) != failing {
						t.Fatalf("command failed unexpectedly: %d %s %s", code, out, errOut)
					}
					if strings.Contains(out+errOut, secret) || strings.Contains(out+errOut, "test-session-token") {
						t.Fatal("secret or session token leaked")
					}
					if !failing && format != "text" && !strings.Contains(out, "TEST_KEY") {
						t.Fatalf("missing key metadata: %s %s", out, errOut)
					}
				}
			}
		}
	}

	if runtime.GOOS != "windows" {
		fail.Store(false)
		out, errOut, code := runAuthenticatedCLI(t, root, server.URL, "exec", "--global", "--path", "/", "--env", "dev", "--", "sh", "-c", `printf '%s\n' "$TEST_KEY"; printf '%s\n' "$TEST_KEY" >&2; exit 13`)
		if code != 13 || strings.Contains(out+errOut, secret) || !strings.Contains(out, "[REDACTED]") || !strings.Contains(errOut, "[REDACTED]") {
			t.Fatalf("unsafe global exec: %d %s %s", code, out, errOut)
		}
	}
}
