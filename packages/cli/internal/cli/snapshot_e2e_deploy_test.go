package cli_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func TestSnapshot_E2E_DeploySubprojectWebUsesS3DefaultWhenWorkspacePreferredIsK8s(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	if _, stderr, code := runBinaryIn(t, ws, "add", "react-spa", "--name", "web", "-y", "-o", "json"); code != 0 {
		t.Fatalf("add web failed: exit %d\n  stderr: %s", code, stderr)
	}
	if _, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json"); code != 0 {
		t.Fatalf("add api failed: exit %d\n  stderr: %s", code, stderr)
	}
	if err := workspace.SetProjectDeployBucket(ws, "web", "test"); err != nil {
		t.Fatalf("set web bucket: %v", err)
	}
	if err := workspace.SetWorkspaceDeployK8sTarget(ws, "prod", "kustomize/overlays/prod"); err != nil {
		t.Fatalf("set workspace deploy target: %v", err)
	}
	kubeconfig := writeTestKubeconfig(t, tmp, "prod", "prod")
	if _, stderr, code := runBinary(t, "configure", "add", "deploy/kustomize", "--profile", "demo-k8s",
		"--kubeconfig", kubeconfig,
		"--kubeconfig-context", "prod",
		"--use",
		"-o", "json"); code != 0 {
		t.Fatalf("profile deploy/kustomize add failed: exit %d\n  stderr: %s", code, stderr)
	}
	if _, stderr, code := runBinary(t, "configure", "add", "deploy/aws-s3", "--profile", "prod",
		"--endpoint", "http://127.0.0.1:9000",
		"--region", "us-east-1",
		"--access-key-id", "ak",
		"--access-key-secret", "sk",
		"--force-path-style",
		"--use",
		"-o", "json"); code != 0 {
		t.Fatalf("profile deploy/aws-s3 add failed: exit %d\n  stderr: %s", code, stderr)
	}

	stdout, stderr, code := runBinaryIn(t, ws, "deploy", "-p", "web", "--dry-run")
	if code != 0 {
		t.Fatalf("deploy -p web --dry-run failed: exit %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "s3-upload --endpoint http://127.0.0.1:9000 --bucket test") {
		t.Fatalf("dry-run output missing s3 upload plan:\n%s", stdout)
	}
	if strings.Contains(stdout, "kubectl apply") || strings.Contains(stdout, "docker build") {
		t.Fatalf("dry-run should only include web s3 deployment:\n%s", stdout)
	}
}

func TestSnapshot_E2E_DeployK8sRequiresSetup(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}

	_, stderr, code = runBinaryIn(t, ws, "deploy", "--dry-run", "-o", "json")
	if code == 0 {
		t.Fatalf("expected deploy to require k8s setup")
	}
	got := mustParseJSON(t, firstJSONLine(stderr))
	errMap := got["error"].(map[string]any)
	if errMap["code"] != "PROFILE_NONE_CONFIGURED" {
		t.Fatalf("expected PROFILE_NONE_CONFIGURED, got %v", got)
	}
	if !strings.Contains(errMap["message"].(string), "one configure add deploy/kustomize") {
		t.Fatalf("expected deploy/kustomize profile guidance, got %v", errMap["message"])
	}
}

func TestSnapshot_E2E_DeployK8sDryRunUsesSetupTarget(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	installFakeKubectl(t, tmp, "amd64")
	ws := bootstrapWorkspace(t, tmp, "demo")
	kubeconfig := writeTestKubeconfig(t, tmp, "cn-prod", "cn-prod", "us-prod")

	if _, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json"); code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}
	if err := workspace.SetWorkspaceDeployTarget(ws, "prod", "kustomize/overlays/prod"); err != nil {
		t.Fatalf("set workspace deploy target: %v", err)
	}
	if _, stderr, code := runBinary(t, "configure", "add", "deploy/kustomize", "--profile", "k8s",
		"--kubeconfig", kubeconfig,
		"--kubeconfig-context", "us-prod",
		"--use",
		"-o", "json"); code != 0 {
		t.Fatalf("profile deploy/kustomize add failed: exit %d\n  stderr: %s", code, stderr)
	}
	if _, stderr, code := runBinary(t, "configure", "add", "container/docker", "--profile", "ghcr",
		"--registry", "ghcr.io",
		"--username", "u",
		"--password", "p",
		"--use",
		"-o", "json"); code != 0 {
		t.Fatalf("profile container/docker add failed: exit %d\n  stderr: %s", code, stderr)
	}

	stdout, stderr, code := runBinaryIn(t, ws, "deploy", "--build-version", "v0.1.0", "--dry-run")
	if code != 0 {
		t.Fatalf("deploy --dry-run failed: exit %d\n  stderr: %s", code, stderr)
	}
	for _, want := range []string{
		"docker build --platform linux/amd64 -t ghcr.io/u/api:v0.1.0",
		"docker push ghcr.io/u/api:v0.1.0",
		"create namespace prod --dry-run=client -o yaml",
		"apply -f -",
		"kubectl apply -k",
		"kustomize/overlays/prod",
		"--kubeconfig " + kubeconfig,
		"--context us-prod",
		"--namespace prod",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, stdout)
		}
	}
}

func TestSnapshot_E2E_DeployK8sDryRunDefaultsNamespaceToProjectID(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	installFakeKubectl(t, tmp, "amd64")
	ws := bootstrapWorkspace(t, tmp, "demo")
	kubeconfig := writeTestKubeconfig(t, tmp, "cn-prod", "cn-prod")

	if _, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json"); code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}
	if err := workspace.SetWorkspaceDeployTarget(ws, "", "kustomize/overlays/prod"); err != nil {
		t.Fatalf("set workspace deploy target: %v", err)
	}
	m, err := workspace.ReadManifest(ws)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	wantNamespace := workspace.WorkspaceID(m)
	if wantNamespace == "" {
		t.Fatal("workspace project id should be set")
	}
	if _, stderr, code := runBinary(t, "configure", "add", "deploy/kustomize", "--profile", "k8s",
		"--kubeconfig", kubeconfig,
		"--kubeconfig-context", "cn-prod",
		"--use",
		"-o", "json"); code != 0 {
		t.Fatalf("profile deploy/kustomize add failed: exit %d\n  stderr: %s", code, stderr)
	}
	if _, stderr, code := runBinary(t, "configure", "add", "container/docker", "--profile", "ghcr",
		"--registry", "ghcr.io",
		"--username", "u",
		"--password", "p",
		"--use",
		"-o", "json"); code != 0 {
		t.Fatalf("profile container/docker add failed: exit %d\n  stderr: %s", code, stderr)
	}

	stdout, stderr, code := runBinaryIn(t, ws, "deploy", "--build-version", "v0.1.0", "--dry-run")
	if code != 0 {
		t.Fatalf("deploy --dry-run failed: exit %d\n  stderr: %s", code, stderr)
	}
	if want := "--namespace " + wantNamespace; !strings.Contains(stdout, want) {
		t.Fatalf("dry-run output missing %q:\n%s", want, stdout)
	}
	if want := "create namespace " + wantNamespace + " --dry-run=client -o yaml"; !strings.Contains(stdout, want) {
		t.Fatalf("dry-run output missing %q:\n%s", want, stdout)
	}
}

func TestSnapshot_E2E_DeployK8sRequiresContainerProfile(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "demo")
	kubeconfig := writeTestKubeconfig(t, tmp, "cn-prod", "cn-prod")

	if _, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "-y", "-o", "json"); code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}
	if err := workspace.SetWorkspaceDeployTarget(ws, "prod", "kustomize/overlays/prod"); err != nil {
		t.Fatalf("set workspace deploy target: %v", err)
	}
	if _, stderr, code := runBinary(t, "configure", "add", "deploy/kustomize", "--profile", "k8s",
		"--kubeconfig", kubeconfig,
		"--kubeconfig-context", "cn-prod",
		"--use",
		"-o", "json"); code != 0 {
		t.Fatalf("profile deploy/kustomize add failed: exit %d\n  stderr: %s", code, stderr)
	}

	_, stderr, code := runBinaryIn(t, ws, "deploy", "--build-version", "v0.1.0", "--dry-run", "-o", "json")
	if code == 0 {
		t.Fatalf("expected deploy to require container profile")
	}
	got := mustParseJSON(t, firstJSONLine(stderr))
	errMap := got["error"].(map[string]any)
	if errMap["code"] != "REGISTRY_CREDENTIAL_MISSING" {
		t.Fatalf("expected REGISTRY_CREDENTIAL_MISSING, got %v", got)
	}
	if !strings.Contains(errMap["message"].(string), "one configure add container/docker") {
		t.Fatalf("expected setup container guidance, got %v", errMap["message"])
	}
}

func TestSnapshot_E2E_DeployCloudflareUsesProjectLocalWrangler(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "demo")

	if _, stderr, code := runBinaryIn(t, ws, "add", "astro-site", "--name", "web", "--deploy-provider", "cloudflare", "-y", "-o", "json"); code != 0 {
		t.Fatalf("add web failed: exit %d\n  stderr: %s", code, stderr)
	}
	if _, stderr, code := runBinary(t, "configure", "add", "deploy/cloudflare", "--profile", "cf-prod",
		"--token", "tok-cf",
		"--account-id", "acct-cf",
		"--use",
		"-o", "json"); code != 0 {
		t.Fatalf("profile deploy/cloudflare add failed: exit %d\n  stderr: %s", code, stderr)
	}
	projectDir := filepath.Join(ws, "apps", "web")
	logPath := filepath.Join(tmp, "wrangler.log")
	installProjectLocalWrangler(t, projectDir, logPath, "https://web.example.workers.dev")
	emptyPath := filepath.Join(tmp, "empty-path")
	if err := os.MkdirAll(emptyPath, 0o755); err != nil {
		t.Fatalf("mkdir empty path: %v", err)
	}
	installFakePackageManager(t, emptyPath, "pnpm", logPath)
	t.Setenv("PATH", emptyPath)

	stdout, stderr, code := runBinaryIn(t, ws, "deploy", "-p", "web")
	if code != 0 {
		t.Fatalf("deploy cloudflare failed: exit %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "https://web.example.workers.dev") {
		t.Fatalf("deploy output missing fake deployment URL:\n%s", stdout)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read wrangler log: %v", err)
	}
	got := string(raw)
	for _, want := range []string{
		"pnpm argv: run build",
		"argv: deploy",
		"CLOUDFLARE_API_TOKEN=tok-cf",
		"CLOUDFLARE_ACCOUNT_ID=acct-cf",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("wrangler log missing %q\n--- log:\n%s", want, got)
		}
	}
}

func installFakePackageManager(t *testing.T, dir, name, logPath string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir fake package-manager dir: %v", err)
	}
	path := filepath.Join(dir, name)
	body := `#!/bin/sh
echo "` + name + ` argv: $@" >> "` + logPath + `"
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake package manager: %v", err)
	}
}

func writeTestKubeconfig(t *testing.T, dir, current string, contexts ...string) string {
	t.Helper()
	path := filepath.Join(dir, "kubeconfig.yaml")
	var b strings.Builder
	b.WriteString("apiVersion: v1\nkind: Config\n")
	if current != "" {
		b.WriteString(fmt.Sprintf("current-context: %s\n", current))
	}
	b.WriteString("clusters:\n- name: test\n  cluster:\n    server: https://127.0.0.1:6443\n")
	b.WriteString("users:\n- name: test\n  user:\n    token: test\n")
	b.WriteString("contexts:\n")
	for _, ctx := range contexts {
		b.WriteString(fmt.Sprintf("- name: %s\n  context:\n    cluster: test\n    user: test\n", ctx))
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	return path
}

func installProjectLocalWrangler(t *testing.T, projectDir, logPath, urlOnDeploy string) {
	t.Helper()
	bin := filepath.Join(projectDir, "node_modules", ".bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir fake wrangler bin: %v", err)
	}
	path := filepath.Join(bin, "wrangler")
	body := `#!/bin/sh
{
  echo "argv: $@"
  echo "CLOUDFLARE_API_TOKEN=$CLOUDFLARE_API_TOKEN"
  echo "CLOUDFLARE_ACCOUNT_ID=$CLOUDFLARE_ACCOUNT_ID"
} >> "` + logPath + `"
case "$1" in
  deploy)
    printf 'Total Upload: 1.23 KiB\n'
    printf 'Uploaded web (1.2 sec)\n'
    printf '  ` + urlOnDeploy + `\n'
    ;;
esac
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake wrangler: %v", err)
	}
}

func installFakeKubectl(t *testing.T, dir, architecture string) {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	path := filepath.Join(bin, "kubectl")
	body := fmt.Sprintf("#!/bin/sh\nprintf '%s\\n'\n", architecture)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake kubectl: %v", err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Sanity-check: actually invoke `kubectl` through PATH and confirm
	// it returns exactly the architecture we wrote. On clean runners
	// (GHA-hosted, dev boxes without kubectl) the prepend wins and this
	// passes trivially. On self-hosted runners with a system kubectl
	// installed, the fake may not get invoked — observed empirically
	// on a macOS ARM64 self-hosted runner; suspected cause is some
	// combination of PATH-resolution quirks specific to that
	// environment. Whatever the cause, sileently falling through here
	// makes `deploy --dry-run` connect to the test's fake cluster URL,
	// fail with K8S_PLATFORM_UNDETECTED, and produce a misleading
	// stack trace pointing at deploy logic rather than test setup.
	// Skip with full diagnostics instead.
	out, err := exec.Command("kubectl").Output()
	if err != nil || strings.TrimSpace(string(out)) != architecture {
		resolved, _ := exec.LookPath("kubectl")
		t.Skipf("fake kubectl shadowing failed: exec(kubectl) → out=%q err=%v "+
			"(expected %q). LookPath → %q (want %q). PATH=%s. "+
			"Likely cause: system kubectl on this runner shadowing the test fake. "+
			"Mitigation: uninstall kubectl from this runner or move it off PATH.",
			strings.TrimSpace(string(out)), err, architecture, resolved, path, os.Getenv("PATH"))
	}
}
