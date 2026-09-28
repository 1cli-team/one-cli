package cli_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSnapshot_E2E_CreateDailyText(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	target := filepath.Join(tmp, "demo")

	stdout, stderr, code := runBinaryIn(t, tmp, "create", target, "--yes", "-o", "text")
	if code != 0 {
		t.Fatalf("create text flow failed: exit=%d stderr=%q", code, stderr)
	}
	want := "✓ 工作区已创建：demo\n  位置：~/demo\n  环境变量来源：本地 .env 文件\n  本地开发：one dev\n\n下一步：\n  cd demo\n  one add\n"
	if stdout != want {
		t.Fatalf("unexpected create success text:\n--- want\n%s--- got\n%s", want, stdout)
	}
	for _, internal := range []string{"profile", "backend", "domain", "manifest selector"} {
		if strings.Contains(strings.ToLower(stdout), internal) {
			t.Errorf("daily create output exposed internal term %q: %s", internal, stdout)
		}
	}

	_, errorText, code := runBinaryIn(t, tmp, "create", "--yes", "-o", "text")
	if code == 0 {
		t.Fatal("non-interactive create without a directory should fail")
	}
	for _, want := range []string{"✗ 非交互模式下必须提供 [dir] 位置参数", "错误代码：PROJECT_NAME_REQUIRED", "可尝试：", "one create <workspace-directory>"} {
		if !strings.Contains(errorText, want) {
			t.Errorf("localized error missing %q: %q", want, errorText)
		}
	}
}

func TestSnapshot_E2E_HelpDailyAndCompleteCatalogues(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	daily, stderr, code := runBinaryIn(t, tmp, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("one --help failed: exit=%d stderr=%q", code, stderr)
	}
	for _, command := range []string{"create", "add", "dev", "build", "env", "login", "run", "exec"} {
		if !strings.Contains(daily, "  "+command) {
			t.Errorf("daily help missing %q:\n%s", command, daily)
		}
	}
	for _, command := range []string{"ci", "templates", "serve"} {
		if strings.Contains(daily, "\n  "+command) {
			t.Errorf("daily help should not advertise advanced command %q:\n%s", command, daily)
		}
	}

	all, stderr, code := runBinaryIn(t, tmp, "help", "--all")
	if code != 0 || stderr != "" {
		t.Fatalf("one help --all failed: exit=%d stderr=%q", code, stderr)
	}
	for _, command := range []string{"create", "add", "dev", "build", "env", "login", "run", "exec", "templates", "serve"} {
		if !strings.Contains(all, "  "+command) {
			t.Errorf("complete help missing %q:\n%s", command, all)
		}
	}
	if strings.Contains(all, "\n  skills") || strings.Contains(all, "\n  ci ") {
		t.Errorf("complete help exposes removed commands:\n%s", all)
	}
}

func TestSnapshot_E2E_WorkspaceOverviewAndDeferredDeployment(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "demo")

	stdout, stderr, code := runBinaryIn(t, ws, "-o", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("bare one failed: exit=%d stderr=%q", code, stderr)
	}
	empty := mustParseJSON(t, stdout)
	if empty["schema"] != "one-cli/workspace-summary/v1" || empty["next_command"] != "one add" {
		t.Fatalf("unexpected empty workspace summary: %v", empty)
	}

	if _, stderr, code = runBinaryIn(t, ws, "add", "react-spa", "--name", "web", "--yes", "-o", "json"); code != 0 {
		t.Fatalf("add failed: exit=%d stderr=%s", code, stderr)
	}
	manifestBefore, err := os.ReadFile(filepath.Join(ws, "one.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := readManifest(t, ws)
	projects := m["projects"].([]any)
	domains := projects[0].(map[string]any)["domains"].(map[string]any)
	if _, exists := domains["deploy"]; exists {
		t.Fatalf("ordinary add configured deployment: %v", domains["deploy"])
	}
	if _, exists := domains["container"]; exists {
		t.Fatalf("ordinary add configured image build: %v", domains["container"])
	}

	stdout, stderr, code = runBinaryIn(t, ws, "-o", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("workspace summary after add failed: exit=%d stderr=%q", code, stderr)
	}
	summary := mustParseJSON(t, stdout)
	project := summary["projects"].([]any)[0].(map[string]any)
	if project["deployment_configured"] != nil || summary["next_command"] != "one dev -p web" {
		t.Fatalf("unexpected project summary: %v", summary)
	}

	savedPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	_, stderr, code = runBinaryIn(t, ws, "exec", "-p", "web", "-o", "json", "--", "pnpm", "dev")
	t.Setenv("PATH", savedPath)
	if code == 0 {
		t.Fatal("dev should report that the native package manager is unavailable")
	}
	devErr := mustParseJSON(t, firstJSONLine(stderr))
	if devErr["error"].(map[string]any)["code"] != "RUN_COMMAND_NOT_FOUND" {
		t.Fatalf("unexpected missing-tool error: %v", devErr)
	}

	_, stderr, code = runBinaryIn(t, ws, "deploy", "--dry-run", "-o", "json")
	if code == 0 {
		t.Fatal("removed deploy command should fail")
	}
	deployErr := mustParseJSON(t, firstJSONLine(stderr))
	if !strings.Contains(deployErr["error"].(map[string]any)["message"].(string), "//:deploy") {
		t.Fatalf("unexpected removed-command error: %v", deployErr)
	}
	manifestAfter, err := os.ReadFile(filepath.Join(ws, "one.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(manifestBefore, manifestAfter) {
		t.Fatal("failed first-deploy setup modified the workspace")
	}
}

func TestSnapshot_E2E_EnvSummarySafeSetAndList(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "demo")

	stdout, stderr, code := runBinaryIn(t, ws, "env", "-o", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("env summary failed: exit=%d stderr=%q", code, stderr)
	}
	summary := mustParseJSON(t, stdout)
	if summary["schema"] != "one-cli/env-summary/v1" || summary["source"] != "dotenv" || summary["default_environment"] != "dev" {
		t.Fatalf("unexpected env summary: %v", summary)
	}

	_, stderr, code = runBinaryIn(t, ws, "env", "set", "TEST_KEY", "-o", "json")
	if code == 0 {
		t.Fatal("non-interactive hidden-value form should require an explicit value")
	}
	setErr := mustParseJSON(t, firstJSONLine(stderr))
	if setErr["error"].(map[string]any)["code"] != "ENV_SET_VALUE_REQUIRED" {
		t.Fatalf("unexpected missing-value error: %v", setErr)
	}

	if _, stderr, code = runBinaryIn(t, ws, "env", "set", "TEST_KEY", "super-secret", "--yes", "-o", "json"); code != 0 {
		t.Fatalf("env set failed: exit=%d stderr=%s", code, stderr)
	}
	stdout, stderr, code = runBinaryIn(t, ws, "env", "list", "-o", "text")
	if code != 0 || stderr != "" {
		t.Fatalf("env list failed: exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "TEST_KEY") || strings.Contains(stdout, "super-secret") {
		t.Fatalf("env list must show names only: %q", stdout)
	}
}

func TestSnapshot_E2E_EnvSummaryYAMLKeepsStableProtocolFields(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "demo")

	stdout, stderr, code := runBinaryIn(t, ws, "env", "-o", "yaml")
	if code != 0 || stderr != "" {
		t.Fatalf("env YAML summary failed: exit=%d stderr=%q", code, stderr)
	}
	var summary map[string]any
	if err := yaml.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("decode env YAML output: %v\n%s", err, stdout)
	}
	if summary["schema"] != "one-cli/env-summary/v1" || summary["source"] != "dotenv" || summary["default_environment"] != "dev" {
		t.Fatalf("unexpected env YAML contract: %v", summary)
	}
}

func TestSnapshot_E2E_RemovedConfigureAndBilingualHelp(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	_, _, code := runBinaryIn(t, tmp, "configure", "-o", "json")
	if code == 0 {
		t.Fatal("removed configure command still accepted")
	}

	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	zh, _, code := runBinaryIn(t, tmp, "create", "--help")
	if code != 0 || !strings.Contains(zh, "创建工作区") || !strings.Contains(zh, "自动化与高级选项") {
		t.Fatalf("Chinese help was not selected:\n%s", zh)
	}
	t.Setenv("LC_ALL", "en_US.UTF-8")
	en, _, code := runBinaryIn(t, tmp, "create", "--help")
	if code != 0 || !strings.Contains(en, "Create a workspace") || !strings.Contains(en, "AUTOMATION AND ADVANCED OPTIONS") {
		t.Fatalf("English help was not selected:\n%s", en)
	}
}
