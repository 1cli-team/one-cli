package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshot_E2E_CIOptionalLifecycle(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "demo")

	for _, args := range [][]string{
		{"add", "react-spa", "--name", "web", "--yes", "-o", "json"},
		{"add", "go-api", "--name", "api", "--yes", "-o", "json"},
	} {
		if stdout, stderr, code := runBinaryIn(t, ws, args...); code != 0 {
			t.Fatalf("%v failed: exit=%d stdout=%s stderr=%s", args, code, stdout, stderr)
		}
	}

	webWorkflow := filepath.Join(ws, ".github", "workflows", "ci-apps-web.yml")
	apiWorkflow := filepath.Join(ws, ".github", "workflows", "ci-services-api.yml")
	if fileExists(t, webWorkflow) || fileExists(t, apiWorkflow) {
		t.Fatal("create/add must not enable CI before one ci enable")
	}

	stdout, stderr, code := runBinaryIn(t, ws, "ci", "-o", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("ci status failed: exit=%d stderr=%q", code, stderr)
	}
	status := mustParseJSON(t, stdout)
	if status["schema"] != "one-cli/ci-status/v1" || status["configured"] != false {
		t.Fatalf("unexpected initial status: %v", status)
	}
	if status["next_command"] != "one ci enable web" {
		t.Fatalf("unexpected initial next command: %v", status["next_command"])
	}
	projects := status["projects"].([]any)
	if len(projects) != 2 || projects[0].(map[string]any)["enabled"] != false {
		t.Fatalf("unexpected initial projects: %v", projects)
	}

	manifestBefore, err := os.ReadFile(filepath.Join(ws, "one.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = runBinaryIn(t, ws, "ci", "enable", "web", "-o", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("ci enable failed: exit=%d stderr=%q", code, stderr)
	}
	enabled := mustParseJSON(t, stdout)
	if enabled["schema"] != "one-cli/ci-enable/v1" || enabled["provider"] != "ci/github-actions" {
		t.Fatalf("unexpected enable result: %v", enabled)
	}
	enabledProjects := enabled["projects"].([]any)
	if len(enabledProjects) != 1 {
		t.Fatalf("enable should target one project: %v", enabledProjects)
	}
	web := enabledProjects[0].(map[string]any)
	if web["name"] != "web" || web["status"] != "created" || web["workflow_path"] != ".github/workflows/ci-apps-web.yml" {
		t.Fatalf("unexpected enabled project: %v", web)
	}
	if !fileExists(t, webWorkflow) || fileExists(t, apiWorkflow) {
		t.Fatalf("enable web wrote the wrong workflow set")
	}
	manifestAfter, err := os.ReadFile(filepath.Join(ws, "one.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(manifestAfter) != string(manifestBefore) {
		t.Fatal("one ci enable must not mutate one.manifest.json")
	}

	if err := os.WriteFile(webWorkflow, []byte("corrupted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = runBinaryIn(t, ws, "ci", "sync", "-o", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("ci sync failed: exit=%d stderr=%q", code, stderr)
	}
	synced := mustParseJSON(t, stdout)
	if synced["schema"] != "one-cli/ci-sync/v1" {
		t.Fatalf("unexpected sync result: %v", synced)
	}
	syncedProjects := synced["projects"].([]any)
	if len(syncedProjects) != 1 || syncedProjects[0].(map[string]any)["name"] != "web" {
		t.Fatalf("sync without selector should refresh only enabled projects: %v", syncedProjects)
	}
	raw, err := os.ReadFile(webWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "corrupted\n" || !strings.Contains(string(raw), "name:") {
		t.Fatalf("sync did not regenerate workflow:\n%s", raw)
	}

	stdout, stderr, code = runBinaryIn(t, ws, "ci", "enable", "-o", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("ci enable all failed: exit=%d stderr=%q", code, stderr)
	}
	allEnabled := mustParseJSON(t, stdout)["projects"].([]any)
	if len(allEnabled) != 2 || !fileExists(t, apiWorkflow) {
		t.Fatalf("enable without selector should cover every project: %v", allEnabled)
	}

	_, stderr, code = runBinaryIn(t, ws, "ci", "disable", "web", "-o", "json")
	if code == 0 {
		t.Fatal("non-interactive disable without --yes should fail")
	}
	disableError := mustParseJSON(t, firstJSONLine(stderr))
	if disableError["error"].(map[string]any)["code"] != "CI_DISABLE_CONFIRMATION_REQUIRED" {
		t.Fatalf("unexpected disable confirmation error: %v", disableError)
	}
	if !fileExists(t, webWorkflow) {
		t.Fatal("unconfirmed non-interactive disable must preserve the workflow")
	}

	stdout, stderr, code = runBinaryIn(t, ws, "ci", "disable", "web", "--yes", "-o", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("ci disable failed: exit=%d stderr=%q", code, stderr)
	}
	disabled := mustParseJSON(t, stdout)
	if disabled["schema"] != "one-cli/ci-disable/v1" {
		t.Fatalf("unexpected disable result: %v", disabled)
	}
	if fileExists(t, webWorkflow) || !fileExists(t, apiWorkflow) {
		t.Fatal("disable web must remove only the selected workflow")
	}

	stdout, stderr, code = runBinaryIn(t, ws, "ci", "disable", "--yes", "-o", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("ci disable all failed: exit=%d stderr=%q", code, stderr)
	}
	if fileExists(t, apiWorkflow) {
		t.Fatal("disable without selector should remove remaining generated workflows")
	}

	_, stderr, code = runBinaryIn(t, ws, "ci", "sync", "web", "-o", "json")
	if code == 0 {
		t.Fatal("syncing a disabled project should fail")
	}
	errEnvelope := mustParseJSON(t, firstJSONLine(stderr))
	if errEnvelope["error"].(map[string]any)["code"] != "CI_NOT_ENABLED" {
		t.Fatalf("unexpected disabled-sync error: %v", errEnvelope)
	}
}

func TestSnapshot_E2E_CIProviderValidationAndLocalizedText(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "demo")
	if _, stderr, code := runBinaryIn(t, ws, "add", "react-spa", "--name", "web", "--yes", "-o", "json"); code != 0 {
		t.Fatalf("add failed: exit=%d stderr=%s", code, stderr)
	}

	_, stderr, code := runBinaryIn(t, ws, "ci", "enable", "web", "--provider", "not-real", "-o", "json")
	if code == 0 {
		t.Fatal("unknown provider should fail")
	}
	envelope := mustParseJSON(t, firstJSONLine(stderr))
	errBody := envelope["error"].(map[string]any)
	if errBody["code"] != "CI_PROVIDER_UNKNOWN" {
		t.Fatalf("unexpected provider error: %v", envelope)
	}
	ctx := errBody["context"].(map[string]any)
	if ctx["provider"] != "not-real" {
		t.Fatalf("provider error context lost requested id: %v", ctx)
	}

	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	stdout, stderr, code := runBinaryIn(t, ws, "ci", "-o", "text")
	if code != 0 || stderr != "" {
		t.Fatalf("localized ci status failed: exit=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{"持续集成：未配置", "web", "未启用", "下一步：one ci enable web"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("localized status missing %q:\n%s", want, stdout)
		}
	}
}
