package cli_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/runtime/mise/miserelease"
)

func runtimeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	isolateHome(t, root)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, ".local", "state"))
	t.Setenv("ONE_RUNTIME", "")
	files := map[string]string{
		"one.manifest.toml": `version = 2

[workspace]
id = "runtime-test"
name = "runtime-test"

[projects."web"]
path = "apps/web"
toolchain = "node"
template = "react-spa"

[projects."api"]
path = "services/api"
toolchain = "go"
template = "go-api"
`,
		"mise.toml":             "[env]\nONE_MISE_TEST_VALUE = 'root'\nONE_MISE_PARENT = 'root-only'\n",
		"apps/web/mise.toml":    "[env]\nONE_MISE_TEST_VALUE = 'project'\nONE_MISE_ONLY = 'from-mise'\n",
		"apps/web/.env":         "ONE_MISE_TEST_VALUE=web-secret\nWEB_ONLY=web-only\n",
		"services/api/.env":     "ONE_MISE_TEST_VALUE=api-secret\nAPI_ONLY=api-only\n",
		"apps/web/package.json": `{"scripts":{"dev":"node dev.cjs"}}`,
	}
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func installFakeMise(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake mise; real integration can run on Windows")
	}
	dir := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = --version ]; then echo '2026.9.7 linux-x64'; exit 0; fi
if [ "$1" = env ]; then
 cat <<'ENV'
${Env:ONE_MISE_TEST_VALUE}='from-mise'
${Env:ONE_MISE_ONLY}='from-mise'
ENV
 exit 0
fi
[ "$1" = exec ] && [ "$2" = -- ] || exit 91
shift 2
export ONE_MISE_TEST_VALUE=from-mise
export ONE_MISE_ONLY=from-mise
exec "$@"
`

	if err := os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ONE_MISE_BINARY", filepath.Join(dir, "mise"))
}

func TestE2E_MiseRuntimeOriginalRunSyntaxAndExitCode(t *testing.T) {
	root := runtimeFixture(t)
	installFakeMise(t)
	args := append([]string{"exec", "web", "--"}, environmentEchoCommand("ONE_MISE_TEST_VALUE")...)
	out, errOut, code := runBinaryIn(t, root, args...)
	if code != 0 || strings.TrimSpace(out) != "from-mise" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out, errOut)
	}
	args = append([]string{"exec", "web", "--"}, exitWithCodeCommand(42)...)
	_, _, code = runBinaryIn(t, root, args...)
	if code != 42 {
		t.Fatalf("exit code = %d, want 42", code)
	}
	// Literal argv must survive mise and the terminal One execution leaf.
	args = []string{"exec", "web", "--", "sh", "-c", `printf '<%s>\n' "$@"`, "probe", "a b", "", `$(not-a-command)`, "中文", `a'b"c`}
	out, errOut, code = runBinaryIn(t, root, args...)
	if code != 0 || out != "<a b>\n<>\n<$(not-a-command)>\n<中文>\n<a'b\"c>\n" {
		t.Fatalf("argv: code=%d stdout=%q stderr=%q", code, out, errOut)
	}
}

func TestE2E_MisePassthroughKeepsArgumentsExitAndDoesNotLoadOneSecrets(t *testing.T) {
	root := runtimeFixture(t)
	installFakeMise(t)
	out, stderr, code := runBinaryIn(t, root, "mise", "--version")
	if code != 0 || !strings.HasPrefix(out, "2026.9.7") {
		t.Fatalf("version: %d %q %s", code, out, stderr)
	}
	out, stderr, code = runBinaryIn(t, filepath.Join(root, "apps/web"), "mise", "exec", "--", "sh", "-c", `printf '<%s>\n' "${WEB_ONLY:-not-loaded}" "$@"`, "probe", "a b", "", `$(literal)`)
	if code != 0 || out != "<not-loaded>\n<a b>\n<>\n<$(literal)>\n" {
		t.Fatalf("passthrough: %d %q %s", code, out, stderr)
	}
	_, _, code = runBinaryIn(t, root, "mise", "exec", "--", "sh", "-c", "exit 37")
	if code != 37 {
		t.Fatalf("passthrough exit = %d", code)
	}
}

func TestE2E_MiseTrustThroughOneBeforeRunningProject(t *testing.T) {
	mise := os.Getenv("ONE_TEST_MISE_BINARY")
	if mise == "" {
		t.Skip("set ONE_TEST_MISE_BINARY for real trust integration")
	}
	root := runtimeFixture(t)
	useRealMise(t, root, mise)
	t.Setenv("MISE_TRUSTED_CONFIG_PATHS", "")
	t.Setenv("MISE_PARANOID", "1")
	// Static env values need no trust; command templates deliberately do.
	if err := os.WriteFile(filepath.Join(root, "apps/web/mise.toml"), []byte(`[env]
ONE_MISE_TEST_VALUE = '{{ exec(command="echo from-mise") }}'
`), 0o644); err != nil {
		t.Fatal(err)
	}
	args := append([]string{"exec", "web", "--"}, environmentEchoCommand("ONE_MISE_TEST_VALUE")...)
	if _, _, code := runBinaryIn(t, root, args...); code == 0 {
		t.Fatal("untrusted configuration ran")
	}
	for _, rel := range []string{"mise.toml", "apps/web/mise.toml"} {
		out, stderr, code := runBinaryIn(t, root, "mise", "trust", filepath.Join(root, rel))
		if code != 0 {
			t.Fatalf("trust: %d %q %s", code, out, stderr)
		}
	}
	out, stderr, code := runBinaryIn(t, root, args...)
	if code != 0 || strings.TrimSpace(out) != "from-mise" {
		t.Fatalf("trusted run: %d %q %s", code, out, stderr)
	}
}

func TestE2E_MiseMissingReportsErrorAndLegacyStillRuns(t *testing.T) {
	root := runtimeFixture(t)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("ONE_MISE_BINARY", filepath.Join(t.TempDir(), "missing-mise"))
	out, stderr, code := runBinaryIn(t, root, "exec", "web", "--", "unavailable-command")
	if code == 0 || !strings.Contains(out+stderr, "MISE_NOT_FOUND") {
		t.Fatalf("missing mise: %d %s %s", code, out, stderr)
	}
	// Removing only the One-generated root configuration models an old workspace.
	if err := os.Remove(filepath.Join(root, "mise.toml")); err != nil {
		t.Fatal(err)
	}
	out, stderr, code = runBinaryIn(t, root, "exec", "web", "--dry-run", "--", "unavailable-command")
	if code != 0 || !strings.Contains(out, `"runtime": "builtin"`) {
		t.Fatalf("legacy runtime: %d %s %s", code, out, stderr)
	}
}

func TestE2E_MiseDryRunDoesNotRunHooksOrNeedMise(t *testing.T) {
	root := runtimeFixture(t)
	probe := filepath.Join(t.TempDir(), "mise")
	if runtime.GOOS != "windows" {
		if err := os.WriteFile(probe, []byte("#!/bin/sh\ntouch \""+filepath.Join(root, "mise-was-called")+"\"\nexit 99\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", filepath.Dir(probe)+string(os.PathListSeparator)+os.Getenv("PATH"))
		t.Setenv("ONE_MISE_BINARY", probe)
	}
	out, errOut, code := runBinaryIn(t, root, "exec", "web", "--dry-run", "--", "does-not-exist")
	if code != 0 {
		t.Fatalf("dry run: %d %s", code, errOut)
	}
	var plan struct {
		Runtime string `json:"runtime"`
		DryRun  bool   `json:"dry_run"`
	}
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Runtime != "mise" || !plan.DryRun || strings.Contains(out, "web-secret") {
		t.Fatalf("bad plan: %s", out)
	}
	for _, rel := range []string{".cache/one/runtimes/mise", ".local/share/one/runtimes/mise"} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatal("dry-run prepared the runtime")
		}
	}
	if _, err := os.Stat(filepath.Join(root, "mise-was-called")); !os.IsNotExist(err) {
		t.Fatal("preview invoked mise")
	}
}

func TestE2E_MiseRealConfigurationLayering(t *testing.T) {
	mise := os.Getenv("ONE_TEST_MISE_BINARY")
	if mise == "" {
		t.Skip("set ONE_TEST_MISE_BINARY to run the real mise integration")
	}
	root := runtimeFixture(t)
	useRealMise(t, root, mise)
	for _, pair := range []struct{ project, key, want string }{
		{"web", "ONE_MISE_TEST_VALUE", "project"}, {"web", "ONE_MISE_ONLY", "from-mise"},
		{"web", "ONE_MISE_PARENT", "root-only"}, {"api", "ONE_MISE_TEST_VALUE", "root"},
	} {
		args := append([]string{"exec", pair.project, "--"}, environmentEchoCommand(pair.key)...)
		out, stderr, code := runBinaryIn(t, root, args...)
		if code != 0 || strings.TrimSpace(out) != pair.want {
			t.Fatalf("%+v: code=%d stdout=%q stderr=%q", pair, code, out, stderr)
		}
	}
	probe := []string{"sh", "-c", `printf '%s' "${WEB_ONLY:-isolated}"`}
	if runtime.GOOS == "windows" {
		probe = []string{"cmd.exe", "/d", "/s", "/c", "if defined WEB_ONLY (echo leaked) else (echo isolated)"}
	}
	out, stderr, code := runBinaryIn(t, root, append([]string{"exec", "api", "--"}, probe...)...)
	if code != 0 || strings.TrimSpace(out) != "isolated" {
		t.Fatalf("project leak: %d %q %s", code, out, stderr)
	}
}

func useRealMise(t *testing.T, root, mise string) {
	t.Helper()
	t.Setenv("PATH", filepath.Dir(mise)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ONE_MISE_BINARY", mise)
	isolateMiseState(t, root)
}

func isolateMiseState(t *testing.T, root string) {
	t.Helper()
	for _, key := range []string{"MISE_DATA_DIR", "MISE_STATE_DIR", "MISE_CACHE_DIR", "MISE_CONFIG_DIR"} {
		t.Setenv(key, filepath.Join(root, key))
	}
	t.Setenv("MISE_TRUSTED_CONFIG_PATHS", root)
	// Never fetch tools during this environment/dispatch contract test.
	t.Setenv("MISE_AUTO_INSTALL", "false")
}

func disableMiseNetwork(t *testing.T) {
	t.Helper()
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		t.Setenv(key, "http://127.0.0.1:1")
	}
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
}

func TestE2E_MiseSystemDiscoveryThenDeletionReportsDownloadFailure(t *testing.T) {
	root := runtimeFixture(t)
	installFakeMise(t)
	system := os.Getenv("ONE_MISE_BINARY")
	t.Setenv("ONE_MISE_BINARY", "")
	t.Setenv("PATH", filepath.Dir(system))
	disableMiseNetwork(t)
	out, stderr, code := runBinaryIn(t, root, "mise", "--version")
	if code != 0 || !strings.HasPrefix(out, "2026.9.7") || strings.Contains(stderr, "Downloading") {
		t.Fatalf("system mise: %d %q %s", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".local/share/one/runtimes/mise")); !os.IsNotExist(err) {
		t.Fatal("system selection installed a managed runtime")
	}
	if err := os.Remove(system); err != nil {
		t.Fatal(err)
	}
	out, stderr, code = runBinaryIn(t, root, "mise", "--version")
	if code == 0 || !strings.Contains(stderr, "Downloading official mise") || !strings.Contains(out+stderr, "MISE_INSTALL_FAILED") {
		t.Fatalf("deleted system/offline: %d %q %s", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".local/share/one/runtimes/mise/2026.9.7")); err != nil {
		t.Fatal("runtime did not attempt managed bootstrap")
	}
}

// An opt-in official pinned binary checks real execution without putting mise
// back into build inputs. Other compatible ONE_TEST_MISE_BINARY versions still
// exercise configuration/trust tests, but cannot satisfy the managed digest.
func TestE2E_MiseManagedLegacyMigrationAndOfflineReuse(t *testing.T) {
	source := os.Getenv("ONE_TEST_MISE_BINARY")
	if source == "" {
		t.Skip("set ONE_TEST_MISE_BINARY to the pinned official mise for managed integration")
	}
	a, err := miserelease.ForPlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != a.BinarySHA256 {
		t.Skip("managed integration requires the pinned official binary digest")
	}
	root := runtimeFixture(t)
	isolateMiseState(t, root)
	t.Setenv("PATH", t.TempDir())
	disableMiseNetwork(t)
	legacy := filepath.Join(root, ".cache/one/runtimes/mise", miserelease.Version, a.Platform, a.BinaryName())
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, raw, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MISE_AUTO_UPDATE", "true")
	t.Setenv("MISE_DISABLE_UPDATE_WARNING", "false")
	out, stderr, code := runBinaryIn(t, root, "mise", "--version")
	if code != 0 || !strings.HasPrefix(out, miserelease.Version) || !strings.Contains(stderr, "Migrating verified mise") {
		t.Fatalf("migration: %d %q %s", code, out, stderr)
	}
	managed := filepath.Join(root, ".local/share/one/runtimes/mise", miserelease.Version, a.Platform, a.BinaryName())
	if _, err := os.Stat(managed); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatal("migration removed legacy binary")
	}
	out, stderr, code = runBinaryIn(t, root, "mise", "--version")
	if code != 0 || !strings.HasPrefix(out, miserelease.Version) || strings.Contains(stderr, "Migrating") || strings.Contains(stderr, "Downloading") {
		t.Fatalf("offline reuse: %d %q %s", code, out, stderr)
	}
	// Pin the current shell path before hiding system mise; this command checks
	// real mise directory settings and update flags through its exec environment.
	shell := "/bin/sh"
	if runtime.GOOS != "windows" {
		args := []string{"exec", "web", "--", shell, "-c", `printf '%s|%s|%s|%s|%s|%s' "$MISE_DATA_DIR" "$MISE_CONFIG_DIR" "$MISE_STATE_DIR" "$MISE_CACHE_DIR" "$MISE_AUTO_UPDATE" "$MISE_DISABLE_UPDATE_WARNING"`}
		out, stderr, code = runBinaryIn(t, root, args...)
		want := strings.Join([]string{filepath.Join(root, ".local/share/one/mise"), filepath.Join(root, ".config/one/mise"), filepath.Join(root, ".local/state/one/mise"), filepath.Join(root, ".cache/one/mise"), "false", "true"}, "|")
		if code != 0 || out != want {
			t.Fatalf("managed environment: %d %q want %q %s", code, out, want, stderr)
		}
	}
}

func TestE2E_MiseCreateAddAndRefreshWithoutNewFlags(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	t.Setenv("ONE_RUNTIME", "")
	root := bootstrapWorkspace(t, tmp, "original-commands")
	for _, p := range []struct{ template, name string }{{"react-spa", "web"}, {"go-api", "api"}} {
		out, stderr, code := runBinaryIn(t, root, "add", p.template, "--name", p.name, "-y", "-o", "json")
		if code != 0 {
			t.Fatalf("add %s: %d %s %s", p.name, code, out, stderr)
		}
	}
	for _, rel := range []string{"mise.toml"} {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil || !strings.Contains(string(raw), "# one:managed-v1") {
			t.Fatalf("configuration %s: %v %s", rel, err, raw)
		}
	}
	for _, rel := range []string{"apps/web/mise.toml", "services/api/mise.toml"} {
		if fileExists(t, filepath.Join(root, rel)) {
			t.Fatalf("unexpected project configuration: %s", rel)
		}
	}
	out, stderr, code := runBinaryIn(t, root, "init", "mise", "--dry-run", "-o", "json")
	if code != 0 || !strings.Contains(out, `"changes": []`) {
		t.Fatalf("refresh: %d %s %s", code, out, stderr)
	}
	out, stderr, code = runBinaryIn(t, root, "exec", "web", "--dry-run", "--", "node", "--version")
	if code != 0 || !strings.Contains(out, `"runtime": "mise"`) {
		t.Fatalf("automatic runtime: %d %s %s", code, out, stderr)
	}
	// A conflicting aggregate task edit must fail before rendering a new project.
	path := filepath.Join(root, "mise.toml")
	raw, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(raw), "//:web:build", "//:custom:build", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, _ := os.ReadFile(filepath.Join(root, "one.manifest.toml"))
	_, _, code = runBinaryIn(t, root, "add", "react-spa", "--name", "another", "-y")
	if code == 0 {
		t.Fatal("add succeeded with conflicting configuration")
	}
	after, _ := os.ReadFile(filepath.Join(root, "one.manifest.toml"))
	if string(after) != string(manifest) {
		t.Fatal("conflict changed manifest")
	}
	if _, err := os.Stat(filepath.Join(root, "apps/another")); !os.IsNotExist(err) {
		t.Fatal("conflict created a project")
	}
}

func TestE2E_MiseNativeTasksDoNotRequireOneContext(t *testing.T) {
	mise := os.Getenv("ONE_TEST_MISE_BINARY")
	if mise == "" {
		t.Skip("set ONE_TEST_MISE_BINARY for real task integration")
	}
	tmp := t.TempDir()
	isolateHome(t, tmp)
	t.Setenv("ONE_RUNTIME", "")
	root := bootstrapWorkspace(t, tmp, "generated-tasks")
	useRealMise(t, root, mise)
	if out, stderr, code := runBinaryIn(t, root, "add", "react-spa", "--name", "web", "-y"); code != 0 {
		t.Fatalf("add: %d %s %s", code, out, stderr)
	}
	// Edit the generated configuration in place, preserving monorepo fields.
	configPath := filepath.Join(root, "mise.toml")
	configRaw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configText := strings.ReplaceAll(string(configRaw), "node = '24.15.0'", "node = 'system'")
	configText = strings.ReplaceAll(configText, "pnpm = '12.3.4'", "pnpm = 'system'")
	configText += "\n[env]\nTASK_DEFAULT = 'mise'\n"
	if err := os.WriteFile(configPath, []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps/web/.env"), []byte("TASK_DEFAULT=one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(t.TempDir(), "one with spaces")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(binDir, "one")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	raw, err := os.ReadFile(binaryPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, raw, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, command := range []string{"echo live-first", "echo live-second"} {
		overrideDevCommand(t, root, "web", command)
		cmd := exec.Command(mise, "run", "//:web:dev")
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), strings.TrimPrefix(command, "echo ")) {
			t.Fatalf("generated task: %v %s", err, out)
		}
	}
}
