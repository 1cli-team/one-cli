package cli_test

// E2E coverage of `one container` — the post capability-interface
// refactor replacement for `one docker`.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func TestSnapshot_E2E_Container_HelpListsSubcommands(t *testing.T) {
	stdout, _, code := runBinary(t, "container", "--help")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	for _, sub := range []string{"info", "build", "push"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("`one container --help` does not mention subcommand %q", sub)
		}
	}
	// `one container --help`'s Long text references the cross-domain
	// `one configure add container/<kind>` tree where registry creds live.
	if !strings.Contains(stdout, "configure add container/<kind>") {
		t.Errorf("`one container --help` should reference `one configure add container/<kind>`")
	}
}

func TestSnapshot_E2E_Container_OutsideWorkspaceReturnsStructuredError(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	_, stderr, code := runBinaryIn(t, tmp, "container", "info", "-o", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit outside a workspace")
	}
	envelope := firstJSONLine(stderr)
	if envelope == "" {
		t.Fatalf("expected JSON error envelope on stderr, got: %q", stderr)
	}
	got := mustParseJSON(t, envelope)
	errMap := got["error"].(map[string]any)
	if errMap["code"] != "NOT_ONE_PROJECT" {
		t.Errorf("error.code: want NOT_ONE_PROJECT, got %v", errMap["code"])
	}
}

func TestSnapshot_E2E_Container_PluginNotEnabledReturnsStructuredError(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "container", "info", "-o", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit when plugin not enabled")
	}
	envelope := firstJSONLine(stderr)
	if envelope == "" {
		t.Fatalf("expected JSON error envelope on stderr, got: %q", stderr)
	}
	got := mustParseJSON(t, envelope)
	errMap := got["error"].(map[string]any)
	if errMap["code"] != "BACKEND_NOT_ENABLED" {
		t.Errorf("error.code: want BACKEND_NOT_ENABLED, got %v", errMap["code"])
	}
}

func TestSnapshot_E2E_Container_InfoWhenEnabled(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	if _, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json"); code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}

	stdout, stderr, code := runBinaryIn(t, ws, "container", "info", "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0 when container backend enabled, got %d\n  stderr: %s", code, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["schema"] != "one-cli/container-info/v2" {
		t.Errorf("schema: want one-cli/container-info/v2, got %v", got["schema"])
	}
	if got["container_backend"] != "docker" {
		t.Errorf("container_backend: want docker, got %v", got["container_backend"])
	}
}

func TestSnapshot_E2E_Container_PerSubprojectOverrideAfterAdd(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}

	stdout, stderr, code := runBinaryIn(t, ws, "container", "info", "-o", "json")
	if code != 0 {
		t.Fatalf("container info should consume per-subproject override, got exit %d\n  stderr: %s", code, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["schema"] != "one-cli/container-info/v2" {
		t.Errorf("schema: want one-cli/container-info/v2, got %v", got["schema"])
	}
	if got["container_backend"] != "docker" {
		t.Errorf("container_backend: want docker, got %v", got["container_backend"])
	}
	subs, _ := got["projects"].([]any)
	if len(subs) != 1 {
		t.Fatalf("subprojects: want 1 container-enabled subproject, got %d: %v", len(subs), got)
	}
	sub, _ := subs[0].(map[string]any)
	if sub["name"] != "api" {
		t.Errorf("subproject name: want api, got %v", sub["name"])
	}
	if sub["backend"] != "docker" {
		t.Errorf("subproject backend: want docker, got %v", sub["backend"])
	}
	if sub["has_artifact"] != true {
		t.Errorf("has_artifact: want true, got %v", sub["has_artifact"])
	}

	_, stderr, code = runBinary(t, "configure", "add", "container/docker", "--profile", "ghcr",
		"--registry", "ghcr.io",
		"--username", "u",
		"--password", "p",
		"--use",
		"-o", "json")
	if code != 0 {
		t.Fatalf("profile container/docker add failed: exit %d\n  stderr: %s", code, stderr)
	}

	stdout, stderr, code = runBinaryIn(t, ws, "container", "build", "-p", "api", "--profile", "ghcr", "--build-version", "v0.1.0", "--dry-run")
	if code != 0 {
		t.Fatalf("container build should consume per-subproject override, got exit %d\n  stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "docker build -t ghcr.io/u/api:v0.1.0") {
		t.Errorf("dry-run output should include docker build for api, got: %s", stdout)
	}
	if !strings.Contains(stdout, "services/api") {
		t.Errorf("dry-run output should target services/api, got: %s", stdout)
	}
}

func TestSnapshot_E2E_Container_BuildUsesSubprojectBuildVersion(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}

	stdout, stderr, code := runBinaryIn(t, ws, "container", "build", "api", "--dry-run")
	if code != 0 {
		t.Fatalf("container build should use buildVersion, got exit %d\n  stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "docker build -t api:v0.1.0") {
		t.Errorf("dry-run output should include buildVersion tag, got: %s", stdout)
	}
}

func TestSnapshot_E2E_Container_BuildIgnoresMachineDefaultProfileByDefault(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}

	_, stderr, code = runBinary(t, "configure", "add", "container/docker", "--profile", "ghcr",
		"--registry", "ghcr.io",
		"--username", "u",
		"--password", "p",
		"--use",
		"-o", "json")
	if code != 0 {
		t.Fatalf("profile container/docker add failed: exit %d\n  stderr: %s", code, stderr)
	}

	stdout, stderr, code := runBinaryIn(t, ws, "container", "build", "api", "--dry-run")
	if code != 0 {
		t.Fatalf("container build should default to local tag, got exit %d\n  stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "docker build -t api:v0.1.0") {
		t.Errorf("bare build should not use default ghcr profile, got: %s", stdout)
	}

	stdout, stderr, code = runBinaryIn(t, ws, "container", "build", "api", "--profile", "ghcr", "--dry-run")
	if code != 0 {
		t.Fatalf("container build --profile should use registry tag, got exit %d\n  stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "docker build -t ghcr.io/u/api:v0.1.0") {
		t.Errorf("profile build should include registry tag, got: %s", stdout)
	}
}

func TestSnapshot_E2E_Container_BuildLoginFailureGuidesProfileFix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake docker shell script is unix-only")
	}
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}
	_, stderr, code = runBinary(t, "configure", "add", "container/docker", "--profile", "ghcr",
		"--registry", "ghcr.io",
		"--username", "u",
		"--password", "bad-token",
		"--use",
		"-o", "json")
	if code != 0 {
		t.Fatalf("profile container/docker add failed: exit %d\n  stderr: %s", code, stderr)
	}

	fakeBin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	dockerPath := filepath.Join(fakeBin, "docker")
	script := "#!/bin/sh\nif [ \"$1\" = \"login\" ]; then echo 'denied: denied' >&2; exit 1; fi\necho docker \"$@\"\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, stderr, code = runBinaryIn(t, ws, "container", "build", "api", "--profile", "ghcr", "-o", "json")
	if code == 0 {
		t.Fatalf("expected docker login failure")
	}
	got := mustParseJSON(t, firstJSONLine(stderr))
	errMap := got["error"].(map[string]any)
	if errMap["code"] != "BACKEND_INVOKE_FAILED" {
		t.Fatalf("code: want BACKEND_INVOKE_FAILED, got %v", errMap["code"])
	}
	msg := errMap["message"].(string)
	for _, want := range []string{"docker login ghcr.io failed", "profile \"ghcr\"", "denied: denied"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message should contain %q, got %q", want, msg)
		}
	}
	ctx := errMap["context"].(map[string]any)
	if ctx["profile"] != "ghcr" || ctx["profile_source"] != "flag" {
		t.Fatalf("context should identify profile/source, got %v", ctx)
	}
	remediation := errMap["remediation"].([]any)
	if len(remediation) < 3 {
		t.Fatalf("expected actionable remediation, got %v", remediation)
	}
	commands := []string{}
	for _, item := range remediation {
		step := item.(map[string]any)
		if command, _ := step["command"].(string); command != "" {
			commands = append(commands, command)
		}
	}
	joined := strings.Join(commands, "\n")
	for _, want := range []string{
		"one configure current container/docker",
		"one configure list container/docker",
		"one configure add container/docker --profile <name>",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("remediation missing %q, got %v", want, commands)
		}
	}
}

func TestSnapshot_E2E_Container_BuildDryRunUsesGate(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	stdout, stderr, code := runBinaryIn(t, tmp,
		"container", "build", "--dry-run", "-o", "json",
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit outside workspace; stdout: %q", stdout)
	}
	if strings.Contains(stdout, "docker build") {
		t.Errorf("--dry-run should NOT print docker command when gate fails: %q", stdout)
	}
	envelope := firstJSONLine(stderr)
	got := mustParseJSON(t, envelope)
	if got["error"].(map[string]any)["code"] != "NOT_ONE_PROJECT" {
		t.Errorf("expected NOT_ONE_PROJECT, got %v", got)
	}
}

func TestSnapshot_E2E_Container_BuildWithoutRegistryProfileUsesLocalTag(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}

	stdout, stderr, code := runBinaryIn(t, ws, "container", "build", "-p", "api", "--dry-run", "-o", "text")
	if code != 0 {
		t.Fatalf("expected local dry-run build without profile, got exit %d\n  stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "docker build -t api:v0.1.0") {
		t.Errorf("expected local image tag, got %s", stdout)
	}
}

func TestSnapshot_E2E_Container_PushDryRunUsesContainerProfile(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}

	// Namespace lives per-subproject post v3 (commit 0c63d7d moved deploy /
	// container targets out of the machine profile). `one add -y` skips the
	// interactive namespace prompt, so write it directly here.
	if err := workspace.SetProjectContainerNamespace(ws, "api", "acme"); err != nil {
		t.Fatalf("set container namespace: %v", err)
	}

	_, stderr, code = runBinary(t, "configure", "add", "container/docker", "--profile", "ghcr",
		"--registry", "ghcr.io",
		"--username", "u",
		"--password", "p",
		"--use",
		"-o", "json")
	if code != 0 {
		t.Fatalf("profile container/docker add failed: exit %d\n  stderr: %s", code, stderr)
	}

	stdout, stderr, code := runBinaryIn(t, ws, "container", "push", "-p", "api", "--profile", "ghcr", "--build-version", "v0.1.0", "--dry-run")
	if code != 0 {
		t.Fatalf("container push --dry-run failed: exit %d\n  stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "docker tag api:v0.1.0 ghcr.io/acme/api:v0.1.0") {
		t.Errorf("dry-run output should show local-to-registry retag, got: %s", stdout)
	}
	if !strings.Contains(stdout, "docker push ghcr.io/acme/api:v0.1.0") {
		t.Errorf("dry-run output should include docker push, got: %s", stdout)
	}
}

func TestSnapshot_E2E_Container_PushRetagsLocalBuild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake docker shell script is unix-only")
	}
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}
	if err := workspace.SetProjectContainerImage(ws, "api", "api:v0.1.0"); err != nil {
		t.Fatalf("set local container image: %v", err)
	}

	_, stderr, code = runBinary(t, "configure", "add", "container/docker", "--profile", "ghcr",
		"--registry", "ghcr.io",
		"--username", "u",
		"--password", "p",
		"--use",
		"-o", "json")
	if code != 0 {
		t.Fatalf("profile container/docker add failed: exit %d\n  stderr: %s", code, stderr)
	}

	fakeBin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	dockerPath := filepath.Join(fakeBin, "docker")
	script := `#!/bin/sh
if [ "$1" = "image" ] && [ "$2" = "inspect" ]; then
  if [ "$3" = "api:v0.1.0" ]; then exit 0; fi
  exit 1
fi
if [ "$1" = "login" ]; then exit 0; fi
if [ "$1" = "tag" ]; then echo "docker tag $2 $3"; exit 0; fi
if [ "$1" = "push" ]; then echo "docker push $2"; exit 0; fi
echo "unexpected docker $@" >&2
exit 1
`
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	stdout, stderr, code := runBinaryIn(t, ws, "container", "push", "api", "-o", "text")
	if code != 0 {
		t.Fatalf("container push should retag local image, got exit %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "docker tag api:v0.1.0 ghcr.io/u/api:v0.1.0") {
		t.Fatalf("push should tag local image for registry, got stdout: %s", stdout)
	}
	if !strings.Contains(stdout, "docker push ghcr.io/u/api:v0.1.0") {
		t.Fatalf("push should push registry image, got stdout: %s", stdout)
	}
}

func TestSnapshot_E2E_Container_PushRequiresRegistryProfile(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}

	_, stderr, code = runBinaryIn(t, ws, "container", "push", "-p", "api", "--dry-run", "-o", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit when image is missing")
	}
	envelope := firstJSONLine(stderr)
	got := mustParseJSON(t, envelope)
	errMap := got["error"].(map[string]any)
	if errMap["code"] != "REGISTRY_CREDENTIAL_MISSING" {
		t.Errorf("expected REGISTRY_CREDENTIAL_MISSING, got %v", got)
	}
	if !strings.Contains(errMap["message"].(string), "one configure add container/docker") {
		t.Errorf("expected user-facing registry message, got %v", errMap["message"])
	}
	remediation := errMap["remediation"].([]any)
	if len(remediation) != 1 {
		t.Fatalf("expected 1 remediation step, got %d: %v", len(remediation), remediation)
	}
	first := remediation[0].(map[string]any)
	if first["action"] != "setup-registry" || first["command"] != "one configure add container/docker --profile <name> --use" {
		t.Errorf("expected setup-registry remediation for api, got %v", first)
	}
}
