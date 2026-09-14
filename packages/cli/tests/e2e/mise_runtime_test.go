package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func runtimeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	isolateHome(t, root)
	t.Setenv("ONE_RUNTIME", "")
	files := map[string]string{
		"one.manifest.json":              `{"version":1,"workspace":{"id":"runtime-test","name":"runtime-test"},"projects":[{"name":"web","relativeDir":"apps/web","toolchain":"node","templateId":"react-spa","domains":{"dev":{"command":"node dev.cjs"}}},{"name":"api","relativeDir":"services/api","toolchain":"go","templateId":"go-api","domains":{"dev":{"command":"node dev.cjs"}}}]}`,
		".mise/conf.d/one.toml":          "[env]\nONE_MISE_TEST_VALUE = 'root'\nONE_MISE_PARENT = 'root-only'\n",
		"apps/web/.mise/conf.d/one.toml": "[env]\nONE_MISE_TEST_VALUE = 'project'\nONE_MISE_ONLY = 'from-mise'\n",
		"apps/web/.env":                  "ONE_MISE_TEST_VALUE=web-secret\nWEB_ONLY=web-only\n",
		"services/api/.env":              "ONE_MISE_TEST_VALUE=api-secret\nAPI_ONLY=api-only\n",
		"apps/web/package.json":          `{"scripts":{"dev":"node dev.cjs"}}`,
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
	script := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo '2026.9.7 linux-x64'; exit 0; fi\n[ \"$1\" = exec ] && [ \"$2\" = -- ] || exit 91\nshift 2\nexport ONE_MISE_TEST_VALUE=from-mise\nexport ONE_MISE_ONLY=from-mise\nexec \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ONE_MISE_BINARY", filepath.Join(dir, "mise"))
}

func TestE2E_MiseRuntimeOriginalRunSyntaxAndExitCode(t *testing.T) {
	root := runtimeFixture(t)
	installFakeMise(t)
	args := append([]string{"run", "web", "--"}, environmentEchoCommand("ONE_MISE_TEST_VALUE")...)
	out, errOut, code := runBinaryIn(t, root, args...)
	if code != 0 || strings.TrimSpace(out) != "web-secret" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out, errOut)
	}
	args = append([]string{"run", "web", "--"}, exitWithCodeCommand(42)...)
	_, _, code = runBinaryIn(t, root, args...)
	if code != 42 {
		t.Fatalf("exit code = %d, want 42", code)
	}
	// Literal argv must survive mise and the terminal One execution leaf.
	args = []string{"run", "web", "--", "sh", "-c", `printf '<%s>\n' "$@"`, "probe", "a b", "", `$(not-a-command)`, "中文", `a'b"c`}
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
	if err := os.WriteFile(filepath.Join(root, "apps/web/.mise/conf.d/one.toml"), []byte(`[env]
ONE_MISE_TEST_VALUE = '{{ exec(command="echo from-mise") }}'
`), 0o644); err != nil {
		t.Fatal(err)
	}
	args := append([]string{"run", "web", "--"}, environmentEchoCommand("ONE_MISE_TEST_VALUE")...)
	if _, _, code := runBinaryIn(t, root, args...); code == 0 {
		t.Fatal("untrusted configuration ran")
	}
	for _, rel := range []string{".mise/conf.d/one.toml", "apps/web/.mise/conf.d/one.toml"} {
		out, stderr, code := runBinaryIn(t, root, "mise", "trust", filepath.Join(root, rel))
		if code != 0 {
			t.Fatalf("trust: %d %q %s", code, out, stderr)
		}
	}
	out, stderr, code := runBinaryIn(t, root, args...)
	if code != 0 || strings.TrimSpace(out) != "web-secret" {
		t.Fatalf("trusted run: %d %q %s", code, out, stderr)
	}
}

func TestE2E_MiseMissingReportsErrorAndLegacyStillRuns(t *testing.T) {
	root := runtimeFixture(t)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("ONE_MISE_BINARY", filepath.Join(t.TempDir(), "missing-mise"))
	out, stderr, code := runBinaryIn(t, root, "run", "web", "--", "unavailable-command")
	if code == 0 || !strings.Contains(out+stderr, "MISE_NOT_FOUND") {
		t.Fatalf("missing mise: %d %s %s", code, out, stderr)
	}
	// Removing only the One-generated root configuration models an old workspace.
	if err := os.Remove(filepath.Join(root, ".mise/conf.d/one.toml")); err != nil {
		t.Fatal(err)
	}
	out, stderr, code = runBinaryIn(t, root, "run", "web", "--dry-run", "--", "unavailable-command")
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
	out, errOut, code := runBinaryIn(t, root, "run", "web", "--dry-run", "--", "does-not-exist")
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
	if _, err := os.Stat(filepath.Join(root, ".cache/one/runtimes/mise")); !os.IsNotExist(err) {
		t.Fatal("dry-run prepared the bundled runtime")
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
		{"web", "ONE_MISE_TEST_VALUE", "web-secret"}, {"web", "ONE_MISE_ONLY", "from-mise"},
		{"web", "ONE_MISE_PARENT", "root-only"}, {"api", "ONE_MISE_TEST_VALUE", "api-secret"},
	} {
		args := append([]string{"run", pair.project, "--"}, environmentEchoCommand(pair.key)...)
		out, stderr, code := runBinaryIn(t, root, args...)
		if code != 0 || strings.TrimSpace(out) != pair.want {
			t.Fatalf("%+v: code=%d stdout=%q stderr=%q", pair, code, out, stderr)
		}
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node required for environment isolation assertion")
	}
	out, stderr, code := runBinaryIn(t, root, "run", "api", "--", "node", "-p", "process.env.WEB_ONLY || 'isolated'")
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

// No runtime network or preinstalled mise is needed, including on first use.
func TestE2E_MiseBundledFirstRunWithoutMiseOnPath(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Linux amd64 offline fixture")
	}
	root := runtimeFixture(t)
	isolateMiseState(t, root)
	t.Setenv("PATH", "/usr/bin:/bin")
	if _, err := exec.LookPath("mise"); err == nil {
		t.Skip("fixture requires mise absent from system directories")
	}
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		t.Setenv(key, "http://127.0.0.1:1")
	}
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
	t.Setenv("MISE_AUTO_UPDATE", "true")
	t.Setenv("MISE_DISABLE_UPDATE_WARNING", "false")
	out, stderr, code := runBinaryIn(t, root, "run", "web", "--", "/bin/sh", "-c", `printf '%s' "$ONE_MISE_TEST_VALUE"`)
	if code != 0 || out != "web-secret" || !strings.Contains(stderr, "Preparing bundled mise") {
		t.Fatalf("first run: %d %q %s", code, out, stderr)
	}
	// The user's auto-update preference must not mutate One's pinned runtime.
	out, stderr, code = runBinaryIn(t, root, "run", "web", "--", "/bin/sh", "-c", `printf '%s %s' "$MISE_AUTO_UPDATE" "$MISE_DISABLE_UPDATE_WARNING"`)
	if code != 0 || out != "false true" {
		t.Fatalf("bundled update settings: %d %q %s", code, out, stderr)
	}
	path := filepath.Join(root, ".cache/one/runtimes/mise/2026.9.7/linux-x64-musl/mise")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	out, stderr, code = runBinaryIn(t, root, "run", "web", "--", "mise", "--version")
	if code != 0 || !strings.HasPrefix(out, "2026.9.7") || strings.Contains(stderr, "Preparing bundled mise") {
		t.Fatalf("cached run: %d %q %s", code, out, stderr)
	}
	out, stderr, code = runBinaryIn(t, root, "mise", "--version")
	if code != 0 || !strings.HasPrefix(out, "2026.9.7") {
		t.Fatalf("offline mise command: %d %q %s", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "MISE_CACHE_DIR", "latest-version")); !os.IsNotExist(err) {
		t.Fatal("bundled runtime performed a version update check")
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
	for _, rel := range []string{".mise/conf.d/one.toml", "apps/web/.mise/conf.d/one.toml", "services/api/.mise/conf.d/one.toml"} {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil || !strings.Contains(string(raw), "Managed by One CLI") {
			t.Fatalf("configuration %s: %v %s", rel, err, raw)
		}
	}
	out, stderr, code := runBinaryIn(t, root, "configure", "mise", "--dry-run", "-o", "json")
	if code != 0 || !strings.Contains(out, `"changes": []`) {
		t.Fatalf("refresh: %d %s %s", code, out, stderr)
	}
	out, stderr, code = runBinaryIn(t, root, "run", "web", "--dry-run", "--", "node", "--version")
	if code != 0 || !strings.Contains(out, `"runtime": "mise"`) {
		t.Fatalf("automatic runtime: %d %s %s", code, out, stderr)
	}
	// A modified generated fragment must fail before rendering a new project.
	path := filepath.Join(root, ".mise/conf.d/one.toml")
	raw, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(raw, []byte("\n# edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, _ := os.ReadFile(filepath.Join(root, "one.manifest.json"))
	_, _, code = runBinaryIn(t, root, "add", "react-spa", "--name", "another", "-y")
	if code == 0 {
		t.Fatal("add succeeded with conflicting configuration")
	}
	after, _ := os.ReadFile(filepath.Join(root, "one.manifest.json"))
	if string(after) != string(manifest) {
		t.Fatal("conflict changed manifest")
	}
	if _, err := os.Stat(filepath.Join(root, "apps/another")); !os.IsNotExist(err) {
		t.Fatal("conflict created a project")
	}
}

func TestE2E_MiseRealGeneratedTasksUseLiveCommands(t *testing.T) {
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
	// User overrides have higher priority than generated defaults. This test
	// exercises real mise without downloading Node or package managers.
	if err := os.WriteFile(filepath.Join(root, "mise.toml"), []byte("[tools]\nnode = 'system'\npnpm = 'system'\n[env]\nTASK_DEFAULT = 'mise'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps/web/.env"), []byte("TASK_DEFAULT=one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "one with spaces")
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
	t.Setenv("ONE_BINARY_PATH", bin)
	for _, command := range []string{"echo live-first", "echo live-second"} {
		overrideDevCommand(t, root, "web", command)
		cmd := exec.Command(mise, "run", "//apps/web:one:dev")
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), strings.TrimPrefix(command, "echo ")) {
			t.Fatalf("generated task: %v %s", err, out)
		}
	}
}
