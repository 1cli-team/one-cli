package vercel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// argv builders ----------------------------------------------------------

func TestBuildPullArgvProduction(t *testing.T) {
	got := buildPullArgv(ApplyInput{
		APIToken: "tok-123",
		Team:     "acme",
		Env:      "prod",
	})
	want := []string{
		CLIBinary, "pull", "--yes",
		"--environment=production",
		"--scope=acme",
		"--token=tok-123",
	}
	assertArgv(t, got, want)
}

func TestBuildPullArgvPreviewNoTeam(t *testing.T) {
	got := buildPullArgv(ApplyInput{
		APIToken: "tok-456",
		Env:      "dev",
	})
	want := []string{
		CLIBinary, "pull", "--yes",
		"--environment=preview",
		"--token=tok-456",
	}
	assertArgv(t, got, want)
}

// Empty Env defaults to the production tier (mirrors the pre-v4 behaviour
// where the unset preview field meant "ship to prod").
func TestBuildPullArgvEmptyEnvDefaultsToProduction(t *testing.T) {
	got := buildPullArgv(ApplyInput{
		APIToken: "tok-default",
	})
	want := []string{
		CLIBinary, "pull", "--yes",
		"--environment=production",
		"--token=tok-default",
	}
	assertArgv(t, got, want)
}

func TestBuildBuildArgvProduction(t *testing.T) {
	got := buildBuildArgv(ApplyInput{
		APIToken: "tok-789",
		Env:      "prod",
	})
	want := []string{CLIBinary, "build", "--prod", "--token=tok-789"}
	assertArgv(t, got, want)
}

// Staging / dev / any non-prod env collapses to preview on Vercel
// (the platform exposes only two tiers).
func TestBuildBuildArgvStagingCollapsesToPreview(t *testing.T) {
	got := buildBuildArgv(ApplyInput{
		APIToken: "tok-stg",
		Env:      "staging",
	})
	want := []string{CLIBinary, "build", "--token=tok-stg"}
	assertArgv(t, got, want)
}

func TestBuildDeployArgvProduction(t *testing.T) {
	got := buildDeployArgv(ApplyInput{
		APIToken: "tok-abc",
		Team:     "myteam",
		Env:      "prod",
	})
	want := []string{
		CLIBinary, "deploy", "--prebuilt", "--prod",
		"--scope=myteam",
		"--token=tok-abc",
	}
	assertArgv(t, got, want)
}

// masking ----------------------------------------------------------------

func TestMaskTokenInArgvReplacesTokenValueOnly(t *testing.T) {
	in := []string{"vercel", "deploy", "--prebuilt", "--prod", "--scope=acme", "--token=secret-token-xyz"}
	got := maskTokenInArgv(in)
	for _, a := range got {
		if strings.Contains(a, "secret-token-xyz") {
			t.Fatalf("argv leaked token: %v", got)
		}
	}
	// Check the masked entry is exactly --token=********
	last := got[len(got)-1]
	if last != "--token=********" {
		t.Fatalf("last argv = %q, want --token=********", last)
	}
	// Other entries must be untouched (--scope=acme stays).
	if got[len(got)-2] != "--scope=acme" {
		t.Fatalf("scope arg got mangled: %q", got[len(got)-2])
	}
}

func TestMaskTokenInArgvLeavesArgvWithoutTokenAlone(t *testing.T) {
	in := []string{"vercel", "build", "--prod"}
	got := maskTokenInArgv(in)
	assertArgv(t, got, in)
}

// Apply: dry-run path ----------------------------------------------------

func TestApplyDryRunReturnsMaskedArgvAndCommandLines(t *testing.T) {
	res, err := Apply(context.Background(), ApplyInput{
		ProjectDir: t.TempDir(),
		APIToken:   "tok-dry",
		Team:       "team-dry",
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
	// argv should not contain plaintext token
	for _, a := range res.Argv {
		if strings.Contains(a, "tok-dry") {
			t.Fatalf("dry-run argv leaked token: %v", res.Argv)
		}
	}
	if len(res.CommandLines) != 3 {
		t.Fatalf("CommandLines len = %d, want 3 (pull / build / deploy)", len(res.CommandLines))
	}
	for i, line := range res.CommandLines {
		if strings.Contains(line, "tok-dry") {
			t.Fatalf("CommandLines[%d] leaked token: %q", i, line)
		}
	}
	// Sanity: lines mention the three steps in order.
	if !strings.Contains(res.CommandLines[0], "vercel pull --yes") {
		t.Errorf("first command line should be `vercel pull --yes ...`, got %q", res.CommandLines[0])
	}
	if !strings.Contains(res.CommandLines[1], "vercel build") {
		t.Errorf("second command line should be `vercel build ...`, got %q", res.CommandLines[1])
	}
	if !strings.Contains(res.CommandLines[2], "vercel deploy --prebuilt") {
		t.Errorf("third command line should be `vercel deploy --prebuilt ...`, got %q", res.CommandLines[2])
	}
}

// Apply: validation paths -------------------------------------------------

func TestApplyEmptyTokenSurfacesVercelProfileInvalid(t *testing.T) {
	_, err := Apply(context.Background(), ApplyInput{
		ProjectDir: t.TempDir(),
		APIToken:   "",
		DryRun:     true,
	})
	if err == nil {
		t.Fatal("expected error for empty APIToken")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "VERCEL_PROFILE_INVALID" {
		t.Fatalf("error code = %v, want VERCEL_PROFILE_INVALID", err)
	}
}

// Apply: real exec via fake vercel binary --------------------------------

func TestApplyRealExecCapturesDeploymentURL(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "vercel.log")
	installFakeVercel(t, tmp, logPath, "https://demo-app-abc123.vercel.app")

	res, err := Apply(context.Background(), ApplyInput{
		ProjectDir: tmp,
		APIToken:   "tok-real",
		Team:       "acme",
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
	if res.DeploymentURL != "https://demo-app-abc123.vercel.app" {
		t.Errorf("DeploymentURL = %q, want https://demo-app-abc123.vercel.app", res.DeploymentURL)
	}
	// log should record three invocations: pull / build / deploy
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(raw)
	for _, want := range []string{
		"pull --yes --environment=production --scope=acme --token=tok-real",
		"build --prod --scope=acme --token=tok-real",
		"deploy --prebuilt --prod --scope=acme --token=tok-real",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log missing %q\n--- log:\n%s", want, got)
		}
	}
	// envelope argv must mask the token even after a real run
	for _, a := range res.Argv {
		if strings.Contains(a, "tok-real") {
			t.Fatalf("real-run argv leaked token: %v", res.Argv)
		}
	}
}

func TestApplyMissingCLISurfacesVercelCLIMissing(t *testing.T) {
	// Force LookPath to fail by pointing PATH at an empty dir AND
	// renaming the binary we're looking for so the system vercel (if
	// any) is invisible.
	prevBinary := CLIBinary
	t.Cleanup(func() { CLIBinary = prevBinary })
	CLIBinary = "vercel-this-binary-does-not-exist-xyz"
	t.Setenv("PATH", t.TempDir())

	_, err := Apply(context.Background(), ApplyInput{
		ProjectDir: t.TempDir(),
		APIToken:   "tok",
		DryRun:     false,
	})
	if err == nil {
		t.Fatal("expected error when vercel CLI missing")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "VERCEL_CLI_MISSING" {
		t.Fatalf("error = %v, want VERCEL_CLI_MISSING", err)
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

// vercelEnv: injected map merges on top of the parent shell (override),
// non-injection paths are no-ops.

func TestVercelEnvMergesInjectedOverShell(t *testing.T) {
	parent := []string{"API_URL=stale", "DATABASE_HOST=ignored-shell"}
	injected := map[string]string{"API_URL": "fresh", "FEATURE_FLAGS": "a,b"}
	got := vercelEnv(parent, injected)
	if !containsString(got, "API_URL=fresh") {
		t.Fatalf("injected API_URL should override shell: %v", got)
	}
	if containsString(got, "API_URL=stale") {
		t.Fatalf("stale shell value leaked: %v", got)
	}
	if !containsString(got, "FEATURE_FLAGS=a,b") {
		t.Fatalf("new injected key missing: %v", got)
	}
	if !containsString(got, "DATABASE_HOST=ignored-shell") {
		t.Fatalf("shell-only var dropped: %v", got)
	}
}

func TestVercelEnvNilInjectedNoop(t *testing.T) {
	parent := []string{"FOO=bar", "PATH=/usr/bin"}
	got := vercelEnv(parent, nil)
	if len(got) != len(parent) {
		t.Fatalf("nil injection should be a no-op, got %v", got)
	}
}

// .vercel/.env.* invariant: Apply must NOT write any files into
// <ProjectDir>/.vercel/. Local env injection is process-env-only;
// touching .vercel/.env.* would create a second source of truth that
// fights the cloud-pulled env files.

func TestApplyNeverWritesDotVercelEnvFiles(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "vercel.log")
	installFakeVercel(t, tmp, logPath, "https://demo-app-abc123.vercel.app")

	_, err := Apply(context.Background(), ApplyInput{
		ProjectDir: tmp,
		APIToken:   "tok-real",
		Env:        "prod",
		DryRun:     false,
		InjectedEnv: map[string]string{
			"API_URL":       "https://api.example.com",
			"FEATURE_FLAGS": "a,b",
		},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	dotVercel := filepath.Join(tmp, ".vercel")
	entries, err := os.ReadDir(dotVercel)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read .vercel: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".env") {
			t.Fatalf("Apply leaked %s into .vercel/ — local env must stay in cmd.Env, never on disk", e.Name())
		}
	}
}

// containsString is a local helper duplicated from cloudflare's tests.
func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// installFakeVercel writes a shell script under <dir>/bin/vercel that
// logs every invocation to logPath and prints urlOnDeploy on the
// `deploy` subcommand. Prepends <dir>/bin to $PATH for the duration
// of the test.
func installFakeVercel(t *testing.T, dir, logPath, urlOnDeploy string) {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	path := filepath.Join(bin, "vercel")
	body := `#!/bin/sh
echo "$@" >> "$VERCEL_LOG"
case "$1" in
  deploy)
    printf '` + urlOnDeploy + `\n'
    ;;
  *)
    :
    ;;
esac
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake vercel: %v", err)
	}
	t.Setenv("VERCEL_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
