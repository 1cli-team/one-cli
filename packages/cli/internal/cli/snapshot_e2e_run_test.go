package cli_test

// E2E coverage of `one run`.
//
// `one run` is an exec-passthrough: it loads the resolved subproject's
// .env into the child process environment and execs the requested command.
// The contract these tests pin down:
//   - no args: prints help (Long usage block) and exits 0
//   - happy path: cd into a subproject with a .env, exec a command,
//     stdout passes through, exit code passes through, .env vars are
//     visible inside the child
//   - --override flips the merge order so .env beats inherited shell vars
//   - missing .env → runs leniently with zero injected vars (not an error)
//   - unknown executable → RUN_COMMAND_NOT_FOUND envelope
//   - -p / --project resolves a subproject by manifest name or relativeDir
//     from any cwd
//
// We don't snapshot stdout — exec passthrough is byte-exact and any drift
// would be a real bug, not a fixture-update event.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// addSubprojectWithDotenv bootstraps a workspace, scaffolds an go-api
// subproject under services/<name>, drops a deterministic .env into it,
// and returns (workspaceRoot, subprojectDir).
func addSubprojectWithDotenv(t *testing.T, name string, dotenv string) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	stdout, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", name, "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add %s failed: exit %d\n  stdout: %s\n  stderr: %s", name, code, stdout, stderr)
	}
	subDir := filepath.Join(ws, "services", name)
	if err := os.WriteFile(filepath.Join(subDir, ".env"), []byte(dotenv), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	return ws, subDir
}

func TestSnapshot_E2E_Run_NoArgs_PrintsHelp(t *testing.T) {
	// `one run` 不带任何位置参数时应当像父命令一样打印 help 并 exit 0，
	// 而不是返回 cobra 的 "requires at least 1 arg(s)" JSON 错误。
	tmp := t.TempDir()
	isolateHome(t, tmp)

	stdout, stderr, code := runBinaryIn(t, tmp, "run")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}
	// Long help 的稳定锚点：第一行短描述里的关键短语。换文案时只需调一次。
	if !strings.Contains(stdout, "注入") {
		t.Errorf("expected help text on stdout (containing %q), got: %q", "注入", stdout)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("expected cobra Usage block on stdout, got: %q", stdout)
	}
}

func TestSnapshot_E2E_Run_HappyPath(t *testing.T) {
	_, subDir := addSubprojectWithDotenv(t, "user-api",
		"RUN_TEST_KEY=hello-from-dotenv\n")

	// Use sh -c so we can both observe stdout and read $RUN_TEST_KEY in
	// one shot. printenv is portable; -- separator pins the contract for
	// users who type it explicitly.
	stdout, stderr, code := runBinaryIn(t, subDir, "run", "--",
		"sh", "-c", "printenv RUN_TEST_KEY")
	if code != 0 {
		t.Fatalf("run failed: exit %d\n  stdout: %q\n  stderr: %q", code, stdout, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "hello-from-dotenv" {
		t.Errorf("stdout: want %q, got %q", "hello-from-dotenv", got)
	}
}

func TestSnapshot_E2E_Run_ExitCodePassthrough(t *testing.T) {
	_, subDir := addSubprojectWithDotenv(t, "billing", "K=v\n")

	_, _, code := runBinaryIn(t, subDir, "run", "--",
		"sh", "-c", "exit 42")
	if code != 42 {
		t.Errorf("expected child exit code 42 to passthrough, got %d", code)
	}
}

func TestSnapshot_E2E_Run_MissingDotenv_RunsLeniently(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	// Add a subproject but DO NOT write a .env. `one run` should now
	// continue with zero injected variables instead of erroring — `.env`
	// is gitignored and optional, so the first-run experience must not
	// require it.
	stdout, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "no-env", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}
	subDir := filepath.Join(ws, "services", "no-env")

	stdout, stderr, code = runBinaryIn(t, subDir, "run", "--", "echo", "hi")
	if code != 0 {
		t.Fatalf("expected exit 0 with no .env, got %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "hi") {
		t.Errorf("stdout: want %q, got %q", "hi", stdout)
	}
}

func TestSnapshot_E2E_Run_UnknownCommand_ReturnsStructuredError(t *testing.T) {
	_, subDir := addSubprojectWithDotenv(t, "billing", "K=v\n")

	// A name that almost certainly isn't on $PATH. If a future CI image
	// somehow ships this binary, the test will yell loudly — the random
	// suffix is deliberately unguessable.
	_, stderr, code := runBinaryIn(t, subDir, "run", "-o", "json", "--",
		"definitely-not-a-real-binary-xZQ7p9")
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown command, got 0\n  stderr: %s", stderr)
	}
	envelope := firstJSONLine(stderr)
	if envelope == "" {
		t.Fatalf("expected JSON error envelope on stderr, got: %q", stderr)
	}
	got := mustParseJSON(t, envelope)
	errMap, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("envelope missing error object: %s", envelope)
	}
	if errMap["code"] != "RUN_COMMAND_NOT_FOUND" {
		t.Errorf("error.code: want RUN_COMMAND_NOT_FOUND, got %v", errMap["code"])
	}
}

func TestSnapshot_E2E_Run_DefaultOverwrite(t *testing.T) {
	ws, subDir := addSubprojectWithDotenv(t, "auth",
		"OVERRIDE_KEY=value-from-dotenv\n")

	// Default behaviour (v0.8+): injected secrets always overwrite shell vars.
	t.Setenv("OVERRIDE_KEY", "value-from-shell")

	stdout, stderr, code := runBinaryIn(t, subDir, "run", "--",
		"sh", "-c", "printenv OVERRIDE_KEY")
	if code != 0 {
		t.Fatalf("default run failed: exit %d\n  stderr: %s", code, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "value-from-dotenv" {
		t.Errorf("default merge: want dotenv to win, got %q", got)
	}

	// Sanity: -p resolves the same subproject from workspace root, both by
	// relative path and by manifest name (v0.7+ name-based selection).
	stdout, stderr, code = runBinaryIn(t, ws, "run", "-p", "services/auth", "--",
		"sh", "-c", "printenv OVERRIDE_KEY")
	if code != 0 {
		t.Fatalf("-p path run failed: exit %d\n  stderr: %s", code, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "value-from-dotenv" {
		t.Errorf("-p path resolution: want dotenv to win, got %q", got)
	}

	stdout, stderr, code = runBinaryIn(t, ws, "run", "-p", "auth", "--",
		"sh", "-c", "printenv OVERRIDE_KEY")
	if code != 0 {
		t.Fatalf("-p name run failed: exit %d\n  stderr: %s", code, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "value-from-dotenv" {
		t.Errorf("-p name resolution: want dotenv to win, got %q", got)
	}
}
