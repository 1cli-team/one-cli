package cloudflare

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// argv builders ----------------------------------------------------------

func TestBuildDeployArgvProduction(t *testing.T) {
	got := buildDeployArgv(ApplyInput{
		APIToken:  "tok-123",
		AccountID: "acct",
		Env:       "prod",
	})
	want := []string{CLIBinary, "deploy"}
	assertArgv(t, got, want)
}

// Empty Env defaults to production (no --env flag); preserves the pre-v4
// behaviour where the unset preview field meant "ship to prod".
func TestBuildDeployArgvEmptyEnvDefaultsToProduction(t *testing.T) {
	got := buildDeployArgv(ApplyInput{
		APIToken: "tok-123",
	})
	want := []string{CLIBinary, "deploy"}
	assertArgv(t, got, want)
}

func TestBuildDeployArgvNamedEnv(t *testing.T) {
	got := buildDeployArgv(ApplyInput{
		APIToken: "tok-123",
		Env:      "staging",
	})
	want := []string{CLIBinary, "deploy", "--env=staging"}
	assertArgv(t, got, want)
}

func TestBuildDeployArgvDevEnv(t *testing.T) {
	got := buildDeployArgv(ApplyInput{
		APIToken: "tok-123",
		Env:      "dev",
	})
	want := []string{CLIBinary, "deploy", "--env=dev"}
	assertArgv(t, got, want)
}

// argv must never contain auth material — wrangler reads token /
// account from env, so the argv is a clean public-safe value.

func TestBuildDeployArgvNeverIncludesToken(t *testing.T) {
	got := buildDeployArgv(ApplyInput{
		APIToken:  "tok-secret-xyz",
		AccountID: "acct-abc",
		Env:       "prod",
	})
	for _, a := range got {
		if strings.Contains(a, "tok-secret-xyz") {
			t.Fatalf("argv leaked token: %v", got)
		}
		if strings.Contains(a, "acct-abc") {
			t.Fatalf("argv leaked account id: %v", got)
		}
	}
}

// wranglerEnv: the token is injected into env, the account id only when
// non-empty, and any pre-existing CLOUDFLARE_API_TOKEN in the parent
// shell is replaced (profile is the source of truth).

func TestWranglerEnvInjectsToken(t *testing.T) {
	parent := []string{"FOO=bar", "PATH=/usr/bin"}
	env := wranglerEnv(parent, nil, "tok-1", "")
	if !containsString(env, "CLOUDFLARE_API_TOKEN=tok-1") {
		t.Fatalf("env missing token: %v", env)
	}
	if containsPrefix(env, "CLOUDFLARE_ACCOUNT_ID=") {
		t.Fatalf("empty account id should not be set: %v", env)
	}
	// Parent vars preserved.
	if !containsString(env, "FOO=bar") {
		t.Fatalf("parent env not preserved: %v", env)
	}
}

func TestWranglerEnvInjectsAccountIDWhenSet(t *testing.T) {
	env := wranglerEnv(nil, nil, "tok-1", "acct-xyz")
	if !containsString(env, "CLOUDFLARE_ACCOUNT_ID=acct-xyz") {
		t.Fatalf("env missing account id: %v", env)
	}
}

func TestWranglerEnvOverridesPreExistingValues(t *testing.T) {
	parent := []string{
		"CLOUDFLARE_API_TOKEN=stale-token",
		"CLOUDFLARE_ACCOUNT_ID=stale-acct",
	}
	env := wranglerEnv(parent, nil, "fresh-tok", "fresh-acct")
	if containsString(env, "CLOUDFLARE_API_TOKEN=stale-token") {
		t.Fatalf("stale token leaked: %v", env)
	}
	if containsString(env, "CLOUDFLARE_ACCOUNT_ID=stale-acct") {
		t.Fatalf("stale account id leaked: %v", env)
	}
	if !containsString(env, "CLOUDFLARE_API_TOKEN=fresh-tok") {
		t.Fatalf("fresh token missing: %v", env)
	}
}

// Injected env tests: project env vars merge into wrangler's env, but auth
// env vars always win (including against an injected map that contains a
// matching name).

func TestWranglerEnvAuthWinsOverInjected(t *testing.T) {
	injected := map[string]string{
		"CLOUDFLARE_API_TOKEN":  "user-tried-to-override",
		"CLOUDFLARE_ACCOUNT_ID": "user-tried-acct",
		"API_URL":               "https://api.example.com",
	}
	env := wranglerEnv(nil, injected, "profile-tok", "profile-acct")
	if !containsString(env, "CLOUDFLARE_API_TOKEN=profile-tok") {
		t.Fatalf("profile token not winning: %v", env)
	}
	if containsString(env, "CLOUDFLARE_API_TOKEN=user-tried-to-override") {
		t.Fatalf("injected api token leaked: %v", env)
	}
	if !containsString(env, "CLOUDFLARE_ACCOUNT_ID=profile-acct") {
		t.Fatalf("profile account id not winning: %v", env)
	}
	// Non-auth user vars do go through.
	if !containsString(env, "API_URL=https://api.example.com") {
		t.Fatalf("non-auth injected var missing: %v", env)
	}
}

func TestWranglerEnvInjectedOverridesShell(t *testing.T) {
	parent := []string{"API_URL=stale", "DATABASE_HOST=ignored-shell"}
	injected := map[string]string{"API_URL": "fresh", "FEATURE_FLAGS": "a,b"}
	env := wranglerEnv(parent, injected, "tok", "")
	if !containsString(env, "API_URL=fresh") {
		t.Fatalf("injected API_URL should override shell: %v", env)
	}
	if containsString(env, "API_URL=stale") {
		t.Fatalf("stale shell value leaked: %v", env)
	}
	if !containsString(env, "FEATURE_FLAGS=a,b") {
		t.Fatalf("new injected key missing: %v", env)
	}
	// Shell-only vars are still passed through.
	if !containsString(env, "DATABASE_HOST=ignored-shell") {
		t.Fatalf("shell-only var dropped: %v", env)
	}
}

func TestWranglerEnvNilInjectedNoop(t *testing.T) {
	parent := []string{"FOO=bar"}
	env := wranglerEnv(parent, nil, "tok", "")
	if !containsString(env, "FOO=bar") {
		t.Fatalf("parent vars dropped: %v", env)
	}
	if !containsString(env, "CLOUDFLARE_API_TOKEN=tok") {
		t.Fatalf("auth env missing: %v", env)
	}
}

// Apply: dry-run path ----------------------------------------------------

func TestApplyDryRunReturnsCleanArgvAndCommandLines(t *testing.T) {
	res, err := Apply(context.Background(), ApplyInput{
		ProjectDir: t.TempDir(),
		APIToken:   "tok-dry",
		AccountID:  "acct-dry",
		Env:        "prod",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res == nil {
		t.Fatal("Apply returned nil result")
	}
	if !res.DryRun {
		t.Errorf("DryRun = false, want true")
	}
	if res.Schema != SchemaApply {
		t.Errorf("Schema = %q, want %q", res.Schema, SchemaApply)
	}
	for _, a := range res.Argv {
		if strings.Contains(a, "tok-dry") {
			t.Fatalf("dry-run argv leaked token: %v", res.Argv)
		}
	}
	if len(res.CommandLines) != 1 {
		t.Fatalf("CommandLines len = %d, want 1 (deploy)", len(res.CommandLines))
	}
	if !strings.Contains(res.CommandLines[0], "wrangler deploy") {
		t.Errorf("command line should be `wrangler deploy ...`, got %q", res.CommandLines[0])
	}
	if strings.Contains(res.CommandLines[0], "tok-dry") {
		t.Fatalf("CommandLines leaked token: %q", res.CommandLines[0])
	}
}

// Apply: validation paths -------------------------------------------------

func TestApplyEmptyTokenSurfacesCloudflareProfileInvalid(t *testing.T) {
	_, err := Apply(context.Background(), ApplyInput{
		ProjectDir: t.TempDir(),
		APIToken:   "",
		DryRun:     true,
	})
	if err == nil {
		t.Fatal("expected error for empty APIToken")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "CLOUDFLARE_PROFILE_INVALID" {
		t.Fatalf("error code = %v, want CLOUDFLARE_PROFILE_INVALID", err)
	}
}

// Apply: real exec via fake wrangler binary -------------------------------

func TestApplyRealExecCapturesDeploymentURL(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "wrangler.log")
	installFakeWrangler(t, tmp, logPath, "https://demo.example.workers.dev")

	res, err := Apply(context.Background(), ApplyInput{
		ProjectDir: tmp,
		APIToken:   "tok-real",
		AccountID:  "acct-real",
		Env:        "prod",
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	if res.DryRun {
		t.Errorf("DryRun = true, want false")
	}
	if res.DeploymentURL != "https://demo.example.workers.dev" {
		t.Errorf("DeploymentURL = %q, want https://demo.example.workers.dev", res.DeploymentURL)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "deploy") {
		t.Errorf("log missing `deploy` invocation\n--- log:\n%s", got)
	}
	// fake wrangler dumps env + argv. Confirm token + account were
	// threaded via env (not argv).
	if !strings.Contains(got, "CLOUDFLARE_API_TOKEN=tok-real") {
		t.Errorf("log missing token in env\n--- log:\n%s", got)
	}
	if !strings.Contains(got, "CLOUDFLARE_ACCOUNT_ID=acct-real") {
		t.Errorf("log missing account id in env\n--- log:\n%s", got)
	}
	for _, a := range res.Argv {
		if strings.Contains(a, "tok-real") {
			t.Fatalf("real-run argv leaked token: %v", res.Argv)
		}
	}
}

func TestApplyRealExecFindsProjectLocalWrangler(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "project")
	logPath := filepath.Join(tmp, "wrangler.log")
	installFakeWranglerAt(t,
		filepath.Join(projectDir, "node_modules", ".bin"),
		logPath,
		"https://local.example.workers.dev")
	emptyPath := filepath.Join(tmp, "empty-path")
	if err := os.MkdirAll(emptyPath, 0o755); err != nil {
		t.Fatalf("mkdir empty path: %v", err)
	}
	t.Setenv("PATH", emptyPath)

	res, err := Apply(context.Background(), ApplyInput{
		ProjectDir: projectDir,
		APIToken:   "tok-local",
		AccountID:  "acct-local",
		Env:        "prod",
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.DeploymentURL != "https://local.example.workers.dev" {
		t.Errorf("DeploymentURL = %q, want project-local URL", res.DeploymentURL)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "CLOUDFLARE_API_TOKEN=tok-local") {
		t.Fatalf("project-local wrangler did not receive token env\n--- log:\n%s", got)
	}
	if !strings.Contains(res.Argv[0], filepath.Join("node_modules", ".bin", "wrangler")) {
		t.Fatalf("expected actual argv to use project-local wrangler, got %v", res.Argv)
	}
}

func TestApplyMissingCLISurfacesCloudflareCLIMissing(t *testing.T) {
	prevBinary := CLIBinary
	t.Cleanup(func() { CLIBinary = prevBinary })
	CLIBinary = "wrangler-this-binary-does-not-exist-xyz"
	t.Setenv("PATH", t.TempDir())

	_, err := Apply(context.Background(), ApplyInput{
		ProjectDir: t.TempDir(),
		APIToken:   "tok",
		DryRun:     false,
	})
	if err == nil {
		t.Fatal("expected error when wrangler CLI missing")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "CLOUDFLARE_CLI_MISSING" {
		t.Fatalf("error = %v, want CLOUDFLARE_CLI_MISSING", err)
	}
}

func TestRunDeployStepAddsD1RemediationWhenBindingConfigured(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, WranglerConfigFilename), []byte(`
name = "demo"

[[d1_databases]]
binding = "DB"
database_name = "demo-db"
database_id = "missing-id"
`), 0o644); err != nil {
		t.Fatalf("write wrangler.toml: %v", err)
	}
	failingWrangler := filepath.Join(tmp, "wrangler")
	if err := os.WriteFile(failingWrangler, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write failing wrangler: %v", err)
	}

	_, err := runDeployStep(context.Background(), tmp, []string{failingWrangler, "deploy"}, "tok", "", nil)
	if err == nil {
		t.Fatal("expected deploy failure")
	}
	outErr, ok := err.(*output.Error)
	if !ok {
		t.Fatalf("error type = %T, want *output.Error", err)
	}
	if outErr.Context["d1_binding_configured"] != true {
		t.Fatalf("d1 context missing: %v", outErr.Context)
	}
	if !hasRemediationAction(outErr.Remediation, "verify-d1-binding") {
		t.Fatalf("D1 remediation missing: %+v", outErr.Remediation)
	}
}

// helpers ----------------------------------------------------------------

func assertArgv(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("argv len = %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q\nfull got:  %v\nfull want: %v", i, got[i], want[i], got, want)
		}
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func containsPrefix(haystack []string, prefix string) bool {
	for _, s := range haystack {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func hasRemediationAction(steps []output.Remediation, action string) bool {
	for _, step := range steps {
		if step.Action == action {
			return true
		}
	}
	return false
}

// installFakeWrangler writes a shell script under <dir>/bin/wrangler
// that logs every invocation (env + argv) to logPath and prints a fake
// deploy URL on the `deploy` subcommand. Prepends <dir>/bin to $PATH
// for the duration of the test.
func installFakeWrangler(t *testing.T, dir, logPath, urlOnDeploy string) {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	installFakeWranglerAt(t, bin, logPath, urlOnDeploy)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func installFakeWranglerAt(t *testing.T, bin, logPath, urlOnDeploy string) {
	t.Helper()
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	path := filepath.Join(bin, "wrangler")
	body := `#!/bin/sh
{
  echo "argv: $@"
  echo "CLOUDFLARE_API_TOKEN=$CLOUDFLARE_API_TOKEN"
  echo "CLOUDFLARE_ACCOUNT_ID=$CLOUDFLARE_ACCOUNT_ID"
} >> "$WRANGLER_LOG"
case "$1" in
  deploy)
    printf 'Total Upload: 1.23 KiB\n'
    printf 'Published demo (1.2 sec)\n'
    printf '  ` + urlOnDeploy + `\n'
    printf 'Current Deployment ID: abc-123\n'
    ;;
  *)
    :
    ;;
esac
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake wrangler: %v", err)
	}
	t.Setenv("WRANGLER_LOG", logPath)
}
