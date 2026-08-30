package edgeone

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// argv builders ----------------------------------------------------------

func TestBuildDeployArgvProductionDefaultDir(t *testing.T) {
	got := buildDeployArgv(ApplyInput{
		APIToken:    "token",
		ProjectName: "demo",
		Env:         "prod",
	})
	want := []string{CLIBinary, "pages", "deploy", ".", "--name=demo", "--token", "token"}
	assertArgv(t, got, want)
}

// Empty Env defaults to the production tier (mirrors the pre-v4 behaviour
// where the unset preview field meant "ship to prod").
func TestBuildDeployArgvEmptyEnvDefaultsToProduction(t *testing.T) {
	got := buildDeployArgv(ApplyInput{
		APIToken:    "token",
		ProjectName: "demo",
	})
	want := []string{CLIBinary, "pages", "deploy", ".", "--name=demo", "--token", "token"}
	assertArgv(t, got, want)
}

func TestBuildDeployArgvPreviewWithCustomAssetDir(t *testing.T) {
	got := buildDeployArgv(ApplyInput{
		APIToken:    "token",
		AssetDir:    "dist",
		ProjectName: "demo",
		Env:         "dev",
	})
	want := []string{CLIBinary, "pages", "deploy", "dist", "--name=demo", "--token", "token", "--env=preview"}
	assertArgv(t, got, want)
}

// Any non-prod env collapses to preview (EdgeOne only has two tiers).
func TestBuildDeployArgvStagingCollapsesToPreview(t *testing.T) {
	got := buildDeployArgv(ApplyInput{
		APIToken:    "token",
		AssetDir:    "dist",
		ProjectName: "demo",
		Env:         "staging",
	})
	want := []string{CLIBinary, "pages", "deploy", "dist", "--name=demo", "--token", "token", "--env=preview"}
	assertArgv(t, got, want)
}

func TestBuildDeployArgvWithoutProjectName(t *testing.T) {
	got := buildDeployArgv(ApplyInput{
		APIToken: "token",
		AssetDir: "dist",
		Env:      "prod",
	})
	want := []string{CLIBinary, "pages", "deploy", "dist", "--token", "token"}
	assertArgv(t, got, want)
}

func TestRedactedDeployArgvHidesToken(t *testing.T) {
	got := redactedDeployArgv(ApplyInput{
		APIToken:    "edgeone-secret-token",
		ProjectName: "demo",
		Env:         "prod",
	})
	for _, a := range got {
		if strings.Contains(a, "edgeone-secret-token") {
			t.Fatalf("redacted argv leaked token: %v", got)
		}
	}
	if !containsString(got, "--token") || !containsString(got, redactedToken) {
		t.Fatalf("redacted token missing: %v", got)
	}
}

func TestEdgeOneEnvInjectedVarsOverrideShell(t *testing.T) {
	parent := []string{"API_URL=stale", "DATABASE_HOST=ignored-shell"}
	injected := map[string]string{"API_URL": "fresh", "FEATURE_FLAGS": "a,b"}
	env := edgeoneEnv(parent, injected)
	if !containsString(env, "API_URL=fresh") {
		t.Fatalf("injected API_URL should override shell: %v", env)
	}
	if containsString(env, "API_URL=stale") {
		t.Fatalf("stale shell value leaked: %v", env)
	}
	if !containsString(env, "FEATURE_FLAGS=a,b") {
		t.Fatalf("new injected key missing: %v", env)
	}
	if !containsString(env, "DATABASE_HOST=ignored-shell") {
		t.Fatalf("shell-only var dropped: %v", env)
	}
}

func TestEdgeOneEnvNilInjectedNoop(t *testing.T) {
	parent := []string{"FOO=bar"}
	env := edgeoneEnv(parent, nil)
	if !containsString(env, "FOO=bar") {
		t.Fatalf("parent vars dropped: %v", env)
	}
}

// Apply: dry-run path ----------------------------------------------------

func TestApplyDryRunReturnsCleanArgvAndCommandLines(t *testing.T) {
	res, err := Apply(context.Background(), ApplyInput{
		ProjectDir:  t.TempDir(),
		APIToken:    "token-dry",
		ProjectName: "demo",
		Env:         "prod",
		DryRun:      true,
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
		if strings.Contains(a, "token-dry") {
			t.Fatalf("dry-run argv leaked token: %v", res.Argv)
		}
	}
	if len(res.CommandLines) != 1 {
		t.Fatalf("CommandLines len = %d, want 1", len(res.CommandLines))
	}
	if !strings.Contains(res.CommandLines[0], "edgeone pages deploy") {
		t.Errorf("command line should be `edgeone pages deploy ...`, got %q", res.CommandLines[0])
	}
}

func TestApplyDryRunWithoutTokenUsesLoginState(t *testing.T) {
	res, err := Apply(context.Background(), ApplyInput{
		ProjectDir: t.TempDir(),
		APIToken:   "",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, a := range res.Argv {
		if a == "--token" {
			t.Fatalf("unexpected token flag without token: %v", res.Argv)
		}
	}
}

// Apply: real exec via fake edgeone binary --------------------------------

func TestApplyRealExecCapturesDeploymentURL(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "edgeone.log")
	installFakeEdgeOne(t, tmp, logPath, "https://demo.eo.dev")

	res, err := Apply(context.Background(), ApplyInput{
		ProjectDir:  tmp,
		APIToken:    "token-real",
		Region:      "ap-guangzhou",
		ProjectName: "demo",
		Env:         "prod",
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	if res.DeploymentURL != "https://demo.eo.dev" {
		t.Errorf("DeploymentURL = %q, want https://demo.eo.dev", res.DeploymentURL)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "--token token-real") {
		t.Errorf("log missing token flag\n--- log:\n%s", got)
	}
	for _, a := range res.Argv {
		if strings.Contains(a, "token-real") {
			t.Fatalf("real-run argv leaked token: %v", res.Argv)
		}
	}
}

func TestApplyMissingCLISurfacesEdgeOneCLIMissing(t *testing.T) {
	prevBinary := CLIBinary
	t.Cleanup(func() { CLIBinary = prevBinary })
	CLIBinary = "edgeone-this-binary-does-not-exist-xyz"
	t.Setenv("PATH", t.TempDir())

	_, err := Apply(context.Background(), ApplyInput{
		ProjectDir: t.TempDir(),
		APIToken:   "token",
		DryRun:     false,
	})
	if err == nil {
		t.Fatal("expected error when edgeone CLI missing")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "EDGEONE_CLI_MISSING" {
		t.Fatalf("error = %v, want EDGEONE_CLI_MISSING", err)
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

// installFakeEdgeOne writes a shell script under <dir>/bin/edgeone that
// logs every invocation (env + argv) to logPath and prints a fake
// deploy URL on the `pages deploy` subcommand.
func installFakeEdgeOne(t *testing.T, dir, logPath, urlOnDeploy string) {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	path := filepath.Join(bin, "edgeone")
	body := `#!/bin/sh
{
  echo "argv: $@"
} >> "$EDGEONE_LOG"
case "$1" in
  pages)
    if [ "$2" = "deploy" ]; then
      printf 'Uploading assets to EdgeOne Pages...\n'
      printf 'Deployment ready at: ` + urlOnDeploy + `\n'
    fi
    ;;
  *)
    :
    ;;
esac
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake edgeone: %v", err)
	}
	t.Setenv("EDGEONE_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
